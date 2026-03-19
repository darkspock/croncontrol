"""
CronControl Python SDK

Thin wrapper around the CronControl REST API.
Zero dependencies beyond the standard library.

Usage:
    from croncontrol import CronControl

    cc = CronControl("http://localhost:8080", "cc_live_...")
    processes = cc.list_processes()
    cc.trigger_process("prc_01HYX...")
    cc.heartbeat("run_01HYX...", total=100, current=50, message="Halfway")
"""

import json
import os
import urllib.request
import urllib.error
import urllib.parse
from typing import Any, Optional


class CronControlError(Exception):
    """API error with structured code, message, and hint."""

    def __init__(self, status: int, code: str, message: str, hint: str = ""):
        super().__init__(message)
        self.status = status
        self.code = code
        self.hint = hint


class CronControl:
    """CronControl API client."""

    def __init__(
        self,
        base_url: Optional[str] = None,
        api_key: Optional[str] = None,
        timeout: int = 30,
    ):
        self.base_url = (base_url or os.environ.get("CRONCONTROL_URL", "http://localhost:8080")).rstrip("/")
        self.api_key = api_key or os.environ.get("CRONCONTROL_API_KEY", "")
        self.timeout = timeout

    def _request(self, method: str, path: str, body: Any = None, params: Optional[dict] = None) -> Any:
        url = f"{self.base_url}/api/v1{path}"
        if params:
            url += "?" + urllib.parse.urlencode({k: v for k, v in params.items() if v is not None})

        data = json.dumps(body).encode() if body else None
        headers = {"Content-Type": "application/json"}
        if self.api_key:
            headers["X-API-Key"] = self.api_key

        req = urllib.request.Request(url, data=data, headers=headers, method=method)
        try:
            with urllib.request.urlopen(req, timeout=self.timeout) as resp:
                if resp.status == 204:
                    return None
                return json.loads(resp.read())
        except urllib.error.HTTPError as e:
            try:
                err = json.loads(e.read()).get("error", {})
            except Exception:
                err = {"code": "UNKNOWN", "message": str(e)}
            raise CronControlError(e.code, err.get("code", "UNKNOWN"), err.get("message", str(e)), err.get("hint", ""))

    # -- Workspace --
    def get_workspace(self) -> dict:
        return self._request("GET", "/workspace")

    # -- Processes --
    def list_processes(self, **params) -> dict:
        return self._request("GET", "/processes", params=params)

    def get_process(self, process_id: str) -> dict:
        return self._request("GET", f"/processes/{process_id}")

    def create_process(self, **data) -> dict:
        return self._request("POST", "/processes", body=data)

    def update_process(self, process_id: str, **data) -> dict:
        return self._request("PUT", f"/processes/{process_id}", body=data)

    def delete_process(self, process_id: str) -> None:
        self._request("DELETE", f"/processes/{process_id}")

    def trigger_process(self, process_id: str) -> dict:
        return self._request("POST", f"/processes/{process_id}/trigger")

    def pause_process(self, process_id: str, cancel_pending: bool = False) -> None:
        self._request("POST", f"/processes/{process_id}/pause", params={"cancel_pending": str(cancel_pending).lower()})

    def resume_process(self, process_id: str) -> None:
        self._request("POST", f"/processes/{process_id}/resume")

    # -- Runs --
    def list_runs(self, **params) -> dict:
        return self._request("GET", "/runs", params=params)

    def get_run(self, run_id: str) -> dict:
        return self._request("GET", f"/runs/{run_id}")

    def cancel_run(self, run_id: str) -> None:
        self._request("POST", f"/runs/{run_id}/cancel")

    def kill_run(self, run_id: str) -> None:
        self._request("POST", f"/runs/{run_id}/kill")

    def replay_run(self, run_id: str) -> dict:
        return self._request("POST", f"/runs/{run_id}/replay")

    def get_run_output(self, run_id: str, stream: Optional[str] = None) -> dict:
        return self._request("GET", f"/runs/{run_id}/output", params={"stream": stream} if stream else None)

    # -- Queues --
    def list_queues(self) -> dict:
        return self._request("GET", "/queues")

    def get_queue(self, queue_id: str) -> dict:
        return self._request("GET", f"/queues/{queue_id}")

    def create_queue(self, **data) -> dict:
        return self._request("POST", "/queues", body=data)

    # -- Jobs --
    def list_jobs(self, **params) -> dict:
        return self._request("GET", "/jobs", params=params)

    def get_job(self, job_id: str) -> dict:
        return self._request("GET", f"/jobs/{job_id}")

    def enqueue_job(self, **data) -> dict:
        return self._request("POST", "/jobs", body=data)

    def cancel_job(self, job_id: str) -> None:
        self._request("POST", f"/jobs/{job_id}/cancel")

    def replay_job(self, job_id: str, **overrides) -> dict:
        return self._request("POST", f"/jobs/{job_id}/replay", body=overrides or None)

    # -- Workers --
    def list_workers(self) -> dict:
        return self._request("GET", "/workers")

    def create_worker(self, **data) -> dict:
        return self._request("POST", "/workers", body=data)

    def delete_worker(self, worker_id: str) -> None:
        self._request("DELETE", f"/workers/{worker_id}")

    # -- API Keys --
    def list_api_keys(self) -> dict:
        return self._request("GET", "/api-keys")

    def create_api_key(self, **data) -> dict:
        return self._request("POST", "/api-keys", body=data)

    def delete_api_key(self, key_id: str) -> None:
        self._request("DELETE", f"/api-keys/{key_id}")

    # -- Run Result --
    def set_result(self, run_id: str, data: Any) -> None:
        self._request("PATCH", f"/runs/{run_id}/result", body=data)

    def get_result(self, run_id: str) -> Any:
        return self._request("GET", f"/runs/{run_id}/result")

    # -- Secrets --
    def list_secrets(self) -> dict:
        return self._request("GET", "/secrets")

    def create_secret(self, name: str, value: str) -> dict:
        return self._request("POST", "/secrets", body={"name": name, "value": value})

    def update_secret(self, name: str, value: str) -> None:
        self._request("PUT", f"/secrets/{name}", body={"value": value})

    def delete_secret(self, name: str) -> None:
        self._request("DELETE", f"/secrets/{name}")

    # -- Artifacts --
    def upload_artifact(self, run_id: str, name: str, file_bytes: bytes, content_type: str = "application/octet-stream") -> dict:
        """Upload a file artifact to a run. Uses multipart form upload."""
        import urllib.request
        boundary = "----CronControlBoundary"
        body = (
            f"--{boundary}\r\n"
            f'Content-Disposition: form-data; name="name"\r\n\r\n{name}\r\n'
            f"--{boundary}\r\n"
            f'Content-Disposition: form-data; name="file"; filename="{name}"\r\n'
            f"Content-Type: {content_type}\r\n\r\n"
        ).encode() + file_bytes + f"\r\n--{boundary}--\r\n".encode()

        url = f"{self.base_url}/api/v1/runs/{run_id}/artifacts"
        headers = {"Content-Type": f"multipart/form-data; boundary={boundary}"}
        if self.api_key:
            headers["X-API-Key"] = self.api_key
        req = urllib.request.Request(url, data=body, headers=headers, method="POST")
        with urllib.request.urlopen(req, timeout=self.timeout) as resp:
            return json.loads(resp.read())

    def list_artifacts(self, run_id: str) -> dict:
        return self._request("GET", f"/runs/{run_id}/artifacts")

    def get_artifact_url(self, run_id: str, name: str) -> str:
        return f"{self.base_url}/api/v1/runs/{run_id}/artifacts/{name}"

    # -- Orchestras --
    def create_orchestra(self, name: str, **kwargs) -> dict:
        return self._request("POST", "/orchestras", body={"name": name, **kwargs})

    def get_score(self, orchestra_id: str) -> dict:
        return self._request("GET", f"/orchestras/{orchestra_id}/score")

    def finish_orchestra(self, orchestra_id: str, summary: str = "") -> None:
        self._request("POST", f"/orchestras/{orchestra_id}/finish", body={"summary": summary})

    def cancel_orchestra(self, orchestra_id: str) -> None:
        self._request("POST", f"/orchestras/{orchestra_id}/cancel")

    def next_movement(self, run_id: str, process_id: str, **kwargs) -> dict:
        return self._request("POST", f"/runs/{run_id}/next", body={"process_id": process_id, **kwargs})

    def ask_choice(self, run_id: str, message: str, choices: list) -> None:
        self._request("POST", f"/runs/{run_id}/choice", body={"message": message, "choices": choices})

    def ask_confirm(self, run_id: str, message: str, on_approve: str, on_reject: Optional[str] = None) -> None:
        self.ask_choice(run_id, message, [
            {"label": "Approve", "process_id": on_approve, "style": "primary"},
            {"label": "Reject", "process_id": on_reject, "style": "danger"},
        ])

    def get_event(self) -> dict:
        """Read orchestra event from env vars (for directors)."""
        return {
            "type": os.environ.get("CRONCONTROL_EVENT_TYPE", ""),
            "orchestra_id": os.environ.get("CRONCONTROL_ORCHESTRA_ID", ""),
            "step": int(os.environ.get("CRONCONTROL_ORCHESTRA_STEP", "0")),
            "run_id": os.environ.get("CRONCONTROL_EVENT_RUN_ID", ""),
            "result": json.loads(os.environ.get("CRONCONTROL_EVENT_RESULT", "null")),
        }

    # -- Chat --
    def post_chat(self, orchestra_id: str, content: str, message_type: str = "text", **kwargs) -> dict:
        return self._request("POST", f"/orchestras/{orchestra_id}/chat",
            body={"content": content, "message_type": message_type, **kwargs})

    def get_chat(self, orchestra_id: str) -> dict:
        return self._request("GET", f"/orchestras/{orchestra_id}/chat")

    # -- Heartbeat --
    def heartbeat(self, run_id: str, total: int, current: int, message: str = "") -> None:
        """Report progress for a running execution. No auth required."""
        self._request("POST", "/heartbeat", body={
            "run_id": run_id,
            "total": total,
            "current": current,
            "message": message,
        })

    # -- Health --
    def health(self) -> dict:
        return self._request("GET", "/health")
