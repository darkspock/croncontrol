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
	"strings"
	"sync"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/croncontrol/croncontrol/internal/executor"
)

var _ executor.Method = (*Method)(nil)
var _ executor.BlockingMethod = (*Method)(nil)

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

type handleData struct {
	ClusterID string `json:"cluster_id,omitempty"`
	Namespace string `json:"namespace"`
	JobName   string `json:"job_name"`
}

// New creates a new K8s execution method.
func New(loader ClusterLoader) *Method {
	return &Method{loadCluster: loader}
}

func (m *Method) Start(ctx context.Context, params executor.StartParams) (executor.StartResult, error) {
	clientset, namespace, clusterID, jobName, labels, err := m.createJob(ctx, params)
	if err != nil {
		result := executor.Result{Error: err}
		return executor.StartResult{
			Handle: executor.Handle{MethodName: "k8s", RunID: params.RunID, Data: map[string]any{}},
			Result: &result,
		}, nil
	}

	handle := executor.Handle{
		MethodName: "k8s",
		RunID:      params.RunID,
		Data: map[string]any{
			"cluster_id": clusterID,
			"namespace":  namespace,
			"job_name":   jobName,
		},
	}

	if !isDetached(params.MethodConfig) {
		m.activeJobs.Store(params.RunID, &activeJobRef{client: clientset, namespace: namespace, name: jobName})
		result, err := m.waitForBlockingResult(ctx, clientset, namespace, jobName, labels)
		m.activeJobs.Delete(params.RunID)
		return executor.StartResult{
			Handle:     handle,
			AcceptedAt: time.Now().UTC(),
			Result:     &result,
		}, err
	}

	return executor.StartResult{
		Handle:     handle,
		AcceptedAt: time.Now().UTC(),
		Result:     nil,
	}, nil
}

func (m *Method) Poll(ctx context.Context, handle executor.Handle, cursor executor.PollCursor) (executor.PollResult, error) {
	data := handleData{
		ClusterID: getString(handle.Data, "cluster_id"),
		Namespace: getString(handle.Data, "namespace"),
		JobName:   getString(handle.Data, "job_name"),
	}
	if data.Namespace == "" || data.JobName == "" {
		return executor.PollResult{}, fmt.Errorf("k8s: incomplete handle data")
	}

	clientset, _, err := m.buildClientForHandle(ctx, data.ClusterID)
	if err != nil {
		return executor.PollResult{}, err
	}

	job, err := clientset.BatchV1().Jobs(data.Namespace).Get(ctx, data.JobName, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return executor.PollResult{State: executor.RemoteKilled}, nil
		}
		return executor.PollResult{}, fmt.Errorf("k8s: get job: %w", err)
	}

	logs := captureJobLogs(ctx, clientset, data.Namespace, job.Spec.Selector)
	chunk := sliceFromOffset(logs, cursor.StdoutOffset)
	result := executor.PollResult{
		Cursor: executor.PollCursor{
			StdoutOffset: int64(len(logs)),
			StderrOffset: cursor.StderrOffset,
		},
		StdoutChunk: chunk,
	}

	switch {
	case job.Status.Succeeded > 0:
		exitCode := 0
		result.State = executor.RemoteCompleted
		result.ExitCode = &exitCode
	case job.Status.Failed > 0:
		exitCode := 1
		result.State = executor.RemoteFailed
		result.ExitCode = &exitCode
	case jobFinished(job):
		exitCode := 1
		result.State = executor.RemoteFailed
		result.ExitCode = &exitCode
	default:
		result.State = executor.RemoteRunning
	}
	return result, nil
}

