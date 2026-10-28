"""Require a real first-task/publication gate, then collect 3 valid samples per length.

Front failures are preserved and bounded, never counted as publication samples.
Any attempted publication input counts (including unknown/failure); never replay it.
"""
import argparse
import hashlib
import importlib.util
import json
from pathlib import Path
import sys

sys.dont_write_bytecode = True
spec = importlib.util.spec_from_file_location("formal_case", Path(__file__).with_name("test-formal-publication-length.py"))
formal = importlib.util.module_from_spec(spec)
spec.loader.exec_module(formal)


def classify(case, row):
    record = formal.load(case.root/"closed-run-record.json")
    stages = record["workflow"].get("StageInputs") or []
    canonical = record["workflow"].get("StageResults") or []
    first = next((p for p in stages if p["step_id"] == "implement.execute"), None)
    pubs = [p for p in stages if p["step_id"] == "publish.execute"]
    complete_first = first is not None and any(v["CommandID"] == first["command_id"] and v["FinalVerified"] and v["Succeeded"] for v in canonical)
    tool_output = False
    effects = {}
    for path in (case.state/"native-input").rglob("*.result.json"):
        if path.name == "input.result.json" or path.name.endswith(".call-result.json"):
            continue
        call = formal.load(path)
        args = call.get("args", [])
        if not args:
            continue
        identity = formal.load(path.parent/"input.intent.json")["input"]
        if args[:2] == ["terminal", "send"] and call.get("invoked"):
            kind = "paste" if "--text" in args else "enter" if "--enter" in args else "other"
            effects.setdefault(identity["command_id"], []).append(kind)
        if args[:2] == ["terminal", "read"] and call.get("stdout"):
            try:
                lines = json.loads(call["stdout"]).get("result", {}).get("terminal", {}).get("tail", [])
                tool_output |= any(line.strip() == "└ SR_INPUT_TOOL_OUTPUT" for line in lines)
            except ValueError:
                pass
    for values in effects.values():
        if values.count("paste") > 1 or values.count("enter") > 1:
            raise RuntimeError("original input effect repeated")
    valid = complete_first and tool_output and len(pubs) == 1 and effects.get(pubs[0]["command_id"], []).count("paste") == 1
    if first:
        text = formal.native_text(first["prompt"])
        if len(text) != 833 or hashlib.sha256(text.encode()).hexdigest() != "ddbdf515b767483e34347e17abf27d7b046856d2c6d2188f3517bfabf5ef53de":
            raise RuntimeError("original frozen 833-character first input changed")
    result = dict(row, first_task_canonical_verified=complete_first, first_real_tool_output_observed=tool_output,
                  valid_publication_sample=valid, effects=effects)
    formal.save(case.root/"effective-sample.json", result)
    return result


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    for name in ("runner", "audit-exe", "baseline", "repository", "first", "publish", "out", "state-root", "runtime", "orca", "pwsh", "phase"):
        parser.add_argument("--"+name, required=True)
    parser.add_argument("--gate-summary")
    args = parser.parse_args()
    if args.phase not in ("gate", "comparison"):
        raise RuntimeError("explicit gate or comparison required")
    for key in ("runner", "audit_exe", "baseline", "repository", "first", "publish", "out", "state_root", "orca", "pwsh"):
        setattr(args, key, str(Path(getattr(args, key)).resolve()))
    args.orca_wrapper = str(Path(__file__).with_name("invoke-owned-orca.ps1").resolve())
    args.runner_sha = formal.digest(Path(args.runner).read_bytes())
    if args.phase == "comparison":
        gate = formal.load(Path(args.gate_summary))
        if not gate.get("qualified") or gate["runner_sha256"] != args.runner_sha:
            raise RuntimeError("same exact Runner build must first pass real first-task/publication gate")
    root, states = Path(args.out), Path(args.state_root)
    root.mkdir(exist_ok=False)
    if states.exists():
        raise RuntimeError("fresh state root required")
    summary = dict(schema="valid-formal-publication-comparison/v1", phase=args.phase, status="running", started_at=formal.now(),
                   source_build=formal.load(args.runner+".build.json"), runner_sha256=args.runner_sha,
                   rows=[], valid_samples=[], front_failures=[], active_case=None, qualified=False)
    formal.save(root/"summary.json", summary)
    baseline = formal.load(Path(args.baseline))
    schedule = [("short", 0)] if args.phase == "gate" else [("long", 1), ("short", 1), ("short", 2), ("long", 2), ("long", 3), ("short", 3)]
    ordinal, consecutive_front = (0 if args.phase == "gate" else 1), 0
    try:
        for group, pair in schedule:
            while True:
                name = f"{ordinal:02d}-{group}-{pair}"
                summary["active_case"] = name
                formal.save(root/"summary.json", summary)
                case = formal.Case(args, baseline, root/name, states/f"{ordinal:02d}", ordinal, group, pair)
                row = case.execute()
                row = classify(case, row)
                summary["rows"].append(row)
                summary["active_case"] = None
                cleanup = row["cleanup"]
                if row.get("cleanup_error") or not all(cleanup.get(k) for k in ("run_stopped", "service_stopped", "resources_settled", "native_zero", "worktree_removed")):
                    raise RuntimeError("owned cleanup not complete; stop before another sample")
                if row["valid_publication_sample"]:
                    summary["valid_samples"].append(row)
                    consecutive_front = 0
                else:
                    summary["front_failures"].append(row)
                    consecutive_front += 1
                formal.save(root/"summary.json", summary)
                print(json.dumps(row, ensure_ascii=False), flush=True)
                ordinal += 1
                if args.phase == "gate":
                    summary["qualified"] = row["valid_publication_sample"] and row["passed"]
                    if not summary["qualified"]:
                        raise RuntimeError("real gate failed; no comparison started or sample reused")
                    break
                if row["valid_publication_sample"]:
                    break
                if consecutive_front >= 3:
                    raise RuntimeError("three consecutive front failures; stop and diagnose; none counted")
        if args.phase == "comparison":
            if len(summary["valid_samples"]) != 6 or any(sum(v["group"] == g for v in summary["valid_samples"]) != 3 for g in ("long", "short")):
                raise RuntimeError("effective sample quota not met")
            summary["qualified"] = all(v["passed"] for v in summary["valid_samples"])
        summary["status"] = "completed"
    except Exception as exc:
        summary.update(status="stopped", error=str(exc))
        raise
    finally:
        summary["finished_at"] = formal.now()
        formal.save(root/"summary.json", summary)
    if not summary["qualified"]:
        raise SystemExit("valid sample schedule completed with failures; no input replay")


if __name__ == "__main__":
    main()
