// Package k8s implements the Kubernetes Job execution method.
//
// Canonical rules (from docs/product-specification.md):
// - Always creates a Kubernetes Job.
// - k8s_cluster is a reusable workspace resource.
// - Resource may define an optional default namespace.
// - Process or queue may override namespace.
// - Only a controlled subset of Job fields is configurable.
// - Pod logs are captured as output.
// - Heartbeat supported (pod can call heartbeat API).
package k8s

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"sync"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/croncontrol/croncontrol/internal/executor"
)

// Compile-time contract.
var _ executor.Method = (*Method)(nil)

// ClusterLoader loads a kubeconfig and default namespace by cluster ID.
type ClusterLoader func(ctx context.Context, clusterID string) (kubeconfig []byte, defaultNamespace string, err error)

// Method implements K8s Job execution.
type Method struct {
	loadCluster ClusterLoader

	// Active jobs keyed by RunID for concurrent kill support.
	activeJobs sync.Map // map[string]*activeJobRef
}

type activeJobRef struct {
	client    kubernetes.Interface
	namespace string
	name      string
}

// New creates a new K8s execution method.
func New(loader ClusterLoader) *Method {
	return &Method{loadCluster: loader}
}

func (m *Method) Execute(ctx context.Context, params executor.ExecuteParams) (executor.Result, error) {
	cfg := params.MethodConfig
	start := time.Now()

	image, _ := cfg["image"].(string)
	if image == "" {
		return executor.Result{Error: fmt.Errorf("k8s: image is required")}, nil
	}

	// Parse command (string or []string)
	var command []string
	switch v := cfg["command"].(type) {
	case string:
		command = []string{"/bin/sh", "-c", v}
	case []any:
		for _, item := range v {
			if s, ok := item.(string); ok {
				command = append(command, s)
			}
		}
	}
	if len(command) == 0 {
		return executor.Result{Error: fmt.Errorf("k8s: command is required")}, nil
	}

	namespace, _ := cfg["namespace"].(string)
	clusterID, _ := cfg["k8s_cluster_id"].(string)

	// Load cluster config
	var kubeconfig []byte
	defaultNamespace := "default"

	if clusterID != "" && m.loadCluster != nil {
		kc, defNs, err := m.loadCluster(ctx, clusterID)
		if err != nil {
			return executor.Result{Error: fmt.Errorf("k8s: load cluster: %w", err), DurationMs: time.Since(start).Milliseconds()}, nil
		}
		kubeconfig = kc
		if defNs != "" {
			defaultNamespace = defNs
		}
	}

	if namespace == "" {
		namespace = defaultNamespace
	}

	// Build k8s client
	clientset, err := buildClient(kubeconfig)
	if err != nil {
		return executor.Result{Error: fmt.Errorf("k8s: build client: %w", err), DurationMs: time.Since(start).Milliseconds()}, nil
	}

	// Build environment variables
	var envVars []corev1.EnvVar
	if params.Environment != nil {
		for k, v := range params.Environment {
			envVars = append(envVars, corev1.EnvVar{Name: k, Value: v})
		}
	}
	envVars = append(envVars,
		corev1.EnvVar{Name: "CRONCONTROL_RUN_ID", Value: params.RunID},
	)
	if params.APIBaseURL != "" {
		envVars = append(envVars, corev1.EnvVar{Name: "CRONCONTROL_API_URL", Value: params.APIBaseURL})
	}

	// Build resource requirements
	resources := corev1.ResourceRequirements{}
	if limits, ok := cfg["resources"].(map[string]any); ok {
		resources.Limits = corev1.ResourceList{}
		resources.Requests = corev1.ResourceList{}
		if cpu, ok := limits["cpu"].(string); ok {
			resources.Limits[corev1.ResourceCPU] = resource.MustParse(cpu)
			resources.Requests[corev1.ResourceCPU] = resource.MustParse(cpu)
		}
		if mem, ok := limits["memory"].(string); ok {
			resources.Limits[corev1.ResourceMemory] = resource.MustParse(mem)
			resources.Requests[corev1.ResourceMemory] = resource.MustParse(mem)
		}
	}

	// Build labels
	labels := map[string]string{
		"app.kubernetes.io/managed-by": "croncontrol",
		"croncontrol.dev/run-id":       params.RunID,
		"croncontrol.dev/workspace-id": params.WorkspaceID,
	}
	if extraLabels, ok := cfg["labels"].(map[string]any); ok {
		for k, v := range extraLabels {
			if s, ok := v.(string); ok {
				labels[k] = s
			}
		}
	}

	// Create Job
	jobName := fmt.Sprintf("cc-%s", params.RunID)
	if len(jobName) > 63 {
		jobName = jobName[:63]
	}
	backoffLimit := int32(0)
	ttl := int32(600) // 10 min TTL after finished

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobName,
			Namespace: namespace,
			Labels:    labels,
		},
		Spec: batchv1.JobSpec{
			BackoffLimit:            &backoffLimit,
			TTLSecondsAfterFinished: &ttl,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: labels},
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyNever,
					Containers: []corev1.Container{{
						Name:      "run",
						Image:     image,
						Command:   command,
						Env:       envVars,
						Resources: resources,
					}},
				},
			},
		},
	}

	createdJob, err := clientset.BatchV1().Jobs(namespace).Create(ctx, job, metav1.CreateOptions{})
	if err != nil {
		return executor.Result{Error: fmt.Errorf("k8s: create job: %w", err), DurationMs: time.Since(start).Milliseconds()}, nil
	}

	slog.Info("k8s: job created", "name", createdJob.Name, "namespace", namespace)

	// Track for kill support (keyed by RunID)
	m.activeJobs.Store(params.RunID, &activeJobRef{client: clientset, namespace: namespace, name: createdJob.Name})
	defer m.activeJobs.Delete(params.RunID)

	// Watch for completion
	if err := waitForJobCompletion(ctx, clientset, namespace, createdJob.Name); err != nil {
		durationMs := time.Since(start).Milliseconds()
		return executor.Result{Error: fmt.Errorf("k8s: wait for job: %w", err), DurationMs: durationMs}, nil
	}

	// Get final job status
	finalJob, err := clientset.BatchV1().Jobs(namespace).Get(ctx, createdJob.Name, metav1.GetOptions{})
	if err != nil {
		return executor.Result{Error: fmt.Errorf("k8s: get final status: %w", err), DurationMs: time.Since(start).Milliseconds()}, nil
	}

	// Capture pod logs
	stdout, stderr := capturePodLogs(ctx, clientset, namespace, labels)

	durationMs := time.Since(start).Milliseconds()

	// Determine success/failure
	if finalJob.Status.Succeeded > 0 {
		exitCode := 0
		return executor.Result{ExitCode: &exitCode, Stdout: stdout, Stderr: stderr, DurationMs: durationMs}, nil
	}

	exitCode := 1
	errMsg := "k8s job failed"
	if len(finalJob.Status.Conditions) > 0 {
		errMsg = finalJob.Status.Conditions[len(finalJob.Status.Conditions)-1].Message
	}
	return executor.Result{
		ExitCode:   &exitCode,
		Stdout:     stdout,
		Stderr:     stderr,
		Error:      fmt.Errorf("%s", errMsg),
		DurationMs: durationMs,
	}, nil
}

