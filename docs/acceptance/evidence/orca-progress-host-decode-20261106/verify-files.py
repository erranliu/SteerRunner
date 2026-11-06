import base64
import hashlib
import json
import sqlite3
from pathlib import Path

root = Path(__file__).resolve().parent
records = []
for side in ("local", "remote"):
    [path] = root.glob(side + "-development-*/passed.json")
    passed = json.loads(path.read_bytes())
    active = json.loads((path.parent / "active-file-proof.json").read_bytes())
    state = Path(passed["state"])
    with sqlite3.connect((state / "runner.db").as_uri() + "?mode=ro", uri=True) as db:
        run = json.loads(db.execute("SELECT body FROM runs WHERE id=?", (passed["run"],)).fetchone()[0])
    assert active["run"] == run["id"] and active["result_status"] == "pending"
    assert active["turn"] == passed["turns"][0]["turn_id"]
    assert run["stop_status"] == "stopped" and not run["new_inputs_allowed"] and not run["resume_required"]
    assert len(run["workflow"]["StageInputs"]) == len(run["workflow"]["StageResults"]) == 1
    [final] = run["workflow"]["StageResults"]
    assert final["FinalVerified"] and final["TurnID"] == active["turn"]
    for channel, prefix, contract in (
        ("progress", "FILE_PROGRESS_", "steerrunner-progress-file/v1"),
        ("result_file", "FILE_RESULT_", "steerrunner-result-file/v1"),
    ):
        item = run[channel]
        view = item["view"]
        want = (prefix + side.upper()).encode()
        assert view["run_id"] == run["id"] and view["contract"] == contract and view["collection_status"] == "closed"
        assert base64.b64decode(view["content_base64"], validate=True) == want
        assert view["sha256"] == hashlib.sha256(want).hexdigest()
        assert item["workspace_instance"] and item["binding"] and item["workspace"]
        if channel == "progress":
            assert view["sha256"] == active["progress_sha256"] and view["version"] >= active["progress_version"]
        else:
            assert item["sealed"] and view["version"] == 1 and view["sha256"] == passed["result_sha256"]
    assert run["progress"]["workspace_instance"] == run["result_file"]["workspace_instance"]
    assert run["progress"]["host_id"] == run["result_file"]["host_id"]
    assert run["progress"]["workspace"] == run["result_file"]["workspace"]
    record = {
        "side": side, "run": run["id"], "turn": active["turn"],
        "host": run["progress"]["host_id"], "instance": run["progress"]["workspace_instance"],
        "progress_sha256": active["progress_sha256"], "result_sha256": passed["result_sha256"],
    }
    records.append(record)
    print(json.dumps(record))
(root / "file-proofs.json").write_text(json.dumps(records, indent=2), encoding="utf-8")
