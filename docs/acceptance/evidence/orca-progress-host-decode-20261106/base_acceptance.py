import hashlib
import json
import sqlite3
import subprocess
import sys
import time
import uuid
from pathlib import Path


ROOT = Path(__file__).resolve().parent
PREVIOUS = Path('C:\\Users\\lmas1\\.codex\\worktrees\\d634\\SteerRunner\\.evidence\\helper-retirement-final')
FIRST = "RPC literal marker: 中文 😀\r\n\r\npath\\with space"
FEEDBACK = "RPC_CONTINUE\r\n\r\n续轮 😀"


class Run:
    def __init__(self, side, case):
        self.side = side
        self.case = case
        self.name = f"{side}-{case}"
        self.out = ROOT / (self.name + "-" + uuid.uuid4().hex[:8])
        self.out.mkdir()
        self.state = self.out / "state"
        self.sequence = 0
        self.run_id = ""
        self.instance = ""

    def call(self, *args, input=None, allow_error=False):
        self.sequence += 1
        cmd = [str(ROOT / "writer.exe"), *args, "--state-dir", str(self.state), "--json"]
        body = None
        if input is not None:
            cmd.extend(["--input", "-"])
            body = json.dumps({"protocol": "steerrunner-cli/v1", "request_id": uuid.uuid4().hex, "input": input}).encode()
        proc = subprocess.run(cmd, input=body, capture_output=True, timeout=75, creationflags=subprocess.CREATE_NO_WINDOW)
        path = self.out / f"{self.sequence:03d}-{'-'.join(args[:2])}.json"
        path.write_bytes(proc.stdout)
        path.with_suffix(".stderr").write_bytes(proc.stderr)
        reply = json.loads(proc.stdout)
        if not reply.get("ok") and not allow_error:
            raise RuntimeError((self.name, args[:2], reply.get("error")))
        return reply.get("data") or {}

    def run_call(self, command, **kwargs):
        return self.call(command, "--run-id", self.run_id, "--expected-instance-id", self.instance, **kwargs)

    def inspect(self):
        return self.run_call("inspect")["run"]

    def wait(self, predicate, seconds=190):
        deadline = time.monotonic() + seconds
        previous = None
        while time.monotonic() < deadline:
            row = self.inspect()
            status = tuple(row.get(key) for key in ("status", "phase", "stop_status", "diagnostic"))
            if status != previous:
                print(self.name, status, flush=True)
                previous = status
            if predicate(row):
                return row
            if row.get("status") == "blocked" or row.get("stop_status") in ("unknown", "unsupported"):
                raise RuntimeError((self.name, status))
            time.sleep(2)
        raise RuntimeError((self.name, "wait expired", previous))

    def request(self):
        if self.case == "execute":
            seed = PREVIOUS / f"p1-{self.side}-matrix" / "normal-request.json"
        else:
            seed = Path('C:\\Users\\lmas1\\.codex\\worktrees\\d634\\SteerRunner\\.evidence\\p2-fixed-live\\formal') / f"{self.side}-request.json"
        data = json.loads(seed.read_text(encoding="utf-8-sig"))["input"]
        data["title"] = "Isolated structured session RPC " + self.name
        data["name_prefix"] = "rpc-" + self.name
        data["result_file"] = {"initial_content_base64": "cnBjLXNlZWQ="}
        settings = data["execution"]["settings"]
        settings.pop("agent_launch", None)
        settings.pop("agent", None)
        if self.case == "pi":
            settings["agent"] = "pi"
        if self.case == "execute":
            prompt = "Isolated transport smoke. Do not call tools or write files. Return exactly three plain text lines: RPC EXEC FIRST, RPC EXEC SECOND, RPC EXEC THIRD.\r\n\r\n" + FIRST
            data["workflow"]["prompt_overrides"] = {"execute": prompt}
        else:
            prompt = (
                "Isolated transport smoke. Do not call tools, modify files, commit or publish. "
                "If caller feedback includes the marker RPC_CONTINUE, return exactly two plain text lines: RPC DEV SECOND and 未完成. "
                "Otherwise return exactly two plain text lines: RPC DEV FIRST and 未完成.\r\n\r\n" + FIRST
            )
            data["workflow"]["prompt_overrides"] = {"implement.execute": prompt}
        return data

    def business(self):
        self.instance = self.call("service", "start")["instance_id"]
        data = self.request()
        self.run_id = self.call("submit", "--expected-instance-id", self.instance, input=data)["run_id"]
        print(self.name, "run", self.run_id, self.out, flush=True)
        if self.case == "execute":
            row = self.wait(lambda row: row.get("stop_status") == "stopped" and row.get("status") == "completed")
            assert row["final"]["verified"] and row["final"]["text"].strip().splitlines() == ["RPC EXEC FIRST", "RPC EXEC SECOND", "RPC EXEC THIRD"], row.get("final")
        else:
            def final(row, marker):
                return row.get("status") == "waiting_input" and row.get("final", {}).get("verified") and row["final"]["text"].strip().splitlines() == [marker, "未完成"]
            first = self.wait(lambda row: final(row, "RPC DEV FIRST"))
            assert first["controls"]["can_reply"], first["controls"]
            self.run_call("reply", input={"expected_revision": first["revision"], "kind": "feedback", "text": FEEDBACK})
            second = self.wait(lambda row: final(row, "RPC DEV SECOND"))
            assert first["execution"]["session_id"] == second["execution"]["session_id"], "session changed"
            assert first["execution"]["turn_id"] != second["execution"]["turn_id"], "turn unchanged"
            self.run_call("stop", input={"expected_revision": second["revision"]})
            row = self.wait(lambda row: row.get("stop_status") == "stopped")
        assert row["stop_evidence_scope"] == "orca_agent_session" and not row["resume_required"] and not row["new_inputs_allowed"] and not row["controls"]["can_resume"], row["controls"]
        file = self.run_call("result-file")
        contract = "steerrunner-result-file-snapshot/v1" if self.case == "pi" else "steerrunner-result-file/v1"
        assert (file["contract"], file["version"], file["collection_status"], file["sha256"]) == (contract, 1, "closed", hashlib.sha256(b"rpc-seed").hexdigest()), file
        return {"name": self.name, "run": self.run_id, "file": {key: file[key] for key in ("contract", "version", "sha256")}}

    def finish(self):
        if not self.instance:
            return
        try:
            if self.run_id:
                row = self.inspect()
                # Use a fresh stop only for Runs without a prior stop operation.
                if row["controls"]["can_stop"] and row.get("stop_status") not in ("pending", "unknown", "unsupported") and not any(op.get("action") == "stop_execution" for op in row.get("operations", [])):
                    self.run_call("stop", input={"expected_revision": row["revision"]})
                    self.wait(lambda row: row.get("stop_status") == "stopped", seconds=40)
        finally:
            self.call("service", "stop", input={"expected_instance_id": self.instance})
        for _ in range(40):
            info = self.call("service", "status")
            if info.get("status") == "not_running":
                break
            time.sleep(1)
        else:
            raise RuntimeError("original writer did not exit")
        proof = self.call("service", "verify-stopped")
        assert proof["resources_settled"] and len(proof["runs"]) == 1 and proof["runs"][0]["run_id"] == self.run_id, proof

    def original_inputs(self):
        uri = (self.state / "runner.db").resolve().as_uri() + "?mode=ro"
        with sqlite3.connect(uri, uri=True) as store:
            row = json.loads(store.execute("SELECT body FROM runs WHERE id=?", (self.run_id,)).fetchone()[0])
        turns = []
        for intent in row["intents"]:
            request = intent["request"]
            receipt = intent.get("receipt") or {}
            if request["Operation"]["action"] != "start_turn":
                continue
            fact = receipt.get("state", {})
            payload = next(envelope["payload"] for envelope in receipt["implementation"] if envelope["implementation_key"] == "orca.agent-session-state")
            prompt = request["Input"]["turn"]["Prompt"]
            assert fact["input_accepted"] and fact["turn_started"] and fact["turn_id"] == payload["turn_id"] and fact["session_id"] == payload["session_id"], fact
            assert payload["input_digest"] == hashlib.sha256(prompt.encode()).hexdigest() and payload["message_id"] and payload["turn_item_id"], "input proof changed"
            turns.append({"runtime": payload["runtime_id"], "worktree": payload["worktree"]["id"], **{key: payload[key] for key in ("session_id", "turn_id", "message_id", "input_digest")}})
            if len(turns) == 1:
                assert FIRST in prompt, "first multiline marker absent"
            else:
                assert FEEDBACK in prompt, "follow-up multiline marker absent"
        assert len(turns) == (1 if self.case == "execute" else 2), turns
        assert len({turn["session_id"] for turn in turns}) == 1 and len({turn["turn_id"] for turn in turns}) == len(turns) and len({turn["message_id"] for turn in turns}) == len(turns), turns
        return turns

    def verify(self):
        try:
            result = self.business()
        finally:
            self.finish()
        result["turns"] = self.original_inputs()
        result["state"] = str(self.state)
        (self.out / "passed.json").write_text(json.dumps(result, indent=2), encoding="utf-8")
        print("PASS", self.name, self.run_id, flush=True)


if __name__ == "__main__":
    matrix = [("local", "execute"), ("remote", "execute"), ("local", "development"), ("remote", "development"), ("local", "pi")]
    failures = []
    for side, case in matrix:
        if sys.argv[1:] and f"{side}-{case}" not in sys.argv[1:]:
            continue
        test = Run(side, case)
        try:
            test.verify()
        except Exception as error:
            failures.append(test.name)
            print("FAIL", test.name, repr(error), test.out, flush=True)
    if failures:
        raise SystemExit("Blocked: " + ", ".join(failures))
