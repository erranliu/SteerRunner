"""Read Runner's stopped acceptance database; never open an Orca database."""
from pathlib import Path
import base64
import hashlib
import json
import sqlite3

evidence = Path(__file__).resolve().parent
root = evidence.parents[3]
state_dir = root / ".evidence/orca-structured-agent-session-mvp-verified-state"
connection = sqlite3.connect((state_dir / "runner.db").as_uri() + "?mode=ro", uri=True)
checks = []
for run_id, raw in connection.execute("select id,body from runs order by id"):
    record = json.loads(raw)
    mode = {"3d5f07fc32e344cc444bea61b0ff7f46":"normal", "6b937ec8a51e471bfd8942c702b56288":"active", "8e21a8ab7ac9ef87f296a6f892610ba7":"large"}[run_id]
    prefix = f"verified-{mode}-native" + ("-after-stop" if mode == "active" else "")
    native = json.loads((evidence / f"{prefix}-summary.json").read_text(encoding="utf-8-sig"))
    items = {}
    for page in evidence.glob(f"{prefix}-history-page-*.json"):
        for item in json.loads(page.read_text(encoding="utf-8-sig"))["result"]["page"]["items"]:
            items[item["itemId"]] = item
    items = sorted(items.values(), key=lambda item: (item["sequence"], item.get("sequenceIndex", 0)))
    users = [item for item in items if item["body"].get("kind") == "message" and item["body"].get("role") == "user"]
    assert len(users) == 1
    stage = record["workflow"]["StageInputs"][0]
    text = stage["prompt"]
    blocks = users[0]["body"]["blocks"]
    assert len(blocks) == 1 and blocks[0]["type"] == "text" and blocks[0]["text"] == text
    assert record["request"]["workflow"]["prompt_overrides"]["execute"] in text
    opaque = next(envelope["payload"] for envelope in record["snapshot"]["Implementation"] if envelope["implementation_key"] == "orca.agent-session-state")
    if isinstance(opaque, str):
        opaque = json.loads(opaque)
    assert opaque["input_digest"] == hashlib.sha256(text.encode()).hexdigest()
    assert len(json.dumps(opaque).encode()) < 65536 and "prompt" not in opaque and "reply" not in opaque
    submission = next(item for item in native["submissions"] if item["clientMessageId"] == opaque["message_id"])
    body_fingerprint = json.dumps({"method": "agentSession.send", "sessionId": opaque["session_id"], "fields": {"body": users[0]["body"]}}, ensure_ascii=False, sort_keys=True, separators=(",", ":"))
    assert hashlib.sha256(body_fingerprint.encode()).hexdigest() == opaque["body_fingerprint"] == submission["payloadFingerprint"]
    assert users[0]["itemId"] == "orca:" + opaque["message_id"]
    assert submission["providerItemId"] == opaque["provider_item_id"] != users[0]["itemId"]
    actions = [intent["request"]["Operation"]["action"] for intent in record["intents"]]
    assert actions.count("start_turn") == 1 and actions.count("stop_execution") == 1
    assert record.get("launch_command", "") == "" and record.get("launch_input_budget") is None
    stop = next(intent for intent in record["intents"] if intent["request"]["Operation"]["action"] == "stop_execution")
    assert opaque["close_attempted"] and not stop["receipt"].get("state", {}).get("stopped", False)
    saved_stop = json.loads(base64.b64decode(stop["request"]["Operation"]["receipt"]))
    close_ack = json.loads(base64.b64decode(saved_stop["operation"]["receipt"]))
    assert close_ack["ok"] is True
    actual_result = "\n\n".join("".join(block["text"] for block in item["body"]["blocks"]) for item in items if item["body"].get("kind") == "message" and item["body"].get("role") == "assistant")
    content = record.get("content_result")
    if mode in ("normal", "large"):
        assert content["status"] == "succeeded" and content["text"] == actual_result
        prefix,expected = ("RUNNER_FINAL_LINE_",300) if mode=="normal" else ("RUNNER_LARGE_LINE_",600)
        assert sum(line.startswith(prefix) for line in actual_result.splitlines()) == expected
        if mode=="large": assert len(actual_result)>8000
    assert record["stop_status"] == "unknown" and record["new_inputs_allowed"] is False
    if mode!="large":
        after_restart = json.loads((evidence / f"verified-{mode}-native-after-restart-summary.json").read_text(encoding="utf-8-sig"))
        assert after_restart["sessionId"] == opaque["session_id"] and after_restart["userCount"] == 1
        assert after_restart["latestTurn"]["turn"]["turnId"] == opaque["turn_id"]
    checks.append({"run_id": run_id, "mode": mode, "session_id": opaque["session_id"], "command_id": opaque["command_id"], "message_id": opaque["message_id"], "native_turn_id": opaque["turn_id"], "user_items": len(users), "frozen_body_bytes": len(text.encode()), "frozen_body_sha256": opaque["input_digest"], "native_body_equal_frozen": True, "input_budget": stage["input_budget"], "provider_item_id": submission["providerItemId"], "normalized_user_item_id": users[0]["itemId"], "durable_actions": actions, "native_close_ack": close_ack, "close_attempted": True, "auxiliary_shells": len(opaque.get("shells", [])), "opaque_state_bytes": len(json.dumps(opaque).encode()), "complete_result_equal_native_assistant_messages": mode in ("normal", "large"), "result_utf8_bytes": len(actual_result.encode()), "content_status": content.get("status") if content else None, "stop_status": "unknown", "public_native_outcome": native["latestTurn"]["turn"].get("outcome"), "runtime_launch_command_empty": True, "same_original_session_and_one_user_after_restart": True if mode!="large" else None})
connection.close()

legacy = []
for name in ["orca-structured-agent-session-p0-20261103", "orca-structured-agent-session-blockers-20261103"]:
    directory = evidence.parent / name
    manifest = json.loads((directory / "manifest.json").read_text(encoding="utf-8-sig"))
    for file in manifest["files"]:
        data = (directory / file["path"]).read_bytes()
        assert hashlib.sha256(data).hexdigest() == file["sha256"] and len(data) == file["bytes"]
    legacy.append({"directory": name, "verified_files": len(manifest["files"]), "manifest_unchanged": True, "source_map_note": "Historical source hashes describe the earlier investigation baseline, not the new implementation."})
summary = {"schema": "orca-structured-mvp-verification/v1", "executable_sha256": hashlib.sha256((root / ".tmp-mvp/steerrunner-final-rpc.exe").read_bytes()).hexdigest(), "state_directory": str(state_dir), "runs": checks, "previous_evidence": legacy, "qualification": "Local Codex execute/1 input and complete public-turn assistant text verified; complete resource settlement remains unverified. Durable intent counts are not independent packet capture."}
(evidence / "verified-persistence-checks.json").write_text(json.dumps(summary, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
print(json.dumps({"runs": [(check["mode"], check["user_items"], check["content_status"], check["stop_status"]) for check in checks], "legacy": legacy}, ensure_ascii=False))
