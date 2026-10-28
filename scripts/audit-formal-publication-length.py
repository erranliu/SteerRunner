"""Offline audit of six formal Runner samples, preserving every failed case."""
import argparse
import collections
import hashlib
import json
from pathlib import Path


def load(path):
    return json.loads(path.read_text(encoding="utf-8-sig"))


def sha(data):
    return hashlib.sha256(data).hexdigest()


def encode(value):
    return json.dumps(value, ensure_ascii=False, separators=(",", ":")).replace("&", "\\u0026").replace("<", "\\u003c").replace(">", "\\u003e").replace("\u2028", "\\u2028").replace("\u2029", "\\u2029")


def native(text):
    return text.replace("\r\n", "\n").replace("\r", "\n").rstrip("\n")


def audit(root):
    summary = load(root/"summary.json")
    assert summary["status"] == "completed" and summary["active_case"] is None
    effective = summary.get("valid_samples", summary["rows"])
    assert len(effective) == 6
    effective_roots = {r["root"] for r in effective}
    new_scope = summary.get("schema") == "valid-formal-publication-comparison/v1"
    rows, baseline, providers = [], None, set()
    for ordinal, source in enumerate(summary["rows"], 1):
        case = root/source["root"]
        record = load(case/"closed-run-record.json")
        request = load(case/"frozen-request.json")
        hidden = source["group"] == "short"
        assert source["runner_sha256"] == summary["runner_sha256"] == summary["source_build"]["sha256"]
        assert request["workflow"]["omit_publish_source_facts"] == hidden
        assert record["request"]["workflow"]["omit_publish_source_facts"] if hidden else not record["request"]["workflow"].get("omit_publish_source_facts", False)
        comparable = json.loads(json.dumps(request))
        del comparable["name_prefix"]
        del comparable["workflow"]["omit_publish_source_facts"]
        for pair in (2, 3):
            comparable["workflow"]["prompt_overrides"]["publish"] = comparable["workflow"]["prompt_overrides"]["publish"].replace(f"SRPUB{pair:02d}", "SRPUB01")
        setup_failure = source["cleanup"].get("no_workspace_created", False)
        if not setup_failure:
            if baseline is None:
                baseline = comparable
            assert comparable == baseline, "unapproved between-case request change"
        assert all(source["cleanup"].get(k) for k in ("run_stopped", "service_stopped", "resources_settled", "native_zero"))
        assert source["cleanup"].get("worktree_removed") or setup_failure
        assert load(case/"verify-stopped.json")["resources_settled"]
        if setup_failure:
            assert not record.get("native_started") and not record.get("workspace_fact")
            assert not record["snapshot"].get("Implementation")
            inventory = load(sorted(case.glob("orca-no-worktree-created-inventory-attempt*.result.json"))[-1])
            assert inventory["ok"] and all(source["run_id"] not in json.dumps(w) for w in inventory["result"]["worktrees"])
        else:
            census = sorted(case.glob("orca-cleanup-census-attempt*.result.json"))[-1]
            value = load(census)["result"]
            assert value["totalCount"] == 0 and value["terminals"] == [] and value["truncated"] is False
            assert value["hostScope"]["hostIds"] and value["hostScope"]["omittedHostIds"] == []
            after_reads = [load(p) for p in case.glob("orca-*worktree-after*.result.json")]
            assert any(v["ok"] is False and v["error"]["code"] == "selector_not_found" for v in after_reads)
        row = {k: source.get(k) for k in ("ordinal", "pair", "group", "root", "passed", "reached_publication", "error", "run_id", "marker")}
        row["cleanup_verified"] = True
        row["setup_failure_before_native_session"] = setup_failure
        inputs = record["workflow"].get("StageInputs") or []
        if new_scope and inputs:
            first_text = native(inputs[0]["prompt"])
            assert len(first_text) == 833 and sha(first_text.encode()) == "ddbdf515b767483e34347e17abf27d7b046856d2c6d2188f3517bfabf5ef53de"
        stages = {p["command_id"]: p for p in inputs}
        pubs = [p for p in inputs if p["step_id"] == "publish.execute"]
        row["stages"] = [p["step_id"] for p in inputs]
        effects, shapes, tool_output, cli_errors = collections.defaultdict(list), collections.defaultdict(list), False, []
        for path in (Path(source["state_dir"])/"native-input").rglob("*.result.json"):
            if path.name == "input.result.json" or path.name.endswith(".call-result.json"):
                continue
            call = load(path)
            if not call.get("executable"):
                continue
            identity = load(path.parent/"input.intent.json")["input"]
            command = identity["command_id"]
            args = call["args"]
            if call.get("error") or call.get("exit_code", 0) != 0:
                cli_errors.append(dict(at=call.get("finished_at"), command_id=command, phase=identity["phase"], error=call.get("error"), exit_code=call.get("exit_code")))
            if args[:2] == ["terminal", "send"] and call.get("invoked"):
                kind = "paste" if "--text" in args else "enter" if "--enter" in args else "other"
                effects[command].append(kind)
                if kind == "paste":
                    body = native(stages[command]["prompt"])
                    assert args[args.index("--text")+1] == "\x1b[200~"+body+"\x1b[201~"
            if args[:2] == ["terminal", "read"] and call.get("stdout"):
                try:
                    terminal = json.loads(call["stdout"]).get("result", {}).get("terminal", {})
                    tool_output |= any("└ SR_INPUT_TOOL_OUTPUT" in line for line in terminal.get("tail", []))
                    draft = terminal.get("draft", "")
                    if draft:
                        expected = native(stages[command]["prompt"])
                        shape = "plain-exact" if draft == expected else "collapsed-exact" if draft == f"[Pasted Content {len(expected)} chars]" else "different"
                        shapes[command].append(dict(shape=shape, scalars=len(draft), sha256=sha(draft.encode())))
                except (ValueError, KeyError):
                    pass
        for values in effects.values():
            assert values.count("paste") <= 1 and values.count("enter") <= 1, "original effect repeated"
        row.update(effects=dict(effects), draft_observations=dict(shapes), cli_errors=cli_errors, first_tool_output_observed=tool_output)
        row["valid_publication_sample"] = source["root"] in effective_roots
        if new_scope:
            first_complete = any(v["CommandID"] == inputs[0]["command_id"] and v["FinalVerified"] and v["Succeeded"] for v in record["workflow"].get("StageResults") or []) if inputs else False
            observed_valid = first_complete and tool_output and len(pubs) == 1 and effects.get(pubs[0]["command_id"], []).count("paste") == 1
            assert observed_valid == row["valid_publication_sample"] == source["valid_publication_sample"]
            row["first_task_canonical_verified"] = first_complete
            viewport = collections.defaultdict(list)
            for path in (Path(source["state_dir"])/"native-input").rglob("*.viewport-result.json"):
                event = load(path)
                rpc = event["rpc"]
                assert rpc["StartedAt"] and rpc["FinishedAt"]
                if source["passed"]:
                    assert rpc["RequestID"] and not rpc["Error"]
                if rpc["Method"] == "terminal.resizeForClient":
                    viewport[event["input"]["command_id"]].append(rpc)
            if source["passed"]:
                for command, calls in viewport.items():
                    calls.sort(key=lambda v: v["StartedAt"])
                    assert len(calls) == 2
                    fit, restore = calls
                    assert fit["Params"]["mode"] == "mobile-fit" and restore["Params"]["mode"] == "restore"
                    assert fit["Params"]["clientId"] == restore["Params"]["clientId"]
                    assert fit["Params"]["terminal"] == restore["Params"]["terminal"]
                    a, z = fit["Result"]["terminal"], restore["Result"]["terminal"]
                    assert (a["cols"], a["rows"]) == (1024, 80)
                    assert (z["cols"], z["rows"]) == (a["previousCols"], a["previousRows"])
                row["owned_viewport_restored"] = True
        if setup_failure:
            row["failure_class"] = "repository_directory_mismatch_before_native_workspace"
        elif not pubs and inputs and all("enter" not in v for v in effects.values()):
            row["failure_class"] = "first_input_draft_mismatch_no_submit_call"
            row["first_input_scalars"] = len(native(inputs[0]["prompt"]))
            row["first_input_sha256"] = sha(native(inputs[0]["prompt"]).encode())
        else:
            row["failure_class"] = None if source["passed"] else "see_original_receipts"
        if pubs:
            assert len(pubs) == 1
            pub = pubs[0]
            results = record["workflow"]["results"]
            impl = next(v for v in results if v["id"] == pub["implementation_result_id"])
            assert impl["status"] == "satisfied" and impl["contract"] == "implementation"
            assert impl["attempts"] and impl["fields"]
            assert all(impl["source"].get(k) for k in ("session_id", "command_id", "turn_id", "attempt_id", "input_version", "final_digest"))
            audit_id = pub.get("audit_result_id")
            audit_result = next((v for v in results if v["id"] == audit_id), None)
            facts = dict(audit_result=audit_result, implementation_result=impl, materials=pub["material_facts"])
            body = record["prompt_bodies"]["publish"]
            visible = body+"\nSource facts:\n"+encode(facts)
            tail = "\n完成后，在最终回复末尾单独写一行："+pub["success_output"]
            tail += "\nFixed execution facts: "+encode(dict(publication_policy=record["request"]["publication"]))+"\n"
            tail += "\nPR target: "+encode(dict(remote=record["request"]["repository"]["remote"], base_branch=record["request"]["repository"]["base_branch"]))+"\n"
            if record["request"].get("progress_file"):
                tail += "\nOptional caller-owned byte report: STEERLINE_PROGRESS_FILE contains the absolute path of this Run's report file. Write complete raw bytes there (max 65536 bytes); coordinate writers with a lock, read/merge if needed, then atomically replace using a temporary file in the same directory. Runner preserves bytes and does not interpret them.\n"
            expected = native((body if hidden else visible)+tail)
            actual = native(pub["prompt"])
            assert actual == expected, "rendered text changed outside the approved visible facts choice"
            row.update(input_scalars=len(actual), input_bytes=len(actual.encode()), input_sha256=sha(actual.encode()),
                       internal_facts_complete=True, implementation_source=impl["source"],
                       visible_facts_scalars=len("\nSource facts:\n"+encode(facts)),
                       virtual_long_scalars=len(native(visible+tail)), virtual_short_scalars=len(native(body+tail)))
            assert (len(actual) < 1000 and "\nSource facts:\n" not in actual) if hidden else "\nSource facts:\n" in actual
            row["publication_effects"] = effects.get(pub["command_id"], [])
        if source["passed"]:
            assert len(inputs) == 2 and row["stages"] == ["implement.execute", "publish.execute"]
            assert tool_output
            assert all(sorted(effects[p["command_id"]]) == ["enter", "paste"] for p in inputs)
            independent = load(case/"independent-original.json")
            assert independent["passed"] and all(independent[k] for k in ("matches_process", "matches_query", "matches_text", "matches_final"))
            assert independent["original_input"]["Text"] == actual and independent["original_input"]["ID"]
            provider = independent["original"]["providerSession"]["id"]
            assert provider not in providers
            providers.add(provider)
            canonical = record["workflow"]["StageResults"]
            assert len(canonical) == 2 and all(v["FinalVerified"] and v["Succeeded"] for v in canonical)
            row.update(public_original_id=independent["original_input"]["ID"], provider=provider, independent_verified=True)
        rows.append(row)
    groups = []
    for group in ("long", "short"):
        selected = [r for r in rows if r["group"] == group and r["root"] in effective_roots]
        assert len(selected) == 3
        groups.append(dict(group=group, attempts=3, reached_publication=sum(bool(r["reached_publication"]) for r in selected),
                           passed=sum(bool(r["passed"]) for r in selected), failed=sum(not r["passed"] for r in selected),
                           input_scalars=[r.get("input_scalars") for r in selected], input_bytes=[r.get("input_bytes") for r in selected]))
    return dict(schema="formal-publication-length-independent-audit/v1", audit_passed=True,
                audit_scope="offline evidence consistency; does not promote pre-publication failures into target trials",
                target_comparison_complete=all(r["reached_publication"] for r in rows if r["root"] in effective_roots),
                formal_publication_qualification_passed=all(r["passed"] for r in rows if r["root"] in effective_roots),
                front_failures_preserved=[r for r in rows if r["root"] not in effective_roots],
                source_build=summary["source_build"], groups=groups, rows=rows, cleanup_verified=True,
                business_tests_run=0, business_rounds_counted=0)


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("root", type=Path)
    parser.add_argument("--out", type=Path, required=True)
    args = parser.parse_args()
    value = audit(args.root)
    with args.out.open("xb") as out:
        out.write((json.dumps(value, ensure_ascii=False, indent=2)+"\n").encode())
    print(json.dumps({"audit_passed": value["audit_passed"], "groups": value["groups"]}, ensure_ascii=False))
