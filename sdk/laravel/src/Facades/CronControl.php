<?php

namespace CronControl\Laravel\Facades;

use CronControl\Laravel\CronControlClient;
use Illuminate\Support\Facades\Facade;

/**
 * @method static array getWorkspace()
 * @method static array listProcesses(array $params = [])
 * @method static array getProcess(string $id)
 * @method static array createProcess(array $data)
 * @method static array triggerProcess(string $id)
 * @method static void pauseProcess(string $id, bool $cancelPending = false)
 * @method static void resumeProcess(string $id)
 * @method static array listRuns(array $params = [])
 * @method static array getRun(string $id)
 * @method static void cancelRun(string $id)
 * @method static void killRun(string $id)
 * @method static array replayRun(string $id)
 * @method static array listQueues()
 * @method static array createQueue(array $data)
 * @method static array listJobs(array $params = [])
 * @method static array getJob(string $id)
 * @method static array enqueueJob(array $data)
 * @method static void cancelJob(string $id)
 * @method static array replayJob(string $id, array $overrides = [])
 * @method static void heartbeat(string $runId, int $total, int $current, string $message = '')
 * @method static array health()
 *
 * @see CronControlClient
 */
class CronControl extends Facade
{
    protected static function getFacadeAccessor(): string
    {
        return CronControlClient::class;
    }
}
