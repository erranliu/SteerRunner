import base64
import hashlib
import importlib.util
import json
import time
import subprocess
from pathlib import Path

ROOT = Path(__file__).resolve().parent
old = ROOT / "base_acceptance.py"
spec = importlib.util.spec_from_file_location("previous_acceptance", old)
module = importlib.util.module_from_spec(spec)
spec.loader.exec_module(module)
module.ROOT = ROOT


class FileRun(module.Run):
    def request(self):
        data = super().request()
        data["title"] = "ProgressHost decode equivalence " + self.side
        data["name_prefix"] = "progress-host-" + self.side
        data["progress_file"] = {"initial_content_base64": base64.b64encode(b"initial-progress").decode()}
        data["result_file"] = {"initial_content_base64": base64.b64encode(b"initial-result").decode()}
        self.progress = ("FILE_PROGRESS_" + self.side.upper()).encode()
        self.result = ("FILE_RESULT_" + self.side.upper()).encode()
        data["workflow"]["prompt_overrides"] = {"implement.execute": (
            "Isolated file-channel check. Use only this fixed worktree. Resolve both original file bindings "
            "to absolute paths in one PowerShell command. Write the exact ASCII text "
            + self.progress.decode()
            + " to the progress file and "
            + self.result.decode()
            + " to the result file using sibling temp files and Move-Item -Force. "
            "Then in that same tool command run Start-Sleep -Seconds 45 and wait for it to finish. "
            "Do not modify other files, commit, push or publish. Finally return two lines: FILES_DONE_"
            + self.side.upper() + " and 未完成.\r\n\r\n" + module.FIRST
        )}
        (self.out / "original-input.json").write_text(json.dumps(data, indent=2), encoding="utf-8")
        checked = subprocess.run([str(ROOT / "writer.exe"), "validate", "--kind", "run", "--input", "-", "--json"], input=json.dumps(data).encode(), capture_output=True, timeout=30, creationflags=subprocess.CREATE_NO_WINDOW)
        (self.out / "validation.json").write_bytes(checked.stdout)
        assert checked.returncode == 0 and json.loads(checked.stdout)["ok"], checked.stdout.decode()
        return data

    def business(self):
        self.instance = self.call("service", "start")["instance_id"]
        self.run_id = self.call("submit", "--expected-instance-id", self.instance, input=self.request())["run_id"]
        print(self.name, self.run_id, self.out, flush=True)
        seen_active = False
        previous = None
        until = time.monotonic() + 260
        final = None
        while time.monotonic() < until:
            before = self.inspect()
            status = (before["status"], before["phase"], before.get("diagnostic"))
            if status != previous:
                print(self.name, status, flush=True)
                previous = status
            if before["status"] == "blocked" or before.get("stop_status") in ("unknown", "unsupported"):
                raise RuntimeError((self.name, status))
            if before["status"] == "running" and before["execution"].get("turn_id") and not seen_active:
                progress = self.run_call("progress-file")
                if progress.get("content_base64") == base64.b64encode(self.progress).decode():
                    result = self.run_call("result-file")
                    after = self.inspect()
                    assert after["status"] == "running" and before["execution"]["turn_id"] == after["execution"]["turn_id"], "active window ended"
                    assert progress["sha256"] == hashlib.sha256(self.progress).hexdigest()
                    assert result["collection_status"] == "pending" and not result.get("content_base64"), result
                    (self.out / "active-file-proof.json").write_text(json.dumps({
                        "run": self.run_id, "turn": after["execution"]["turn_id"],
                        "progress_sha256": progress["sha256"], "progress_version": progress["version"],
                        "result_status": result["collection_status"]
                    }, indent=2), encoding="utf-8")
                    seen_active = True
                    print(self.name, "new progress observed while result pending", flush=True)
            if before["status"] == "waiting_input" and before.get("final", {}).get("verified"):
                final = before
                break
            time.sleep(2)
        assert final and seen_active, (self.name, "missing active or completed proof", previous)
        assert final["final"]["verified"] and final["final"]["text"].strip().splitlines() == ["FILES_DONE_" + self.side.upper(), "未完成"], final["final"]
        self.run_call("stop", input={"expected_revision": final["revision"]})
        final = self.wait(lambda row: row.get("stop_status") == "stopped")
        assert final["stop_evidence_scope"] == "orca_agent_session" and not final["resume_required"] and not final["new_inputs_allowed"]
        result = self.run_call("result-file")
        until = time.monotonic() + 25
        while result["collection_status"] != "closed" and time.monotonic() < until:
            time.sleep(1)
            result = self.run_call("result-file")
        again = self.run_call("result-file")
        assert result["contract"] == "steerrunner-result-file/v1" and result["collection_status"] == "closed" and result["version"] == 1, result
        assert result["content_base64"] == base64.b64encode(self.result).decode() and result["sha256"] == hashlib.sha256(self.result).hexdigest()
        for key in ("contract", "content_base64", "observed_at", "version", "sha256"):
            assert result[key] == again[key], "sealed result changed"
        return {"name": self.name, "run": self.run_id, "result_sha256": result["sha256"], "result_version": result["version"]}

    def original_inputs(self):
        self.case = "execute"
        try:
            return super().original_inputs()
        finally:
            self.case = "development"


failures = []
for side in ("local", "remote"):
    test = FileRun(side, "development")
    try:
        test.verify()
    except Exception as error:
        failures.append(side)
        print("FAIL", side, repr(error), test.out, flush=True)
if failures:
    raise SystemExit("Blocked: " + ", ".join(failures))
