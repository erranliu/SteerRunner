"""Six bounded fresh formal Runner runs; no direct native input or business tests."""
import argparse
import base64
import copy
import datetime as dt
import hashlib
import json
import os
from pathlib import Path
import sqlite3
import subprocess
import time
import uuid


def now():
    return dt.datetime.now(dt.timezone.utc).isoformat()


def digest(data):
    return hashlib.sha256(data).hexdigest()


def load(path):
    return json.loads(Path(path).read_text(encoding="utf-8-sig"))


def save(path, value):
    Path(path).write_bytes((json.dumps(value, ensure_ascii=False, indent=2)+"\n").encode())


def native_text(text):
    return text.replace("\r\n", "\n").replace("\r", "\n").rstrip("\n")


def submit_digest(request):
    # storage.RequestDigest: sorted JSON map inside an ordered Go struct.
    body = json.dumps(request, ensure_ascii=False, sort_keys=True, separators=(",", ":"))
    body = body.replace("&", "\\u0026").replace("<", "\\u003c").replace(">", "\\u003e").replace("\u2028", "\\u2028").replace("\u2029", "\\u2029")
    return digest(('{"Protocol":"steerrunner-cli/v1","Command":"submit","Target":"","Input":'+body+'}').encode())


def complete_zero(value):
    return (value.get("totalCount") == 0 and value.get("terminals") == []
            and value.get("truncated") is False and bool(value["hostScope"]["hostIds"])
            and value["hostScope"]["omittedHostIds"] == [])


