# CronControl Glossary

## Public Terms (use in API, dashboard, CLI, MCP, SDKs, docs)

| Term | Definition |
|---|---|
| **workspace** | An isolated organizational unit. All resources belong to a workspace. |
| **user** | A person who accesses CronControl via dashboard or API. |
| **process** | A configured unit of scheduled or on-demand work. |
| **run** | A single execution instance of a process. |
| **queue** | A named channel for event-driven background jobs. |
| **job** | A single unit of work dispatched into a queue. |
| **worker** | A customer-deployed runtime and network gateway for private execution. |
| **webhook subscription** | A configured endpoint that receives event notifications. |
| **API key** | A workspace-scoped credential for programmatic API access. |
| **SSH credential** | A reusable workspace resource for SSH key-based authentication. |
| **SSM profile** | A reusable workspace resource for AWS Systems Manager access. |
| **K8s cluster** | A reusable workspace resource for Kubernetes cluster access. |

## Internal-Only Terms (never expose in product surfaces)

| Term | Maps to | Usage |
|---|---|---|
| **tenant** | workspace | Database/code internals only |
| **scheduled_slot** | run | Legacy name, do not use |

## Schedule Types

| Public name | Description |
|---|---|
| `cron` | Fixed schedule using 5-field cron syntax. |
| `fixed_delay` | Fixed duration after previous run finishes. |
| `on_demand` | No automatic schedule. Manual trigger or dependency only. |

## Run Origins

| Origin | Description |
|---|---|
| `cron` | Created by the planner from a cron schedule. |
| `fixed_delay` | Created automatically after a fixed-delay run completes. |
| `manual` | Created by a user via trigger action. |
| `one_time` | Scheduled by a user for a specific date/time. |
| `recovery` | Created on startup for missed runs. |
| `dependency` | Triggered by a dependent process completing. |
| `replay` | Replayed from a terminal run. |

## Execution Methods

| Method | Description |
|---|---|
| `http` | HTTP request to a URL. |
| `ssh` | Remote command via SSH. |
| `ssm` | AWS Systems Manager SendCommand. |
| `k8s` | Kubernetes Job. |

## Runtimes

| Runtime | Description |
|---|---|
| `direct` | Executed directly from the SaaS control plane. Default. |
| `worker` | Executed via a customer-deployed CronControl Worker. |

## ID Prefixes

| Prefix | Resource |
|---|---|
| `wsp_` | workspace |
| `usr_` | user |
| `wmb_` | workspace membership |
| `wrk_` | worker |
| `prc_` | process |
| `run_` | run |
| `rat_` | run attempt |
| `que_` | queue |
| `job_` | job |
| `jat_` | job attempt |
| `whs_` | webhook subscription |
| `key_` | API key |
| `ssh_` | SSH credential |
| `ssp_` | SSM profile |
| `k8c_` | K8s cluster |
