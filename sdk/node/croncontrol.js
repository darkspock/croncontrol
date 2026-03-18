/**
 * CronControl Node.js SDK
 *
 * Thin wrapper around the CronControl REST API.
 * Zero dependencies — uses native fetch (Node 18+).
 *
 * Usage:
 *   const { CronControl } = require('./croncontrol');
 *   const cc = new CronControl('http://localhost:8080', 'cc_live_...');
 *   const processes = await cc.listProcesses();
 *   await cc.triggerProcess('prc_01HYX...');
 */

class CronControlError extends Error {
  constructor(status, code, message, hint) {
    super(message);
    this.name = 'CronControlError';
    this.status = status;
    this.code = code;
    this.hint = hint || '';
  }
}

class CronControl {
  /**
   * @param {string} [baseUrl] - CronControl server URL (or CRONCONTROL_URL env)
   * @param {string} [apiKey] - API key (or CRONCONTROL_API_KEY env)
   */
  constructor(baseUrl, apiKey) {
    this.baseUrl = (baseUrl || process.env.CRONCONTROL_URL || 'http://localhost:8080').replace(/\/$/, '');
    this.apiKey = apiKey || process.env.CRONCONTROL_API_KEY || '';
  }

  async _request(method, path, { body, params } = {}) {
    let url = `${this.baseUrl}/api/v1${path}`;
    if (params) {
      const qs = new URLSearchParams(
        Object.entries(params).filter(([, v]) => v != null)
      ).toString();
      if (qs) url += `?${qs}`;
    }

    const headers = { 'Content-Type': 'application/json' };
    if (this.apiKey) headers['X-API-Key'] = this.apiKey;

    const res = await fetch(url, {
      method,
      headers,
      body: body ? JSON.stringify(body) : undefined,
    });

    if (res.status === 204) return undefined;

    const data = await res.json().catch(() => null);

    if (!res.ok) {
      const err = data?.error || {};
      throw new CronControlError(
        res.status,
        err.code || 'UNKNOWN',
        err.message || res.statusText,
        err.hint
      );
    }

    return data;
  }

  // -- Workspace --
  getWorkspace() { return this._request('GET', '/workspace'); }

  // -- Processes --
  listProcesses(params) { return this._request('GET', '/processes', { params }); }
  getProcess(id) { return this._request('GET', `/processes/${id}`); }
  createProcess(data) { return this._request('POST', '/processes', { body: data }); }
  updateProcess(id, data) { return this._request('PUT', `/processes/${id}`, { body: data }); }
  deleteProcess(id) { return this._request('DELETE', `/processes/${id}`); }
  triggerProcess(id) { return this._request('POST', `/processes/${id}/trigger`); }
  pauseProcess(id, cancelPending = false) {
    return this._request('POST', `/processes/${id}/pause`, { params: { cancel_pending: cancelPending } });
  }
  resumeProcess(id) { return this._request('POST', `/processes/${id}/resume`); }

  // -- Runs --
  listRuns(params) { return this._request('GET', '/runs', { params }); }
  getRun(id) { return this._request('GET', `/runs/${id}`); }
  cancelRun(id) { return this._request('POST', `/runs/${id}/cancel`); }
  killRun(id) { return this._request('POST', `/runs/${id}/kill`); }
  replayRun(id) { return this._request('POST', `/runs/${id}/replay`); }
  getRunOutput(id, stream) {
    return this._request('GET', `/runs/${id}/output`, { params: stream ? { stream } : undefined });
  }

  // -- Queues --
  listQueues() { return this._request('GET', '/queues'); }
  getQueue(id) { return this._request('GET', `/queues/${id}`); }
  createQueue(data) { return this._request('POST', '/queues', { body: data }); }

  // -- Jobs --
  listJobs(params) { return this._request('GET', '/jobs', { params }); }
  getJob(id) { return this._request('GET', `/jobs/${id}`); }
  enqueueJob(data) { return this._request('POST', '/jobs', { body: data }); }
  cancelJob(id) { return this._request('POST', `/jobs/${id}/cancel`); }
  replayJob(id, overrides) {
    return this._request('POST', `/jobs/${id}/replay`, { body: overrides || undefined });
  }

  // -- Workers --
  listWorkers() { return this._request('GET', '/workers'); }
  createWorker(data) { return this._request('POST', '/workers', { body: data }); }
  deleteWorker(id) { return this._request('DELETE', `/workers/${id}`); }

  // -- API Keys --
  listApiKeys() { return this._request('GET', '/api-keys'); }
  createApiKey(data) { return this._request('POST', '/api-keys', { body: data }); }
  deleteApiKey(id) { return this._request('DELETE', `/api-keys/${id}`); }

  // -- Heartbeat (no auth) --
  heartbeat(runId, total, current, message = '') {
    return this._request('POST', '/heartbeat', {
      body: { run_id: runId, total, current, message },
    });
  }

  // -- Health --
  health() { return this._request('GET', '/health'); }
}

module.exports = { CronControl, CronControlError };
