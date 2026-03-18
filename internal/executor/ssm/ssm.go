// Package ssm implements the AWS Systems Manager execution method.
//
// Canonical rules (from docs/product-specification.md):
// - Targeting supports instance IDs and tags.
// - Target must resolve to exactly one instance.
// - SSM profiles are reusable workspace resources.
// - Heartbeat supported (remote process calls heartbeat API).
package ssm

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/aws/aws-sdk-go-v2/service/ssm/types"
	"github.com/aws/aws-sdk-go-v2/service/sts"

	"github.com/croncontrol/croncontrol/internal/executor"
)

// Compile-time contract.
var _ executor.Method = (*Method)(nil)

// ProfileLoader loads SSM profile data (region, role_arn) by profile ID.
type ProfileLoader func(ctx context.Context, profileID string) (region string, roleARN string, err error)

// Method implements SSM execution.
type Method struct {
	loadProfile ProfileLoader

	// Active commands keyed by RunID for concurrent kill support.
	activeCommands sync.Map // map[string]*activeSSMRef
}

type activeSSMRef struct {
	commandID *string
	client    *ssm.Client
}

// New creates a new SSM execution method.
func New(loader ProfileLoader) *Method {
	return &Method{loadProfile: loader}
}

func (m *Method) Execute(ctx context.Context, params executor.ExecuteParams) (executor.Result, error) {
	cfg := params.MethodConfig
	start := time.Now()

	command, _ := cfg["command"].(string)
	if command == "" {
		return executor.Result{Error: fmt.Errorf("ssm: command is required")}, nil
	}

	// Determine target: instance_id or tag-based
	instanceID, _ := cfg["instance_id"].(string)
	tagKey, _ := cfg["tag_key"].(string)
	tagValue, _ := cfg["tag_value"].(string)

	if instanceID == "" && (tagKey == "" || tagValue == "") {
		return executor.Result{Error: fmt.Errorf("ssm: instance_id or tag_key+tag_value is required")}, nil
	}

	// Load SSM profile for region and optional role
	profileID, _ := cfg["ssm_profile_id"].(string)
	region := "us-east-1"
	var roleARN string

	if profileID != "" && m.loadProfile != nil {
		r, arn, err := m.loadProfile(ctx, profileID)
		if err != nil {
			return executor.Result{Error: fmt.Errorf("ssm: load profile: %w", err), DurationMs: time.Since(start).Milliseconds()}, nil
		}
		if r != "" {
			region = r
		}
		roleARN = arn
	}
	if r, ok := cfg["region"].(string); ok && r != "" {
		region = r
	}

	// Build AWS config
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(region))
	if err != nil {
		return executor.Result{Error: fmt.Errorf("ssm: load aws config: %w", err), DurationMs: time.Since(start).Milliseconds()}, nil
	}

	// Assume role if configured
	if roleARN != "" {
		stsClient := sts.NewFromConfig(awsCfg)
		creds := stscreds.NewAssumeRoleProvider(stsClient, roleARN)
		awsCfg.Credentials = aws.NewCredentialsCache(creds)
	}

	client := ssm.NewFromConfig(awsCfg)

	// Build targets
	var targets []types.Target
	if instanceID != "" {
		targets = []types.Target{{
			Key:    aws.String("InstanceIds"),
			Values: []string{instanceID},
		}}
	} else {
		targets = []types.Target{{
			Key:    aws.String("tag:" + tagKey),
			Values: []string{tagValue},
		}}
	}

	// Inject CronControl env vars into the command
	envPrefix := ""
	if params.Environment != nil {
		for k, v := range params.Environment {
			envPrefix += fmt.Sprintf("export %s=%q; ", k, v)
		}
	}
	envPrefix += fmt.Sprintf("export CRONCONTROL_RUN_ID=%q; ", params.RunID)
	if params.APIBaseURL != "" {
		envPrefix += fmt.Sprintf("export CRONCONTROL_API_URL=%q; ", params.APIBaseURL)
	}
	fullCommand := envPrefix + command

	// Send command
	sendOut, err := client.SendCommand(ctx, &ssm.SendCommandInput{
		DocumentName: aws.String("AWS-RunShellScript"),
		Targets:      targets,
		Parameters: map[string][]string{
			"commands": {fullCommand},
		},
		TimeoutSeconds: aws.Int32(3600),
		Comment:        aws.String(fmt.Sprintf("CronControl run %s", params.RunID)),
	})
	if err != nil {
		return executor.Result{Error: fmt.Errorf("ssm: send command: %w", err), DurationMs: time.Since(start).Milliseconds()}, nil
	}

	commandID := sendOut.Command.CommandId

	// Track for kill support (keyed by RunID)
	m.activeCommands.Store(params.RunID, &activeSSMRef{commandID: commandID, client: client})
	defer m.activeCommands.Delete(params.RunID)

	// Determine the target instance ID for GetCommandInvocation
	targetInstanceID := instanceID
	if targetInstanceID == "" {
		// For tag-based targets, we need to get the instance from command status
		targetInstanceID, err = resolveTargetInstance(ctx, client, *commandID)
		if err != nil {
			return executor.Result{Error: fmt.Errorf("ssm: resolve target instance: %w", err), DurationMs: time.Since(start).Milliseconds()}, nil
		}
	}

	// Poll for completion
	result, err := pollCommandResult(ctx, client, *commandID, targetInstanceID)
	durationMs := time.Since(start).Milliseconds()
	if err != nil {
		return executor.Result{Error: fmt.Errorf("ssm: poll result: %w", err), DurationMs: durationMs}, nil
	}

	result.DurationMs = durationMs
	return result, nil
}

