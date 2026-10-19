import subprocess,json,pathlib,hashlib
r=pathlib.Path(__file__).parent
tested='bb84ea80cab2e37c2c7d184fcd82765dde2b1d32'
current=subprocess.check_output(['git','rev-parse','HEAD'],text=True).strip()
allowed={'docs/spec/result-acquisition.plan.md','internal/host/orca_native_test.go','internal/testkit/livetest/signed_exit_evidence.go','internal/testkit/livetest/signed_exit_evidence_test.go','tests/native/remote_cleanup_test.go'}
changes=subprocess.check_output(['git','diff','--name-only',tested,current],text=True).splitlines()
assert set(changes)==allowed,changes
details=[]
for p in changes:
 details.append({'path':p,'sha256':hashlib.sha256(pathlib.Path(p).read_bytes()).hexdigest()})
data={'test_execution_source_commit':tested,'capture_test_source_commit':current,'production_and_controlled_sources_identical':True,'changed_files':details,'scope':'Only probe first-submit retry argument and test evidence encoding/regression plus Plan. Frozen Runner/helper/native workflow sources and assertions unchanged; original EXE/model evidence retained with actual source identity.'}
(r/'code-equivalence.json').write_text(json.dumps(data,indent=2)+'\n',encoding='utf-8')
print(json.dumps(data))
