import importlib.util
import json
from pathlib import Path

ROOT = Path(__file__).resolve().parent
HISTORY = Path(r"C:\Users\lmas1\.codex\worktrees\d634\SteerRunner\.evidence")
spec = importlib.util.spec_from_file_location("prior_acceptance", HISTORY / "orca-workspace-receipt-live" / "acceptance.py")
prior = importlib.util.module_from_spec(spec)
spec.loader.exec_module(prior)
prior.ROOT = ROOT
prior.PREVIOUS = HISTORY / "helper-retirement-final"


class Run(prior.Run):
    def request(self):
        data = super().request()
        data["title"] = "Workspace creation facts equivalence " + self.side
        data["name_prefix"] = "facts-" + self.side
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
            run.verify()
        except Exception as error:
            print("BLOCKED", side, repr(error), run.out, flush=True)
            failures.append(side)
    if failures:
        raise SystemExit("Blocked: " + ", ".join(failures))
