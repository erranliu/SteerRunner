import importlib.util
import json
import subprocess
import time
from pathlib import Path

ROOT = Path(__file__).resolve().parent
HISTORY = Path(r"C:\Users\lmas1\.codex\worktrees\d634\SteerRunner\.evidence")
spec = importlib.util.spec_from_file_location("prior_acceptance", HISTORY / "orca-workspace-receipt-live" / "acceptance.py")
prior = importlib.util.module_from_spec(spec)
spec.loader.exec_module(prior)
prior.ROOT = ROOT
prior.PREVIOUS = HISTORY / "helper-retirement-final"


class Run(prior.Run):
    def run_call(self, command, **kwargs):
        result = super().run_call(command, **kwargs)
        if command == "result-file":
            deadline = time.monotonic() + 10
            while result.get("collection_status") == "pending" and time.monotonic() < deadline:
                time.sleep(0.2)
                result = super().run_call(command, **kwargs)
            assert result.get("collection_status") == "closed" and result.get("version") == 1, result
        return result

    def request(self):
        data = super().request()
        data["title"] = "Profile decode PR audit " + self.side
        data["name_prefix"] = "decode-review-" + self.side
        (self.out / "original-input.json").write_text(json.dumps(data, indent=2), encoding="utf-8")
        return data


if __name__ == "__main__":
    import sys
    failures = []
    for side in ("local", "remote"):
        if sys.argv[1:] and side not in sys.argv[1:]:
            continue
        run = Run(side, "execute")
        try:
            data = run.request()
            checked = subprocess.run([str(ROOT / "writer.exe"), "validate", "--kind", "run", "--input", "-", "--json"], input=json.dumps(data).encode(), capture_output=True, timeout=30, creationflags=subprocess.CREATE_NO_WINDOW)
            (run.out / "validation.json").write_bytes(checked.stdout)
            assert checked.returncode == 0 and json.loads(checked.stdout)["ok"], checked.stdout.decode()
            run.verify()
        except Exception as error:
            print("BLOCKED", side, repr(error), run.out, flush=True)
            failures.append(side)
    if failures:
        raise SystemExit("Blocked: " + ", ".join(failures))
