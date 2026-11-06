import base64
import hashlib
import json
import sqlite3
from pathlib import Path

ROOT = Path(__file__).resolve().parent


def digest(value):
    return hashlib.sha256(value).hexdigest()


for side in ("local", "remote"):
    [record_file] = ROOT.glob(f"{side}-execute-*/passed.json")
    record = json.loads(record_file.read_bytes())
    state = Path(record["state"])
    [owner] = (state / "orca-creations").iterdir()
    raw_plan = (owner / "orca-create-owner.json").read_bytes()
    raw_started = (owner / "create-started.json").read_bytes()
    plan = json.loads(raw_plan)
    started = json.loads(raw_started)
    output = json.loads((owner / "create-result.json").read_bytes())
    response_bytes = base64.b64decode(output["Output"], validate=True)
    response = json.loads(response_bytes)
    assert started["PlanDigest"] == output["PlanDigest"] == digest(raw_plan)
    assert output["StartedSHA"] == digest(raw_started) and output["OutputSHA"] == digest(response_bytes)
    assert not output["Failed"] and response["ok"]
    assert plan["Arguments"][:2] == ["worktree", "create"] and plan["Key"]["Scope"] == "workspace"
    assert plan["ImageSHA256"] == digest((ROOT / "writer.exe").read_bytes())
    worktree = response["result"]["worktree"]
    assert plan["Runtime"] == response["_meta"]["runtimeId"] == record["turns"][0]["runtime"]
    assert worktree["id"] == record["turns"][0]["worktree"] and worktree["instanceId"]
    if side == "local":
        assert plan["Selector"] == "path:" + plan["Repository"]
    else:
        assert worktree["repoId"] and plan["Selector"] == "id:" + worktree["repoId"]

    with sqlite3.connect((state / "runner.db").as_uri() + "?mode=ro", uri=True) as db:
        row = json.loads(db.execute("SELECT body FROM runs WHERE id=?", (record["run"],)).fetchone()[0])
    [intent] = [item for item in row["intents"] if item["request"]["Operation"]["action"] == "prepare_workspace"]
    req, receipt = intent["request"], intent["receipt"]
    operation = req["Operation"]
    assert plan["Key"]["Execution"] == row["id"] == record["run"]
    assert plan["Key"]["Configuration"] == row["input_digest"]
    assert plan["Key"]["Operation"] == operation["id"] and plan["Key"]["Input"] == operation["payload_digest"]
    assert receipt["disposition"] == "confirmed"
    checkpoint = json.loads(base64.b64decode(receipt["operation"]["receipt"], validate=True))
    assert checkpoint["runtime_id"] == checkpoint["resolved_runtime_id"] == plan["Runtime"]
    if side == "remote":
        assert checkpoint["resolved_repository_id"] == worktree["repoId"]
    for key in ("id", "instanceId", "path", "head", "baseRef"):
        assert checkpoint["worktree"][key] == worktree[key]
    assert worktree["head"] == worktree["baseRef"] == req["Input"]["workspace"]["base_commit"]
    agent = next(item["payload"] for item in receipt["implementation"] if item["implementation_key"] == "orca.agent-session-state")
    assert agent["worktree"]["id"] == worktree["id"] and agent["worktree"]["instanceId"] == worktree["instanceId"]
    assert agent["session_id"] == record["turns"][0]["session_id"]
    workspace, target = receipt["workspace"], receipt["target"]
    snapshot = req["Snapshot"]
    expected_branch = worktree["branch"].removeprefix("refs/heads/")
    assert workspace["inspection_scope"] == "revision"
    assert workspace["repository"] == snapshot["Session"]["ProjectID"]
    assert workspace["worktree"] == worktree["path"] and workspace["head"] == worktree["head"]
    assert workspace["base"] == req["Input"]["workspace"]["base_commit"] and workspace["branch"] == expected_branch
    assert not workspace.get("tree") and not workspace.get("qualified") and not workspace["dirty"] and workspace["checked_at"]
    expected_target = {**snapshot["Ref"]["target"], "worktree_id": worktree["id"], "branch": expected_branch, "base_revision": workspace["base"]}
    assert target == expected_target, (target, expected_target)
    assert receipt["process"]["kind"] == "orca_agent_session"
    assert receipt["state"]["runtime_id"] == plan["Runtime"] and receipt["state"]["worktree_id"] == worktree["id"]
    assert not receipt["state"].get("session_id") and not receipt["state"].get("worker_live")
    evidence = {
        "run": row["id"], "input_digest": row["input_digest"], "operation": operation["id"],
        "selector_kind": plan["Selector"].split(":", 1)[0], "runtime": plan["Runtime"],
        "worktree": worktree["id"], "instance": worktree["instanceId"], "base": worktree["baseRef"],
        "session": agent["session_id"], "plan_sha256": output["PlanDigest"],
        "dispatch_sha256": output["StartedSHA"], "result_sha256": output["OutputSHA"],
        "workspace": workspace, "target": target, "process": receipt["process"], "state": receipt["state"],
    }
    (record_file.parent / "creation-proof.json").write_text(json.dumps(evidence, indent=2), encoding="utf-8")
    print("verified", side, row["id"], operation["id"], evidence["selector_kind"])
