/**
 * Example usage of the CronControl Node.js SDK.
 *
 * Run: CRONCONTROL_URL=http://localhost:8080 CRONCONTROL_API_KEY=cc_live_... node example.mjs
 */

import { CronControl } from './croncontrol.js';

const cc = new CronControl();

// List processes
const { data: processes } = await cc.listProcesses();
for (const proc of processes) {
  console.log(`  ${proc.name} (${proc.schedule_type})`);
}

// Trigger a run
const run = await cc.triggerProcess('prc_01HYX...');
console.log(`Run started: ${run.data.id}`);

// Enqueue a job
const job = await cc.enqueueJob({
  queue_id: 'que_01HYX...',
  payload: { to: 'user@example.com', subject: 'Hello' },
  reference: 'order-12345',
});
console.log(`Job enqueued: ${job.data.id}`);

// Report heartbeat
for (let i = 0; i < 100; i++) {
  await cc.heartbeat('run_01HYX...', 100, i + 1, `Step ${i + 1}`);
}

// Health check
const health = await cc.health();
console.log(`Status: ${health.status}`);
