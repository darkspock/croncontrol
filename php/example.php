<?php
/**
 * Example: Using CronControl heartbeat in a PHP cron script.
 *
 * When CronControl launches this script, it sets:
 *   CRONCONTROL_RUN_ID=run_01JABC...
 *   CRONCONTROL_API_URL=https://your-instance.croncontrol.io
 */

require_once __DIR__ . '/CronControl.php';

$cc = new CronControl();
$total = 1000;

$cc->heartbeat($total, 0, "Starting data import...");

for ($i = 0; $i < $total; $i++) {
    // Simulate work
    usleep(10000); // 10ms per item

    // Report progress every 100 items
    if ($i % 100 === 0) {
        $cc->heartbeat($total, $i, "Processing item $i of $total");
    }
}

$cc->heartbeat($total, $total, "Import complete");

echo "Done! Processed $total items.\n";
