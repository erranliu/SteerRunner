"""Verify the archived initialization runs sent no control or business input."""
import hashlib
import json
from pathlib import Path

root = Path(__file__).resolve().parent
for case in ['before-readiness', 'restart-before-readiness']:
    folder = root / case
    window = json.loads((folder / 'initialization-window.json').read_text(encoding='utf-8-sig'))
    snapshot = window['snapshot']
    # Read the original transport payload, never reconstruct a live session.
    state = next(json.loads(e['payload']) if isinstance(e['payload'], str) else e['payload']
                 for e in snapshot['Implementation'] if e['implementation_key'] == 'orca.terminal-state')
    calls = [json.loads(line) for line in (folder / 'native-calls.jsonl').read_text(encoding='utf-8-sig').splitlines()]
    sends = [c['args'] for c in calls if c.get('kind') == 'intent' and c.get('args', [])[:2] == ['terminal', 'send']]
    assert len(sends) == 1, (case, 'unexpected native/control input')
    args = sends[0]
    assert args[args.index('--terminal') + 1] == state['process']['terminal']
    assert args[args.index('--terminal') + 1] != state['control']['process']['terminal']
    text = args[args.index('--text') + 1]
    assert hashlib.sha256(text.encode()).hexdigest() == state['launch_digest']
    if case == 'restart-before-readiness':
        restart = json.loads((folder / 'initialization-restart.json').read_text(encoding='utf-8-sig'))
        after = next(e['payload'] for e in restart['snapshot']['Implementation']
                     if e['implementation_key'] == 'orca.terminal-state')
        assert after['control'] == state['control']
        assert after['remote_ownership'] == state['remote_ownership']
    print(case + ': exactly one original helper launch; zero control requests and business sends')
