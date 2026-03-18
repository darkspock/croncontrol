<?php
/**
 * CronControl PHP Heartbeat SDK
 *
 * Single file, zero dependencies. Reports heartbeat and progress
 * from PHP processes back to the CronControl control plane.
 *
 * Environment variables (set automatically by CronControl):
 *   CRONCONTROL_RUN_ID  - UUID of the current run
 *   CRONCONTROL_API_URL - Base URL of the CronControl API
 *
 * Usage:
 *   $cc = new CronControl();
 *   $cc->heartbeat(1000, 0, "Starting...");
 *   // ... do work ...
 *   $cc->heartbeat(1000, 500, "Halfway done");
 *   // ... more work ...
 *   $cc->heartbeat(1000, 1000, "Complete");
 */
class CronControl
{
    private string $runId;
    private string $apiUrl;

    public function __construct(?string $runId = null, ?string $apiUrl = null)
    {
        $this->runId = $runId ?? getenv('CRONCONTROL_RUN_ID') ?: '';
        $this->apiUrl = rtrim($apiUrl ?? getenv('CRONCONTROL_API_URL') ?: '', '/');

        if (empty($this->runId)) {
            $this->log('WARNING: CRONCONTROL_RUN_ID not set. Heartbeats will be ignored.');
        }
        if (empty($this->apiUrl)) {
            $this->log('WARNING: CRONCONTROL_API_URL not set. Heartbeats will be ignored.');
        }
    }

    /**
     * Send a heartbeat with progress information.
     *
     * @param int    $total   Total items to process
     * @param int    $current Items processed so far
     * @param string $message Optional status message
     */
    public function heartbeat(int $total, int $current, string $message = ''): void
    {
        if (empty($this->runId) || empty($this->apiUrl)) {
            return;
        }

        $payload = json_encode([
            'run_id'  => $this->runId,
            'total'   => $total,
            'current' => $current,
            'message' => $message,
        ]);

        $url = $this->apiUrl . '/api/v1/heartbeat';

        $context = stream_context_create([
            'http' => [
                'method'  => 'POST',
                'header'  => "Content-Type: application/json\r\n",
                'content' => $payload,
                'timeout' => 5,
                'ignore_errors' => true,
            ],
        ]);

        $result = @file_get_contents($url, false, $context);

        if ($result === false) {
            $this->log("Heartbeat failed: could not reach $url");
        }
    }

    /**
     * Convenience: send a heartbeat with just a message (no progress tracking).
     */
    public function ping(string $message = ''): void
    {
        $this->heartbeat(0, 0, $message);
    }

    private function log(string $message): void
    {
        fwrite(STDERR, "[CronControl] $message\n");
    }
}
