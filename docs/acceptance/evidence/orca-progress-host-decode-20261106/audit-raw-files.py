import base64
import json
from pathlib import Path

root = Path(__file__).resolve().parent
for side in ("local", "remote"):
    [passed_file] = root.glob(f"{side}-development-*/passed.json")
    passed = json.loads(passed_file.read_bytes())
    entries = []
    for file in sorted(passed_file.parent.glob("*.json")):
        if file.name[:3].isdigit():
            response = json.loads(file.read_bytes())
            entries.append((int(file.name[:3]), file.name[4:], response))
    progress_text = base64.b64encode(("FILE_PROGRESS_" + side.upper()).encode()).decode()
    [progress] = [item for item in entries if item[1].startswith("progress-file-") and item[2]["data"].get("content_base64") == progress_text]
    before = [item for item in entries if item[0] < progress[0] and item[1].startswith("inspect-")][-1][2]["data"]["run"]
    pending = next(item for item in entries if item[0] > progress[0] and item[1].startswith("result-file-"))
    after = next(item for item in entries if item[0] > pending[0] and item[1].startswith("inspect-"))[2]["data"]["run"]
    assert pending[2]["data"]["collection_status"] == "pending" and not pending[2]["data"].get("content_base64")
    assert before["run_id"] == after["run_id"] == passed["run"]
    assert before["status"] == after["status"] == "running"
    assert before["execution"]["turn_id"] == after["execution"]["turn_id"] == passed["turns"][0]["turn_id"]
    assert not before.get("stop_status") and not after.get("stop_status")
    closed = [item for item in entries if item[1].startswith("result-file-") and item[2]["data"]["collection_status"] == "closed"]
    assert len(closed) >= 2
    stopped = [item for item in entries if item[1].startswith("inspect-") and item[2]["data"]["run"].get("stop_status") == "stopped"]
    assert stopped and stopped[0][0] < closed[0][0]
    acknowledgements = [item for item in entries if item[1].startswith("stop-") and item[2]["ok"]]
    assert len(acknowledgements) == 1 and pending[0] < acknowledgements[0][0] < stopped[0][0]
    first, last = closed[0][2]["data"], closed[-1][2]["data"]
    for key in ("run_id", "contract", "version", "observed_at", "sha256", "content_base64"):
        assert first[key] == last[key]
    assert first["run_id"] == passed["run"] and first["version"] == 1
    assert first["contract"] == "steerrunner-result-file/v1"
    assert base64.b64decode(first["content_base64"]) == ("FILE_RESULT_" + side.upper()).encode()
    print(side, passed["run"], "running progress -> pending result -> explicit stop -> sealed result", "verified")

for directory in sorted(root.glob("*-execute-*")):
    responses = [json.loads(file.read_bytes()) for file in directory.glob("*-submit*.json")]
    assert len(responses) == 1 and not responses[0]["ok"] and responses[0]["error"]["code"] == "invalid_input"
    proof = [json.loads(file.read_bytes()) for file in directory.glob("*-service-verify-stopped.json")]
    assert len(proof) == 1 and proof[0]["ok"] and proof[0]["data"]["resources_settled"] and proof[0]["data"]["runs"] == []
    print(directory.name, "submission rejected before Run admission", "verified")
