<?php

namespace CronControl\Laravel;

use GuzzleHttp\Client;
use GuzzleHttp\Exception\ClientException;
use GuzzleHttp\Exception\GuzzleException;

/**
 * CronControl API client for Laravel.
 *
 * Thin wrapper around the REST API with Guzzle HTTP client.
 * Integrates with Laravel via service provider, config, and facade.
 *
 * Usage via facade:
 *   use CronControl\Laravel\Facades\CronControl;
 *   $processes = CronControl::listProcesses();
 *   CronControl::triggerProcess('prc_01HYX...');
 *   CronControl::heartbeat('run_01HYX...', 100, 50, 'Halfway');
 *
 * Usage via DI:
 *   public function __construct(private CronControlClient $cc) {}
 *   $this->cc->enqueueJob(['queue_id' => 'que_...', 'payload' => [...]]);
 */
class CronControlClient
{
    private Client $http;
    private string $apiKey;

    public function __construct(string $baseUrl, string $apiKey, int $timeout = 30)
    {
        $this->apiKey = $apiKey;
        $this->http = new Client([
            'base_uri' => rtrim($baseUrl, '/') . '/api/v1/',
            'timeout' => $timeout,
            'headers' => [
                'Content-Type' => 'application/json',
                'Accept' => 'application/json',
            ],
        ]);
    }

    // ── Workspace ──────────────────────────────────────────────────────────

    public function getWorkspace(): array
    {
        return $this->get('workspace');
    }

    // ── Processes ──────────────────────────────────────────────────────────

    public function listProcesses(array $params = []): array
    {
        return $this->get('processes', $params);
    }

    public function getProcess(string $id): array
    {
        return $this->get("processes/{$id}");
    }

    public function createProcess(array $data): array
    {
        return $this->post('processes', $data);
    }

    public function updateProcess(string $id, array $data): array
    {
        return $this->put("processes/{$id}", $data);
    }

    public function deleteProcess(string $id): void
    {
        $this->delete("processes/{$id}");
    }

    public function triggerProcess(string $id): array
    {
        return $this->post("processes/{$id}/trigger");
    }

    public function pauseProcess(string $id, bool $cancelPending = false): void
    {
        $this->post("processes/{$id}/pause", [], ['cancel_pending' => $cancelPending ? 'true' : 'false']);
    }

    public function resumeProcess(string $id): void
    {
        $this->post("processes/{$id}/resume");
    }

    // ── Runs ───────────────────────────────────────────────────────────────

    public function listRuns(array $params = []): array
    {
        return $this->get('runs', $params);
    }

    public function getRun(string $id): array
    {
        return $this->get("runs/{$id}");
    }

    public function cancelRun(string $id): void
    {
        $this->post("runs/{$id}/cancel");
    }

    public function killRun(string $id): void
    {
        $this->post("runs/{$id}/kill");
    }

    public function replayRun(string $id): array
    {
        return $this->post("runs/{$id}/replay");
    }

    public function getRunOutput(string $id, ?string $stream = null): array
    {
        $params = $stream ? ['stream' => $stream] : [];
        return $this->get("runs/{$id}/output", $params);
    }

    // ── Queues ─────────────────────────────────────────────────────────────

    public function listQueues(): array
    {
        return $this->get('queues');
    }

    public function getQueue(string $id): array
    {
        return $this->get("queues/{$id}");
    }

    public function createQueue(array $data): array
    {
        return $this->post('queues', $data);
    }

    // ── Jobs ───────────────────────────────────────────────────────────────

    public function listJobs(array $params = []): array
    {
        return $this->get('jobs', $params);
    }

    public function getJob(string $id): array
    {
        return $this->get("jobs/{$id}");
    }

    public function enqueueJob(array $data): array
    {
        return $this->post('jobs', $data);
    }

    public function cancelJob(string $id): void
    {
        $this->post("jobs/{$id}/cancel");
    }

    public function replayJob(string $id, array $overrides = []): array
    {
        return $this->post("jobs/{$id}/replay", $overrides ?: null);
    }

    // ── Workers ────────────────────────────────────────────────────────────

    public function listWorkers(): array
    {
        return $this->get('workers');
    }

    public function createWorker(array $data): array
    {
        return $this->post('workers', $data);
    }

    public function deleteWorker(string $id): void
    {
        $this->delete("workers/{$id}");
    }

    // ── API Keys ───────────────────────────────────────────────────────────

    public function listApiKeys(): array
    {
        return $this->get('api-keys');
    }

    public function createApiKey(array $data): array
    {
        return $this->post('api-keys', $data);
    }

    public function deleteApiKey(string $id): void
    {
        $this->delete("api-keys/{$id}");
    }

    // ── Run Results ─────────────────────────────────────────────────────────

    public function setResult(string $runId, array $data): array
    {
        return $this->patch("runs/{$runId}/result", $data);
    }