func (m *Method) Execute(ctx context.Context, params executor.ExecuteParams) (executor.Result, error) {
	startResult, err := m.Start(ctx, executor.StartParams{
		RunID:        params.RunID,
		WorkspaceID:  params.WorkspaceID,
		MethodConfig: params.MethodConfig,
		Environment:  params.Environment,
		APIBaseURL:   params.APIBaseURL,
	})
	if err != nil {
		return executor.Result{}, err
	}
	if startResult.Result != nil {
		return *startResult.Result, nil
	}

	start := time.Now()
	cursor := executor.PollCursor{}
	var stdout strings.Builder
	for {
		poll, err := m.Poll(ctx, startResult.Handle, cursor)
		if err != nil {
			return executor.Result{Error: err, DurationMs: time.Since(start).Milliseconds()}, nil
		}
		cursor = poll.Cursor
		stdout.WriteString(poll.StdoutChunk)
		switch poll.State {
		case executor.RemoteRunning:
			time.Sleep(2 * time.Second)
			continue
		case executor.RemoteCompleted:
			return executor.Result{
				ExitCode:   poll.ExitCode,
				Stdout:     stdout.String(),
				DurationMs: time.Since(start).Milliseconds(),
			}, nil
		case executor.RemoteKilled:
			exitCode := 137
			return executor.Result{
				ExitCode:   &exitCode,
				Stdout:     stdout.String(),
				DurationMs: time.Since(start).Milliseconds(),
				Error:      fmt.Errorf("k8s job killed"),
			}, nil
		default:
			exitCode := 1
			if poll.ExitCode != nil {
				exitCode = *poll.ExitCode
			}
			return executor.Result{
				ExitCode:   &exitCode,
				Stdout:     stdout.String(),
				DurationMs: time.Since(start).Milliseconds(),
				Error:      poll.Error,
			}, nil
		}
	}
}

func (m *Method) Kill(ctx context.Context, handle executor.Handle) error {
	if val, ok := m.activeJobs.Load(handle.RunID); ok {
		ref := val.(*activeJobRef)
		return deleteJob(ref.client, ref.namespace, ref.name)
	}

	data := handleData{
		ClusterID: getString(handle.Data, "cluster_id"),
		Namespace: getString(handle.Data, "namespace"),
		JobName:   getString(handle.Data, "job_name"),
	}
	clientset, _, err := m.buildClientForHandle(ctx, data.ClusterID)
	if err != nil {
		return err
	}
	return deleteJob(clientset, data.Namespace, data.JobName)
}

func (m *Method) SupportsKill() bool      { return true }
func (m *Method) SupportsHeartbeat() bool { return true }
func (m *Method) IsAsync() bool           { return true }

func (m *Method) createJob(ctx context.Context, params executor.StartParams) (kubernetes.Interface, string, string, string, map[string]string, error) {
	cfg := params.MethodConfig

	image, _ := cfg["image"].(string)
	if image == "" {
		return nil, "", "", "", nil, fmt.Errorf("k8s: image is required")
	}

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
		return nil, "", "", "", nil, fmt.Errorf("k8s: command is required")
	}

	namespace, _ := cfg["namespace"].(string)
	clusterID, _ := cfg["k8s_cluster_id"].(string)
	clientset, defaultNamespace, err := m.buildClientForHandle(ctx, clusterID)
	if err != nil {
		return nil, "", "", "", nil, err
	}
	if namespace == "" {
		namespace = defaultNamespace
		if namespace == "" {
			namespace = "default"
		}
	}

	var envVars []corev1.EnvVar
	for k, v := range params.Environment {
		envVars = append(envVars, corev1.EnvVar{Name: k, Value: v})
	}
	envVars = append(envVars, corev1.EnvVar{Name: "CRONCONTROL_RUN_ID", Value: params.RunID})
	if params.APIBaseURL != "" {
		envVars = append(envVars, corev1.EnvVar{Name: "CRONCONTROL_API_URL", Value: params.APIBaseURL})
	}

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

	jobName := fmt.Sprintf("cc-%s", params.RunID)
	if len(jobName) > 63 {
		jobName = jobName[:63]
	}
	backoffLimit := int32(0)
	ttl := int32(600)
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
		return nil, "", "", "", nil, fmt.Errorf("k8s: create job: %w", err)
	}
	return clientset, namespace, clusterID, createdJob.Name, labels, nil
}

func (m *Method) waitForBlockingResult(ctx context.Context, clientset kubernetes.Interface, namespace, jobName string, labels map[string]string) (executor.Result, error) {
	start := time.Now()
	for {
		job, err := clientset.BatchV1().Jobs(namespace).Get(ctx, jobName, metav1.GetOptions{})
		if err != nil {
			return executor.Result{Error: fmt.Errorf("k8s: get job: %w", err), DurationMs: time.Since(start).Milliseconds()}, nil
		}
		if job.Status.Succeeded > 0 {
			exitCode := 0
			stdout := captureLogsByLabels(ctx, clientset, namespace, labels)
			return executor.Result{ExitCode: &exitCode, Stdout: stdout, DurationMs: time.Since(start).Milliseconds()}, nil
		}
		if job.Status.Failed > 0 || jobFinished(job) {
			exitCode := 1
			stdout := captureLogsByLabels(ctx, clientset, namespace, labels)
			return executor.Result{
				ExitCode:   &exitCode,
				Stdout:     stdout,
				DurationMs: time.Since(start).Milliseconds(),
				Error:      fmt.Errorf("k8s job failed"),
			}, nil
		}
		select {
		case <-ctx.Done():
			return executor.Result{Error: ctx.Err(), DurationMs: time.Since(start).Milliseconds()}, nil
		case <-time.After(2 * time.Second):
		}
	}
}

