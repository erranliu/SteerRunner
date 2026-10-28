"""Fresh development/2 probe through the shipped Runner CLI, never test EXE input.

The second reply is a unique structured needs_input marker so the unmodified
workflow settles without issuing a third report prompt or performing publication.
This probe does not qualify business publication or SteerLine integration.
"""
import argparse
import datetime as dt
import hashlib
import json
import pathlib
import subprocess
import time
import uuid


def now():
    return dt.datetime.now(dt.timezone.utc).isoformat()


def dump(path, value):
    path.write_text(json.dumps(value, ensure_ascii=False, indent=2), encoding="utf-8")


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--runner", required=True)
    ap.add_argument("--profile", required=True, help="frozen contracts.Execution JSON")
    ap.add_argument("--repository", required=True)
    ap.add_argument("--revision", required=True)
    ap.add_argument("--out", required=True)
    ap.add_argument("--audit-exe", required=True)
    args = ap.parse_args()
    root = pathlib.Path(args.out).resolve()
    root.mkdir(parents=True, exist_ok=False)
    runner = str(pathlib.Path(args.runner).resolve())
    execution = json.loads(pathlib.Path(args.profile).read_text(encoding="utf-8-sig"))
    if execution["adapter"] != "orca-terminal" or execution["target"]["kind"] != "orca_environment":
        raise ValueError("explicit original remote Orca execution required")
    state = root / "state"
    marker = "SR_CLI_FOLLOWUP_" + uuid.uuid4().hex
    final = json.dumps({"outcome": "needs_input", "summary": marker, "question": marker}, separators=(",", ":"))
    first = ("仅执行隔离输入诊断，不运行任何业务测试，不改文件、不提交、不推送、不创建PR。\n"
             "用一次 shell 工具执行 Write-Output 'SR_CLI_TOOL_OUTPUT'，确认输出并正常结束。\n"
             "完成后仅返回以下 ASCII JSON，不输出代码块或额外自然语言：\n"
             '{"outcome":"done","summary":"SR_CLI_TOOL_OUTPUT"}')
    second = "后续发布输入的最小诊断：禁止真实发布、工具调用、文件修改和PR创建。只返回以下唯一结构化标记（不要代码块或说明）：\n" + final + "\n\n"
    second += ("只读中文占位材料，保留完整输入与空行。\n\n" * 200)[:3427-len(second)]
    request_id = "followup-" + uuid.uuid4().hex
    request = {"title": "Runner isolated followup input diagnostic", "name_prefix": "sr-followup-cli",
               "repository": {"url": "https://github.com/erranliu/SteerlineTest.git", "local_path": str(pathlib.Path(args.repository).resolve()), "remote": "origin", "base_branch": "main", "base_revision": args.revision},
               "documents": {"spec": {"path": "README.md"}, "plan": {"path": "SteerlineFlow/README.md"}},
               "workflow": {"id": "development", "version": 2, "prompt_overrides": {"implement.execute": first, "publish": second}},
               "execution": execution,
               "publication": {"provider": "command", "repository": "erranliu/SteerlineTest", "base_branch": "main", "settings": {"base_url": "https://github.com", "query": {"executable": "gh", "args": ["api"]}}}}
    dump(root / "frozen-input.json", request)
    result = {"passed": False, "started_at": now(), "runner": runner, "runner_sha256": hashlib.sha256(pathlib.Path(runner).read_bytes()).hexdigest(), "request_id": request_id, "marker": final, "cleanup": {}}
    count = 0
    instance = run_id = None

    def call(command, body=None, request=None, extra=(), offline=False, check=True):
        nonlocal count
        count += 1
        argv = [runner, *command, "--state-dir", str(state), "--json", *extra]
        if instance and command[:1] != ["service"]:
            argv += ["--expected-instance-id", instance]
        if body is not None:
            argv += ["--input", "-"]
            body = {"protocol": "steerrunner-cli/v1", "request_id": request or uuid.uuid4().hex, "input": body}
        if offline:
            argv += ["--offline"]
        started = now()
        proc = subprocess.run(argv, input=None if body is None else json.dumps(body, ensure_ascii=False).encode(), capture_output=True, timeout=90, creationflags=subprocess.CREATE_NO_WINDOW)
        dump(root / f"call-{count:03d}.json", {"started_at": started, "finished_at": now(), "argv": argv, "stdin": body, "exit_code": proc.returncode, "stdout": proc.stdout.decode("utf-8", errors="replace"), "stderr": proc.stderr.decode("utf-8", errors="replace")})
        value = json.loads(proc.stdout.decode("utf-8-sig"))
        if check and (proc.returncode or not value.get("ok")):
            raise RuntimeError(value)
        return value

    def inspect():
        return call(["inspect"], extra=["--run-id", run_id])["data"]["run"]

    try:
        started = call(["service", "start"])["data"]
        instance = started["instance_id"]
        # One submission only. Ambiguity is resolved by read-only original ID.
        accepted = call(["submit"], request, request_id)["data"]
        run_id = accepted["run_id"]
        result["run_id"] = run_id
        until = time.monotonic() + 240
        while time.monotonic() < until:
            view = inspect()
            dump(root / "latest-run.json", view)
            # Publication has no business-input continuation state: its explicit
            # needs_input result settles as blocked, unlike implementation input.
            if view["step"].startswith("publish") and view["status"] in ("waiting_input", "blocked"):
                msg = call(["last-message"], extra=["--run-id", run_id])["data"]
                dump(root / "final-message.json", msg)
                if msg.get("message", {}).get("text", "").strip() != final:
                    raise RuntimeError("unique structured final marker mismatch")
                intents = []
                for path in state.glob("native-input/*/*/input.result.json"):
                    row = json.loads(path.read_text(encoding="utf-8"))
                    if row["input"]["phase"] == "prepare" and row["receipt"].get("disposition") == "confirmed":
                        for envelope in row["receipt"].get("implementation", []):
                            if envelope.get("implementation_key") == "orca.terminal-state":
                                saved = envelope["payload"]
                                for turn in saved["turns"]:
                                    if turn["command"] == row["input"]["command_id"]:
                                        intents.append((saved, turn))
                # Exactly two distinct production inputs; no supplemental report.
                unique = {turn["command"]: (saved, turn) for saved, turn in intents}
                if len(unique) != 2:
                    raise RuntimeError(f"expected two prepared production inputs, got {len(unique)}")
                saved, turn = next((s, t) for s, t in unique.values() if t.get("step_id") == "publish.execute")
                audit = {"environment": execution["target"]["environment"], "process": saved["process"], "pane": turn["native"]["pane"], "baseline": turn["native"]["base_root_at"], "input": turn["prompt"].replace("\r\n", "\n").replace("\r", "\n").rstrip("\n"), "final": final}
                dump(root / "audit-request.json", audit)
                proc = subprocess.run([str(pathlib.Path(args.audit_exe).resolve()), str(root / "audit-request.json")], capture_output=True, timeout=60, creationflags=subprocess.CREATE_NO_WINDOW)
                (root / "independent-original.json").write_bytes(proc.stdout)
                (root / "independent-original-stderr.txt").write_bytes(proc.stderr)
                if proc.returncode or not json.loads(proc.stdout).get("passed"):
                    raise RuntimeError("independent full original input/final audit failed")
                result["public_session_verified"] = True
                result["passed"] = True
                break
            if view["status"] in ("blocked", "stopped") or (view["status"] == "waiting_input" and not view["step"].startswith("publish")):
                raise RuntimeError(f"unexpected boundary {view['step']} {view['status']}: {view.get('diagnostic')}")
            if any(op["status"] == "outcome_unknown" for op in view.get("operations", [])):
                raise RuntimeError("unknown original operation; no replay or Enter")
            time.sleep(1)
        else:
            raise TimeoutError("bounded formal two-task probe expired; no replay")
    except Exception as exc:
        result["error"] = str(exc)
        if not run_id and instance:
            try:
                run_id = call(["inspect"], extra=["--request-id", request_id])["data"]["run"]["run_id"]
                result["run_id"] = run_id
            except Exception as lookup_error:
                result["lookup_error"] = str(lookup_error)
    finally:
        try:
            if run_id:
                for attempt in range(5):
                    view = inspect()
                    stopped = call(["stop"], {"expected_revision": view["revision"]}, extra=["--run-id", run_id], check=False)
                    if stopped.get("ok"):
                        break
                    if stopped.get("error", {}).get("code") != "stale_revision":
                        raise RuntimeError("stop result unknown/rejected; never replay: " + str(stopped))
                else:
                    raise RuntimeError("stop revisions remained unstable")
                until = time.monotonic()+45
                while time.monotonic() < until:
                    view = inspect()
                    if view.get("stop_status") == "stopped" and not view["new_inputs_allowed"]:
                        result["cleanup"]["run_stopped"] = True
                        break
                    time.sleep(1)
                dump(root / "closed-run.json", view)
            if instance:
                call(["service", "stop"], {"expected_instance_id": instance})
                until = time.monotonic()+30
                while time.monotonic() < until:
                    if call(["service", "status"])["data"]["status"] == "not_running":
                        result["cleanup"]["service_stopped"] = True
                        break
                    time.sleep(1)
                proof = call(["service", "verify-stopped"])["data"]
                dump(root / "verify-stopped.json", proof)
                result["cleanup"]["resources_settled"] = proof["resources_settled"]
        except Exception as cleanup_error:
            result["cleanup_error"] = str(cleanup_error)
        result["passed"] = result["passed"] and all(result["cleanup"].get(k) for k in ("run_stopped", "service_stopped", "resources_settled"))
        result["finished_at"] = now()
        dump(root / "result.json", result)
    if not result["passed"]:
        raise SystemExit(json.dumps(result, ensure_ascii=False))


if __name__ == "__main__":
    main()