func (m *Method) Kill(_ context.Context, handle executor.Handle) error {
	val, ok := m.activeCommands.Load(handle.RunID)
	if !ok {
		return fmt.Errorf("ssm: no active command for run %s", handle.RunID)
	}
	ref := val.(*activeSSMRef)
	commandID := ref.commandID
	client := ref.client

	_, err := client.CancelCommand(context.Background(), &ssm.CancelCommandInput{
		CommandId: commandID,
	})
	if err != nil {
		return fmt.Errorf("ssm: cancel command: %w", err)
	}

	slog.Info("ssm: command cancelled", "command_id", *commandID)
	return nil
}

func (m *Method) SupportsKill() bool     { return true }
func (m *Method) SupportsHeartbeat() bool { return true }

// resolveTargetInstance waits briefly for the command to register an instance.
func resolveTargetInstance(ctx context.Context, client *ssm.Client, commandID string) (string, error) {
	for i := 0; i < 10; i++ {
		out, err := client.ListCommandInvocations(ctx, &ssm.ListCommandInvocationsInput{
			CommandId: aws.String(commandID),
		})
		if err != nil {
			return "", err
		}
		if len(out.CommandInvocations) == 1 {
			return *out.CommandInvocations[0].InstanceId, nil
		}
		if len(out.CommandInvocations) > 1 {
			return "", fmt.Errorf("target resolved to %d instances, expected exactly 1", len(out.CommandInvocations))
		}
		time.Sleep(2 * time.Second)
	}
	return "", fmt.Errorf("timeout waiting for command to register target instance")
}

// pollCommandResult polls GetCommandInvocation until the command reaches a terminal state.
func pollCommandResult(ctx context.Context, client *ssm.Client, commandID, instanceID string) (executor.Result, error) {
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return executor.Result{}, ctx.Err()
		case <-ticker.C:
			out, err := client.GetCommandInvocation(ctx, &ssm.GetCommandInvocationInput{
				CommandId:  aws.String(commandID),
				InstanceId: aws.String(instanceID),
			})
			if err != nil {
				// InvocationDoesNotExist is expected early on; keep polling
				if strings.Contains(err.Error(), "InvocationDoesNotExist") {
					continue
				}
				return executor.Result{}, err
			}

			switch out.Status {
			case types.CommandInvocationStatusSuccess:
				exitCode := 0
				return executor.Result{
					ExitCode: &exitCode,
					Stdout:   deref(out.StandardOutputContent),
					Stderr:   deref(out.StandardErrorContent),
				}, nil

			case types.CommandInvocationStatusFailed,
				types.CommandInvocationStatusTimedOut:
				exitCode := 1
				if out.ResponseCode > 0 {
					rc := int(out.ResponseCode)
					exitCode = rc
				}
				return executor.Result{
					ExitCode: &exitCode,
					Stdout:   deref(out.StandardOutputContent),
					Stderr:   deref(out.StandardErrorContent),
					Error:    fmt.Errorf("ssm command %s: %s", out.Status, deref(out.StatusDetails)),
				}, nil

			case types.CommandInvocationStatusCancelled:
				exitCode := 137
				return executor.Result{
					ExitCode: &exitCode,
					Stdout:   deref(out.StandardOutputContent),
					Stderr:   deref(out.StandardErrorContent),
					Error:    fmt.Errorf("ssm command cancelled"),
				}, nil

			default:
				// InProgress, Pending, Delayed — keep polling
				continue
			}
		}
	}
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