func (m *Method) buildClientForHandle(ctx context.Context, clusterID string) (kubernetes.Interface, string, error) {
	var kubeconfig []byte
	defaultNamespace := "default"
	if clusterID != "" && m.loadCluster != nil {
		kc, defNs, err := m.loadCluster(ctx, clusterID)
		if err != nil {
			return nil, "", fmt.Errorf("k8s: load cluster: %w", err)
		}
		kubeconfig = kc
		if defNs != "" {
			defaultNamespace = defNs
		}
	}
	client, err := buildClient(kubeconfig)
	if err != nil {
		return nil, "", fmt.Errorf("k8s: build client: %w", err)
	}
	return client, defaultNamespace, nil
}

func buildClient(kubeconfig []byte) (kubernetes.Interface, error) {
	if len(kubeconfig) > 0 {
		config, err := clientcmd.RESTConfigFromKubeConfig(kubeconfig)
		if err != nil {
			return nil, fmt.Errorf("parse kubeconfig: %w", err)
		}
		return kubernetes.NewForConfig(config)
	}
	config, err := clientcmd.BuildConfigFromFlags("", "")
	if err != nil {
		return nil, fmt.Errorf("in-cluster config: %w", err)
	}
	return kubernetes.NewForConfig(config)
}

func captureJobLogs(ctx context.Context, clientset kubernetes.Interface, namespace string, selector *metav1.LabelSelector) string {
	if selector == nil {
		return ""
	}
	return captureLogsBySelector(ctx, clientset, namespace, metav1.FormatLabelSelector(selector))
}

func captureLogsByLabels(ctx context.Context, clientset kubernetes.Interface, namespace string, labels map[string]string) string {
	selector := metav1.FormatLabelSelector(&metav1.LabelSelector{MatchLabels: labels})
	return captureLogsBySelector(ctx, clientset, namespace, selector)
}

func captureLogsBySelector(ctx context.Context, clientset kubernetes.Interface, namespace, selector string) string {
	pods, err := clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{LabelSelector: selector})
	if err != nil || len(pods.Items) == 0 {
		return ""
	}

	var allLogs bytes.Buffer
	for _, pod := range pods.Items {
		for _, container := range pod.Spec.Containers {
			req := clientset.CoreV1().Pods(namespace).GetLogs(pod.Name, &corev1.PodLogOptions{Container: container.Name})
			stream, err := req.Stream(ctx)
			if err != nil {
				continue
			}
			_, _ = io.Copy(&allLogs, stream)
			_ = stream.Close()
		}
	}
	return allLogs.String()
}

func deleteJob(clientset kubernetes.Interface, namespace, name string) error {
	propagation := metav1.DeletePropagationForeground
	err := clientset.BatchV1().Jobs(namespace).Delete(context.Background(), name, metav1.DeleteOptions{
		PropagationPolicy: &propagation,
	})
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("k8s: delete job: %w", err)
	}
	return nil
}

func jobFinished(job *batchv1.Job) bool {
	for _, c := range job.Status.Conditions {
		if c.Type == batchv1.JobComplete || c.Type == batchv1.JobFailed {
			return true
		}
	}
	return false
}

func isDetached(cfg map[string]any) bool {
	if v, ok := cfg["detach"].(bool); ok {
		return v
	}
	if v, ok := cfg["wait"].(bool); ok {
		return !v
	}
	return true
}

func getString(cfg map[string]any, key string) string {
	v, _ := cfg[key].(string)
	return v
}

func sliceFromOffset(s string, offset int64) string {
	if offset <= 0 {
		return s
	}
	if offset >= int64(len(s)) {
		return ""
	}
	return s[offset:]
}
