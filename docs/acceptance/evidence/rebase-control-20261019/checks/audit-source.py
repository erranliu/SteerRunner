import pathlib, subprocess, hashlib, json
r=pathlib.Path(__file__).parent
def blob(ref,path):
    return subprocess.check_output(['git','show',ref+':'+path])
main='2aa17cce7da6fd75e59db76e18f2521d9e9777db'
source=subprocess.check_output(['git','rev-parse','HEAD'],text=True).strip()
native=[]
for path in [
    'internal/execution/adapters/codex/remote_exit.go',
    'internal/execution/adapters/codex/remote_turn.go',
    'internal/execution/adapters/protocol/orca/artifact.go',
    'internal/execution/adapters/protocol/orca/stopped.go']:
    a,b=blob(main,path),blob(source,path)
    assert a==b,path
    native.append({'path':path,'identical_to_main':True,'git_blob_sha256':hashlib.sha256(b).hexdigest()})
execution=subprocess.check_output(['git','diff','--name-only',
    'codex/rebase-a3dc-verified-l1-20261019',source,'--','internal/execution'],text=True)
assert not execution.strip(),execution
report={
    'source_commit':source,'latest_integrated_main':main,
    'native_proof_files':native,
    'execution_source_unchanged_by_independent_configuration_integration':True,
    'manual_review':[
        'Original helper Ed25519 private key remains memory-only; original readiness owner/public key bind the full native Job/TUI/authentication exit body.',
        'Main history reader Job creation, cleanup and ReaderWritersAbsent semantics retained; history diff only adds NativeHome accessor.',
        'A query failure may carry cleanup facts; validateRecordedTurn rejects InspectionFailed as business success.',
        'Checkpoint replacement validates original input/readiness/reader settlement first; no unknown reader response is overwritten by query or revocation.',
        'Stop revokes original authentication then requests and verifies original signed exit before closing original business PTY; unknown proof retains original route.',
        'Passive shutdown and offline verification cannot promote PTY close into stopped, cannot recreate control/reader or send business input.',
        'Record/Intents/Commit/CAS retains business state authority; caller checkpoint is bound transport evidence only, not a workflow queue.',
        'Independent configuration keeps validate, capability identity, frozen configuration/catalog/build/requirements digests and per-case availability; workflow selections consistently use version 2.',
        'UTF-8 is scoped to native JSON capture and restored; obsolete feedback qualification fields are removed rather than accepted as ignored configuration.',
        'Initialization creation identity unknown remains blocking; no replacement-by-title or empty-inventory inference.'
    ],
    'unresolved_findings':[],
    'qualification':'Manual source review plus byte checks only; final tests and native proofs recorded separately.'
}
(r/'source-audit.json').write_text(json.dumps(report,indent=2)+'\n',encoding='utf-8')
print(json.dumps({'source':source,'main':main,'native_files_preserved':len(native),'unresolved_findings':0}))