class Case:
    def __init__(self, args, baseline, root, state, ordinal, group, pair):
        self.args, self.baseline, self.root, self.state = args, baseline, root, state
        self.ordinal, self.group, self.pair = ordinal, group, pair
        self.count, self.instance, self.run_id = 0, None, None
        self.worktree, self.process = None, None
        self.result = dict(ordinal=ordinal, pair=pair, group=group, root=root.name,
                           started_at=now(), passed=False, reached_publication=False,
                           cleanup={}, runner_sha256=args.runner_sha, state_dir=str(state))

    def command(self, argv, body=None, timeout=90, label="call"):
        self.count += 1
        begin = now()
        payload = None if body is None else json.dumps(body, ensure_ascii=False).encode()
        record = dict(started_at=begin, argv=argv, stdin=body)
        try:
            proc = subprocess.run(argv, input=payload, capture_output=True, timeout=timeout,
                                  creationflags=subprocess.CREATE_NO_WINDOW)
            record.update(exit_code=proc.returncode, stdout=proc.stdout.decode("utf-8-sig", errors="replace"),
                          stderr=proc.stderr.decode("utf-8", errors="replace"),
                          stdout_base64=base64.b64encode(proc.stdout).decode(), stderr_base64=base64.b64encode(proc.stderr).decode())
        except subprocess.TimeoutExpired as exc:
            record.update(timed_out_at=now(), exit_code=None,
                          stdout=(exc.stdout or b"").decode("utf-8", errors="replace"),
                          stderr=(exc.stderr or b"").decode("utf-8", errors="replace"),
                          stdout_base64=base64.b64encode(exc.stdout or b"").decode(), stderr_base64=base64.b64encode(exc.stderr or b"").decode())
            raise
        finally:
            record["finished_at"] = now()
            save(self.root / f"{self.count:03d}-{label}.json", record)
        return proc

    def runner(self, command, body=None, request_id=None, extra=(), check=True):
        argv = [self.args.runner, *command, "--state-dir", str(self.state), "--json", *extra]
        if self.instance and command[0] != "service":
            argv += ["--expected-instance-id", self.instance]
        if body is not None:
            argv += ["--input", "-"]
            body = dict(protocol="steerrunner-cli/v1", request_id=request_id or uuid.uuid4().hex, input=body)
        proc = self.command(argv, body, label="runner")
        value = json.loads(proc.stdout.decode("utf-8-sig"))
        if check and (proc.returncode or not value.get("ok")):
            raise RuntimeError(value)
        return value

    def orca(self, arguments, label):
        # Only read-only remote unavailability is retried; mutations never are.
        readonly = arguments[:2] in (["terminal", "list"], ["terminal", "show"], ["terminal", "read"], ["worktree", "show"], ["worktree", "list"])
        for attempt in range(1, 4):
            request = self.root / f"orca-{label}-attempt{attempt}.request.json"
            if request.exists():
                raise RuntimeError("do not overwrite or replay an owned Orca call")
            save(request, dict(executable=self.args.orca, arguments=[*arguments, "--json", "--environment", self.baseline["execution"]["target"]["environment"]]))
            proc = self.command([self.args.pwsh, "-NoProfile", "-File", self.args.orca_wrapper, "-RequestFile", str(request)], timeout=60, label="orca-"+label)
            value = json.loads(proc.stdout.decode("utf-8-sig"))
            save(self.root / f"orca-{label}-attempt{attempt}.result.json", value)
            if value.get("ok") and value["_meta"]["runtimeId"] != self.args.runtime:
                raise RuntimeError("fixed runtime changed")
            if value.get("ok") or not readonly or value.get("error", {}).get("code") != "remote_runtime_unavailable" or attempt == 3:
                return value
            time.sleep(.7)

    def inspect(self):
        return self.runner(["inspect"], extra=["--run-id", self.run_id])["data"]["run"]

    def reconcile_admission(self, request, original):
        """Read the original durable acceptance receipt; never resend submit."""
        expected = submit_digest(request)
        until = time.monotonic()+60
        while time.monotonic() < until:
            conn = sqlite3.connect((self.state/"runner.db").as_uri()+"?mode=ro", uri=True)
            try:
                row = conn.execute("SELECT digest,command,run_id,command_id,response FROM commands WHERE request_id=?", (self.request_id,)).fetchone()
            finally:
                conn.close()
            if not row:
                time.sleep(.5)
                continue
            stored_digest, command, run_id, command_id, response = row
            receipt = json.loads(response)
            if stored_digest != expected or command != "submit" or not run_id or not command_id or not receipt.get("ok") or receipt["request_id"] != self.request_id:
                raise RuntimeError("original admission receipt identity/full-input mismatch; no replay")
            data = receipt["data"]
            if data["receipt_status"] != "accepted" or data["run_id"] != run_id or data["command_id"] != command_id:
                raise RuntimeError("original durable admission not accepted; no replay")
            view = self.runner(["inspect"], extra=["--request-id", self.request_id])["data"]["run"]
            if view["run_id"] != run_id:
                raise RuntimeError("original request lookup points at another Run")
            self.run_id = run_id
            record = self.stored("read-only-admission-run-record.json")
            if record["submit_command_id"] != command_id or record["snapshot"]["Ref"]["run_id"] != run_id:
                raise RuntimeError("original accepted command/Run binding changed")
            save(self.root/"read-only-admission-proof.json", dict(original_response=original, expected_request_digest=expected,
                 stored_request_digest=stored_digest, command_id=command_id, original_receipt=receipt, lookup=view,
                 expected_instance_id=self.instance, input_resent=False))
            self.result["admission_response_reconciled"] = True
            return run_id
        raise RuntimeError("original admission still unknown after bounded read-only lookup; no replay")

    def stored(self, name):
        conn = sqlite3.connect((self.state/"runner.db").as_uri()+"?mode=ro", uri=True)
        try:
            row = conn.execute("SELECT body FROM runs WHERE id=?", (self.run_id,)).fetchone()
        finally:
            conn.close()
        if not row:
            raise RuntimeError("new owned run record missing")
        raw = row[0].encode() if isinstance(row[0], str) else bytes(row[0])
        (self.root/name).write_bytes(raw)
        value = json.loads(raw)
        ref = value["snapshot"].get("Ref", {})
        external = ref.get("external", {})
        if external:
            self.process = external
            self.worktree = external.get("worktree_id")
        self.worktree = self.worktree or ref.get("target", {}).get("worktree_id")
        return value

    def execute(self):
        self.root.mkdir(exist_ok=False)
        self.state.mkdir(parents=True, exist_ok=False)
        request = copy.deepcopy(self.baseline)
        request["title"] = "Isolated formal publication input length comparison"
        request["name_prefix"] = f"sr-pub3-{self.ordinal:02d}"
        request["repository"]["local_path"] = self.args.repository
        request["workflow"] = dict(id="development", version=2,
            omit_publish_source_facts=self.group == "short", prompt_overrides={
                "implement.execute": Path(self.args.first).read_text(encoding="utf-8"),
                "publish": Path(self.args.publish).read_text(encoding="utf-8").replace("SRPUB01", f"SRPUB{self.pair:02d}")})
        marker = f"SRPUB{self.pair:02d}"
        final = json.dumps(dict(outcome="needs_input", summary=marker, question=marker), separators=(",", ":"))
        save(self.root/"frozen-request.json", request)
        self.request_id = "pub3-"+uuid.uuid4().hex
        self.result.update(request_id=self.request_id, marker=marker)
        save(self.root/"intent.json", self.result)
        try:
            if digest(Path(self.args.runner).read_bytes()) != self.args.runner_sha:
                raise RuntimeError("frozen Runner EXE changed")
            started = self.runner(["service", "start"])["data"]
            self.instance = started["instance_id"]
            admitted = self.runner(["submit"], request, self.request_id, check=False)
            if admitted.get("ok"):
                self.run_id = admitted["data"]["run_id"]
            elif admitted.get("error", {}).get("code") == "request_outcome_unknown":
                self.run_id = self.reconcile_admission(request, admitted)
            else:
                raise RuntimeError(admitted)
            self.result["run_id"] = self.run_id
            until = time.monotonic()+360
            while time.monotonic() < until:
                view = self.inspect()
                save(self.root/"latest-run.json", view)
                if any(op["status"] == "outcome_unknown" for op in view.get("operations", [])):
                    raise RuntimeError("unknown original operation; no resend or Enter")
                if view["step"] == "implement.report":
                    raise RuntimeError("first-task contract insufficient; extra report input is outside the controlled two-task pair")
                if view["step"].startswith("publish") and view["status"] in ("waiting_input", "blocked"):
                    raw = self.stored("active-run-record.json")
                    inputs = raw["workflow"]["StageInputs"]
                    pubs = [p for p in inputs if p["step_id"] == "publish.execute"]
                    if len(pubs) != 1 or len(inputs) != 2:
                        raise RuntimeError("expected exactly implement.execute and publish.execute, no report input")
                    pub = pubs[0]
                    text = native_text(pub["prompt"])
                    self.result.update(reached_publication=True, input_scalars=len(text), input_bytes=len(text.encode()), input_sha256=digest(text.encode()))
                    (self.root/"publication.native.txt").write_bytes(text.encode())
                    (self.root/"publication.logical.txt").write_bytes(pub["prompt"].encode())
                    save(self.root/"complete-internal-results.json", raw["workflow"]["results"])
                    if self.group == "short" and (len(text) >= 1000 or "\nSource facts:\n" in text):
                        raise RuntimeError("short complete input not under 1000 or facts still visible")
                    if self.group == "long" and (not 3300 <= len(text) <= 3500 or "\nSource facts:\n" not in text):
                        raise RuntimeError("long input not in agreed baseline shape")
                    last = self.runner(["last-message"], extra=["--run-id", self.run_id])["data"]
                    save(self.root/"final-message.json", last)
                    if last.get("message", {}).get("text", "").strip() != final:
                        raise RuntimeError("exact structured final marker mismatch")
                    # Use the original complete owned transport state, not prompt previews.
                    for envelope in raw["snapshot"].get("Implementation", []):
                        if envelope.get("implementation_key") == "orca.terminal-state":
                            state = envelope["payload"]
                            turn = next(t for t in state["turns"] if t["command"] == pub["command_id"])
                            audit = dict(environment=request["execution"]["target"]["environment"], process=state["process"],
                                         pane=turn["native"]["pane"], baseline=turn["native"]["base_root_at"], input=text, final=final)
                            break
                    else:
                        raise RuntimeError("complete original transport identity missing")
                    save(self.root/"audit-request.json", audit)
                    proc = self.command([self.args.audit_exe, str(self.root/"audit-request.json")], timeout=60, label="independent-audit")
                    (self.root/"independent-original.json").write_bytes(proc.stdout)
                    if proc.returncode or not json.loads(proc.stdout).get("passed"):
                        raise RuntimeError("independent full original/process/final audit failed")
                    self.result.update(passed=True, public_session_verified=True)
                    break
                if view["status"] in ("blocked", "stopped", "waiting_input"):
                    raise RuntimeError(f"pre-publication boundary {view['step']} {view['status']}: {view.get('diagnostic')}")
                time.sleep(2)
            else:
                raise TimeoutError("bounded formal probe expired; no replay")
        except Exception as exc:
            self.result["error"] = str(exc)
            if not self.run_id and self.instance:
                try:
                    self.run_id = self.runner(["inspect"], extra=["--request-id", self.request_id])["data"]["run"]["run_id"]
                    self.result["run_id"] = self.run_id
                except Exception as lookup:
                    self.result["lookup_error"] = str(lookup)
        finally:
            self.cleanup()
            self.result["finished_at"] = now()
            self.result["passed"] &= all(self.result["cleanup"].get(k) for k in ("run_stopped", "service_stopped", "resources_settled", "native_zero", "worktree_removed"))
            save(self.root/"result.json", self.result)
        return self.result

    def cleanup(self):
        try:
            if self.run_id:
                before = self.stored("before-stop-run-record.json")
                for stage in before["workflow"].get("StageInputs") or []:
                    (self.root/(stage["step_id"]+".native.txt")).write_bytes(native_text(stage["prompt"]).encode())
                    if stage["step_id"] == "publish.execute":
                        text = native_text(stage["prompt"])
                        self.result.update(reached_publication=True, input_scalars=len(text), input_bytes=len(text.encode()), input_sha256=digest(text.encode()))
                if self.process and self.process.get("terminal"):
                    try:
                        shown = self.orca(["terminal", "show", "--terminal", self.process["terminal"]], "before-stop-terminal-show")
                        if (not shown.get("ok") or shown["result"]["terminal"]["incarnationId"] != self.process["incarnation"]):
                            raise RuntimeError("owned terminal identity not confirmed for read")
                        self.orca(["terminal", "read", "--terminal", self.process["terminal"], "--limit", "1000"], "before-stop-terminal-read")
                    except Exception as read_error:
                        self.result["cleanup_observation_error"] = str(read_error)
                for _ in range(5):
                    view = self.inspect()
                    reply = self.runner(["stop"], dict(expected_revision=view["revision"]), extra=["--run-id", self.run_id], check=False)
                    if reply.get("ok"):
                        break
                    if reply.get("error", {}).get("code") != "stale_revision":
                        raise RuntimeError("stop unconfirmed; do not replay an unknown stop")
                else:
                    raise RuntimeError("stop revisions unstable")
                until = time.monotonic()+60
                while time.monotonic() < until:
                    view = self.inspect()
                    if view.get("stop_status") == "stopped" and not view["new_inputs_allowed"]:
                        self.result["cleanup"]["run_stopped"] = True
                        break
                    time.sleep(1)
                save(self.root/"closed-run.json", view)
            if self.instance:
                self.runner(["service", "stop"], dict(expected_instance_id=self.instance))
                until = time.monotonic()+30
                while time.monotonic() < until:
                    if self.runner(["service", "status"])["data"]["status"] == "not_running":
                        self.result["cleanup"]["service_stopped"] = True
                        break
                    time.sleep(1)
                proof = self.runner(["service", "verify-stopped"])["data"]
                save(self.root/"verify-stopped.json", proof)
                self.result["cleanup"]["resources_settled"] = proof["resources_settled"]
            if self.run_id:
                raw = self.stored("closed-run-record.json")
                save(self.root/"complete-internal-results.json", raw["workflow"].get("results") or [])
            if self.worktree:
                owned_tail = self.worktree.split("::", 1)[-1].replace("\\", "/").rsplit("/", 1)[-1]
                if not self.worktree.startswith("59f29cc4-7282-47f4-9929-e8fe97fe1628::") or self.run_id not in owned_tail or not owned_tail.startswith(f"sr-pub3-{self.ordinal:02d}-"):
                    raise RuntimeError("refuse cleanup outside this newly admitted exact worktree")
                census = self.orca(["terminal", "list", "--worktree", "id:"+self.worktree], "cleanup-census")
                if not census.get("ok") or not complete_zero(census["result"]):
                    raise RuntimeError("complete empty owned native census not verified")
                self.result["cleanup"]["native_zero"] = True
                removed = self.orca(["worktree", "rm", "--worktree", "id:"+self.worktree, "--force"], "worktree-remove")
                if not removed.get("ok"):
                    raise RuntimeError("owned worktree removal unconfirmed; do not repeat")
                for read in range(1, 61):
                    after = self.orca(["worktree", "show", "--worktree", "id:"+self.worktree], f"worktree-after-{read:02d}")
                    if not after.get("ok") and after.get("error", {}).get("code") == "selector_not_found":
                        break
                    if not after.get("ok"):
                        raise RuntimeError("owned removal reconciliation unavailable; no repeat rm")
                    time.sleep(1)
                else:
                    raise RuntimeError("owned worktree still present after bounded read-only removal reconciliation")
                self.result["cleanup"]["worktree_removed"] = True
            elif not self.run_id:
                self.result["cleanup"].update(run_stopped=True, native_zero=True, worktree_removed=True, no_run_admitted=True)
            elif (not raw.get("native_started") and not raw.get("workspace_fact")
                  and not raw["snapshot"].get("Implementation")
                  and "no Orca Git repository has directory name" in self.result.get("error", "")):
                # The production resolver rejects before worktree creation.
                # Preserve that failure and independently check the exact repo.
                inventory = self.orca(["worktree", "list", "--repo", "id:59f29cc4-7282-47f4-9929-e8fe97fe1628"], "no-worktree-created-inventory")
                if not inventory.get("ok") or "worktrees" not in inventory["result"] or any(self.run_id in json.dumps(w) for w in inventory["result"]["worktrees"]):
                    raise RuntimeError("pre-creation failure workspace absence not verified")
                self.result["cleanup"].update(native_zero=True, no_workspace_created=True, worktree_removed=False)
            else:
                raise RuntimeError("new admitted worktree identity unavailable; stop for owned read-only reconciliation")
        except Exception as exc:
            self.result["cleanup_error"] = str(exc)