    public function getResult(string $runId): array
    {
        return $this->get("runs/{$runId}/result");
    }

    // ── Secrets ─────────────────────────────────────────────────────────────

    public function listSecrets(): array
    {
        return $this->get('secrets');
    }

    public function createSecret(string $name, string $value): array
    {
        return $this->post('secrets', ['name' => $name, 'value' => $value]);
    }

    public function updateSecret(string $name, string $value): array
    {
        return $this->put("secrets/{$name}", ['value' => $value]);
    }

    public function deleteSecret(string $name): void
    {
        $this->delete("secrets/{$name}");
    }

    // ── Artifacts ───────────────────────────────────────────────────────────

    public function listArtifacts(string $runId): array
    {
        return $this->get("runs/{$runId}/artifacts");
    }

    public function getArtifactUrl(string $runId, string $name): string
    {
        return rtrim($this->http->getConfig('base_uri'), '/') . "/runs/{$runId}/artifacts/{$name}";
    }

    // ── Orchestras ──────────────────────────────────────────────────────────

    public function createOrchestra(array $data): array
    {
        return $this->post('orchestras', $data);
    }

    public function getScore(string $orchestraId): array
    {
        return $this->get("orchestras/{$orchestraId}/score");
    }

    public function finishOrchestra(string $id, string $summary = ''): array
    {
        return $this->post("orchestras/{$id}/finish", ['summary' => $summary]);
    }

    public function cancelOrchestra(string $id): array
    {
        return $this->post("orchestras/{$id}/cancel");
    }

    public function nextMovement(string $runId, string $processId, ?array $payload = null): array
    {
        return $this->post("runs/{$runId}/next", ['process_id' => $processId, 'payload' => $payload]);
    }

    public function askChoice(string $runId, string $message, array $choices): array
    {
        return $this->post("runs/{$runId}/choice", ['message' => $message, 'choices' => $choices]);
    }

    /**
     * Read orchestra event context from environment variables.
     */
    public function getEvent(): array
    {
        $result = getenv('CRONCONTROL_EVENT_RESULT');

        return [
            'type' => getenv('CRONCONTROL_EVENT_TYPE') ?: '',
            'orchestraId' => getenv('CRONCONTROL_ORCHESTRA_ID') ?: '',
            'step' => (int) (getenv('CRONCONTROL_ORCHESTRA_STEP') ?: '0'),
            'runId' => getenv('CRONCONTROL_EVENT_RUN_ID') ?: '',
            'result' => $result ? json_decode($result, true) : null,
        ];
    }

    public function postChat(string $orchestraId, string $content, string $type = 'text'): array
    {
        return $this->post("orchestras/{$orchestraId}/chat", ['content' => $content, 'message_type' => $type]);
    }

    public function getChat(string $orchestraId): array
    {
        return $this->get("orchestras/{$orchestraId}/chat");
    }

    // ── Heartbeat (no auth required) ───────────────────────────────────────

    /**
     * Report execution progress. Called from within a running job/process.
     */
    public function heartbeat(string $runId, int $total, int $current, string $message = ''): void
    {
        $this->post('heartbeat', [
            'run_id' => $runId,
            'total' => $total,
            'current' => $current,
            'message' => $message,
        ]);
    }

    // ── Health ─────────────────────────────────────────────────────────────

    public function health(): array
    {
        return $this->get('health');
    }

    // ── HTTP helpers ───────────────────────────────────────────────────────

    private function get(string $path, array $query = []): array
    {
        return $this->request('GET', $path, ['query' => $query]);
    }

    private function post(string $path, ?array $body = null, array $query = []): array
    {
        $options = ['query' => $query];
        if ($body !== null) {
            $options['json'] = $body;
        }
        return $this->request('POST', $path, $options);
    }

    private function put(string $path, array $body): array
    {
        return $this->request('PUT', $path, ['json' => $body]);
    }

    private function patch(string $path, array $body): array
    {
        return $this->request('PATCH', $path, ['json' => $body]);
    }

    private function delete(string $path): void
    {
        $this->request('DELETE', $path);
    }

    private function request(string $method, string $path, array $options = []): array
    {
        $options['headers'] = ['X-API-Key' => $this->apiKey];

        try {
            $response = $this->http->request($method, $path, $options);
            $status = $response->getStatusCode();

            if ($status === 204) {
                return [];
            }

            return json_decode($response->getBody()->getContents(), true) ?? [];
        } catch (ClientException $e) {
            $body = json_decode($e->getResponse()->getBody()->getContents(), true);
            $error = $body['error'] ?? [];

            throw new CronControlException(
                $e->getResponse()->getStatusCode(),
                $error['code'] ?? 'UNKNOWN',
                $error['message'] ?? $e->getMessage(),
                $error['hint'] ?? '',
            );
        }
    }
}
