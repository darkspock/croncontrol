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

  // -- Run Result --
  setResult(runId, data) { return this._request('PATCH', `/runs/${runId}/result`, { body: data }); }
  getResult(runId) { return this._request('GET', `/runs/${runId}/result`); }

  // -- Secrets --
  listSecrets() { return this._request('GET', '/secrets'); }
  createSecret(name, value) { return this._request('POST', '/secrets', { body: { name, value } }); }
  updateSecret(name, value) { return this._request('PUT', `/secrets/${name}`, { body: { value } }); }
  deleteSecret(name) { return this._request('DELETE', `/secrets/${name}`); }

  // -- Artifacts --
  listArtifacts(runId) { return this._request('GET', `/runs/${runId}/artifacts`); }
  getArtifactUrl(runId, name) { return `${this.baseUrl}/api/v1/runs/${runId}/artifacts/${name}`; }

  // -- Orchestras --
  createOrchestra(data) { return this._request('POST', '/orchestras', { body: data }); }
  getScore(orchestraId) { return this._request('GET', `/orchestras/${orchestraId}/score`); }
  finishOrchestra(id, summary = '') { return this._request('POST', `/orchestras/${id}/finish`, { body: { summary } }); }
  cancelOrchestra(id) { return this._request('POST', `/orchestras/${id}/cancel`); }
  nextMovement(runId, processId, payload) { return this._request('POST', `/runs/${runId}/next`, { body: { process_id: processId, payload } }); }
  askChoice(runId, message, choices) { return this._request('POST', `/runs/${runId}/choice`, { body: { message, choices } }); }
  getEvent() {
    return {
      type: process.env.CRONCONTROL_EVENT_TYPE || '',
      orchestraId: process.env.CRONCONTROL_ORCHESTRA_ID || '',
      step: parseInt(process.env.CRONCONTROL_ORCHESTRA_STEP || '0'),
      runId: process.env.CRONCONTROL_EVENT_RUN_ID || '',
      result: JSON.parse(process.env.CRONCONTROL_EVENT_RESULT || 'null'),
    };
  }
  postChat(orchestraId, content, type = 'text') { return this._request('POST', `/orchestras/${orchestraId}/chat`, { body: { content, message_type: type } }); }
  getChat(orchestraId) { return this._request('GET', `/orchestras/${orchestraId}/chat`); }

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