func (m *Method) Kill(_ context.Context, handle executor.Handle) error {
	val, ok := m.activeJobs.Load(handle.RunID)
	if !ok {
		return fmt.Errorf("k8s: no active job for run %s", handle.RunID)
	}
	ref := val.(*activeJobRef)

	propagation := metav1.DeletePropagationForeground
	err := ref.client.BatchV1().Jobs(ref.namespace).Delete(context.Background(), ref.name, metav1.DeleteOptions{
		PropagationPolicy: &propagation,
	})
	if err != nil {
		return fmt.Errorf("k8s: delete job: %w", err)
	}

	slog.Info("k8s: job deleted", "name", ref.name, "namespace", ref.namespace)
	return nil
}

func (m *Method) SupportsKill() bool     { return true }
func (m *Method) SupportsHeartbeat() bool { return true }

func buildClient(kubeconfig []byte) (kubernetes.Interface, error) {
	if len(kubeconfig) > 0 {
		config, err := clientcmd.RESTConfigFromKubeConfig(kubeconfig)
		if err != nil {
			return nil, fmt.Errorf("parse kubeconfig: %w", err)
		}
		return kubernetes.NewForConfig(config)
	}

	// Fall back to in-cluster config
	config, err := clientcmd.BuildConfigFromFlags("", "")
	if err != nil {
		return nil, fmt.Errorf("in-cluster config: %w", err)
	}
	return kubernetes.NewForConfig(config)
}

// waitForJobCompletion watches the job until it completes or fails.
func waitForJobCompletion(ctx context.Context, clientset kubernetes.Interface, namespace, name string) error {
	watcher, err := clientset.BatchV1().Jobs(namespace).Watch(ctx, metav1.ListOptions{
		FieldSelector: fmt.Sprintf("metadata.name=%s", name),
	})
	if err != nil {
		return fmt.Errorf("watch job: %w", err)
	}
	defer watcher.Stop()

	for event := range watcher.ResultChan() {
		if event.Type == watch.Error {
			return fmt.Errorf("watch error")
		}

		job, ok := event.Object.(*batchv1.Job)
		if !ok {
			continue
		}

		if job.Status.Succeeded > 0 || job.Status.Failed > 0 {
			return nil
		}

		for _, c := range job.Status.Conditions {
			if c.Type == batchv1.JobComplete || c.Type == batchv1.JobFailed {
				return nil
			}
		}
	}

	return fmt.Errorf("watch ended unexpectedly")
}

// capturePodLogs fetches logs from pods matching the given labels.
func capturePodLogs(ctx context.Context, clientset kubernetes.Interface, namespace string, labels map[string]string) (string, string) {
	selector := metav1.FormatLabelSelector(&metav1.LabelSelector{MatchLabels: labels})

	pods, err := clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: selector,
	})
	if err != nil || len(pods.Items) == 0 {
		return "", ""
	}

	var allLogs bytes.Buffer
	for _, pod := range pods.Items {
		for _, container := range pod.Spec.Containers {
			req := clientset.CoreV1().Pods(namespace).GetLogs(pod.Name, &corev1.PodLogOptions{
				Container: container.Name,
			})
			stream, err := req.Stream(ctx)
			if err != nil {
				continue
			}
			io.Copy(&allLogs, stream)
			stream.Close()
		}
	}

	return allLogs.String(), ""
}