def main():
    ap = argparse.ArgumentParser(description=__doc__)
    ap.add_argument("--resume", action="store_true", help="Continue unattempted ordinals after owned cleanup; never replay a case")
    for name in ("runner", "audit-exe", "baseline", "repository", "first", "publish", "out", "state-root", "runtime", "orca", "pwsh"):
        ap.add_argument("--"+name, required=True)
    args = ap.parse_args()
    for name in ("runner", "audit_exe", "baseline", "repository", "first", "publish", "out", "state_root", "orca", "pwsh"):
        setattr(args, name, str(Path(getattr(args, name)).resolve()))
    args.orca_wrapper = str(Path(__file__).with_name("invoke-owned-orca.ps1").resolve())
    root, states = Path(args.out), Path(args.state_root)
    args.runner_sha = digest(Path(args.runner).read_bytes())
    baseline = load(args.baseline)
    if args.resume:
        summary = load(root/"summary.json")
        if summary["status"] != "stopped" or summary["active_case"] or not 1 <= len(summary["rows"]) < 6 or summary["runner_sha256"] != args.runner_sha:
            raise RuntimeError("only settled attempted cases may resume to unattempted ordinals")
        index = 1
        while (root/f"interruption-{index:02d}-summary.json").exists():
            index += 1
        save(root/f"interruption-{index:02d}-summary.json", summary)
        for i, row in enumerate(summary["rows"]):
            repair = root/row["root"]/"manual-cleanup-result.json"
            if repair.exists():
                repaired = load(repair)
                if repaired["passed"] != row["passed"] or repaired["run_id"] != row["run_id"]:
                    raise RuntimeError("read-only cleanup reconciliation changed original outcome/identity")
                summary["rows"][i] = row = repaired
            if row["ordinal"] != i+1 or not all(row["cleanup"].get(k) for k in ("run_stopped", "service_stopped", "resources_settled", "native_zero")) or not (row["cleanup"].get("worktree_removed") or row["cleanup"].get("no_workspace_created")):
                raise RuntimeError("original attempted case not fully settled")
        summary.pop("error", None)
        summary.update(status="running", resumed_at=now(), resume_reason="All original failed cases retained; cleanup-only driver corrections. Continue unattempted ordinals; no input/creation replay or Runner EXE change.")
    else:
        root.mkdir(exist_ok=False)
        if states.exists():
            raise RuntimeError("state root must be new")
        summary = dict(schema="formal-publication-length-comparison/v1", started_at=now(), status="running",
                       source_build=load(args.runner+".build.json"), runner_sha256=args.runner_sha,
                       audit_exe_sha256=digest(Path(args.audit_exe).read_bytes()), state_root=str(states),
                       planned_per_group=3, rows=[], active_case=None,
                       scope="actual Runner service/submit development/2 implement.execute to publish.execute; no business tests or real publication")
    save(root/"summary.json", summary)
    try:
        for ordinal, (group, pair) in enumerate([("long", 1), ("short", 1), ("short", 2), ("long", 2), ("long", 3), ("short", 3)], 1):
            if ordinal <= len(summary["rows"]):
                continue
            name = f"{ordinal:02d}-{group}-{pair}"
            summary["active_case"] = name
            save(root/"summary.json", summary)
            case = Case(args, baseline, root/name, states/f"{ordinal:02d}", ordinal, group, pair)
            result = case.execute()
            summary["rows"].append(result)
            summary["active_case"] = None
            save(root/"summary.json", summary)
            print(json.dumps(result, ensure_ascii=False), flush=True)
            if result.get("cleanup_error") or not all(result["cleanup"].get(k) for k in ("run_stopped", "service_stopped", "resources_settled", "native_zero")) or not (result["cleanup"].get("worktree_removed") or result["cleanup"].get("no_workspace_created")):
                raise RuntimeError("owned cleanup incomplete; stop before next sample")
        summary["status"] = "completed"
    except Exception as exc:
        summary.update(status="stopped", error=str(exc))
        raise
    finally:
        summary["finished_at"] = now()
        save(root/"summary.json", summary)
    if not all(row["passed"] for row in summary["rows"]):
        raise SystemExit("Formal schedule finished with failed samples; see preserved per-case outcomes")


if __name__ == "__main__":
    main()
