import pathlib,json,hashlib,subprocess,shutil,datetime
root=pathlib.Path.cwd();src=pathlib.Path(__file__).parent;batch=src/'final-latest';dst=root/'docs/acceptance/evidence/rebase-control-20261019'
def read(p):return json.loads(p.read_text(encoding='utf-8-sig'))
def write(p,v):p.parent.mkdir(parents=True,exist_ok=True);p.write_text(json.dumps(v,ensure_ascii=False,indent=2)+'\n',encoding='utf-8',newline='\n')
origins={}
def copy(p,name):
 target=dst/name;target.parent.mkdir(parents=True,exist_ok=True);shutil.copyfile(p,target)
 origins[name]={'original':str(p.resolve()),'sha256':hashlib.sha256(p.read_bytes()).hexdigest()}
s=read(batch/'summary.json');assert s['status']=='passed_with_exclusions' and s['executed_case_count']==3 and s['passed_case_count']==3
assert s['source_commit']=='bb84ea80cab2e37c2c7d184fcd82765dde2b1d32' and not s['source_dirty']
assert read(src/'control-verification.json')['verified']
probes=read(src/'probes.json');assert len(probes)==2 and all(x['exit_code']==0 for x in probes)
for name in ['summary.json','frozen-config.json','frozen-catalog.json','bin/steerrunner.exe.build.json','l1.jsonl','l1-stderr.log','terminal-extensions.tap','native-compile-stderr.log','framework/result.json','controlled/result.json','controlled/source-identity.json','controlled/integration.log','controlled/steerrunner.exe.build.json','controlled/unit-foundation.log']:
 copy(batch/name,'prerequisites/'+name)
for name in ['component.log','component-r2.log','stop-consumer.log','helper-build-r1.json','helper-build-r2.json','helper-build-r3.json','helper-created-r3.json','helper-read-r3.json','helper-show-r3.json','helper-closed-r3.json','stage-reuse-r3.log','upload-helper-r2.log','live-config-r2.json','test-config-r3.json','source-audit.json','audit-source.py','code-equivalence.py','code-equivalence.json','capture-validation.json','capture-probe-regression-final.log','signature-archive-initial.log','signature-exact-bytes.log','signature-third-export.log','export-exit/main.go','newmain-components.log','newmain-components-r2.log','newmain-framework.log','newmain-framework-r2.log','newmain-framework-r3.log','control-verification.json','archive.py','render-final.py','validate-archive.py','configure-latest.py','post-proofs.ps1','post-proofs.log','verify-control.py','verify-proof/main.go','stage-helper.ps1','stage-reuse-helper.ps1','upload-helper.cjs','probes.ps1','after-final.ps1','probe-local.log','probe-remote.log','probes.json']:
 copy(src/name,'checks/'+name)
for name in ['result.json','native.log']:
 copy(src/'capture-native-zero-match'/name,'checks/capture-native-zero-match/'+name)
for name in ['probes-initial-failed.json','probe-local-initial-failed.log']:
 copy(src/name,'historical/'+name)
for p in pathlib.Path('C:/SteerlineDebug/sr-rebase-control-20261019-local-bb84ea80').glob('*.json'):
 copy(p,'historical/failed-local-probe/'+p.name)
for name in ['summary.json','l1.jsonl','l1-stderr.log']:
 copy(src/'l1'/name,'historical/l1-eight-minute/'+name)
for name in ['application12-result.json','application12.jsonl','application12-stderr.log']:
 copy(src/name,'historical/'+name)
for name in ['summary.json','l1.jsonl','l1-stderr.log','terminal-extensions.tap','framework/result.json','controlled/source-identity.json','controlled/integration.log']:
 copy(src/'final'/name,'historical/interrupted-before-latest-main/'+name)
for name in ['interrupted-before-latest-main.json','interrupted-service-stop.json','interrupted-service-status.json','interrupted-service-residue-before.json','interrupted-service-residue-stop.json','interrupted-service-residue-after.json','interrupted-service-residue-stop-bound.json','interrupted-service-residue-after-bound.json','interrupted-processes-final.json']:
 copy(src/name,'historical/'+name)
for attempt in ['newmain-framework','newmain-framework-r2']:
 for name in ['configuration/boundary-isolation/local-codex.smoke-default-r1-case.json','configuration/boundary-isolation/local-codex.smoke-default-r1-worker-stderr.log']:
  copy(src/attempt/name,'historical/'+attempt+'/'+name)
cleanup=[];prs=[]
for case in [x for x in s['results'] if x.get('level')=='L3' and x.get('status')!='excluded']:
 assert case['status']=='passed'
 ev=pathlib.Path(case['evidence']);fixture=ev.parent;prefix='native/'+case['scenario']+'/'+case['case_id']+'/'+fixture.name
 assert read(fixture/'manifest.json')['source_commit']==s['source_commit']
 signature=read(ev/'native-signature-proof.json');assert signature['signature_verified'] and signature['writers_absent'] and signature['authentication_closed']
 for p in ev.glob('*.json'):
  if not p.name.endswith('-events.json'):copy(p,prefix+'/'+p.name)
 for name in ['native.log','fixed-control-native-calls.jsonl','fixed-control-driver-events.jsonl']:
  copy(ev/name,prefix+'/'+name)
 for name in ['manifest.json','bin/steerrunner.exe.build.json']:copy(fixture/name,prefix+'/'+pathlib.Path(name).name)
 query=subprocess.run([str(fixture/'bin/steerrunner.exe'),'service','status','--state-dir',str(fixture/'s'),'--json'],capture_output=True,text=True,encoding='utf-8',timeout=30)
 data=json.loads(query.stdout)['data'];assert query.returncode==0 and data['status']=='not_running' and data['last_shutdown']['resources_settled']
 write(dst/prefix/'service-status-final.json',{'at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'exit_code':query.returncode,'stdout':query.stdout,'stderr':query.stderr})
 cleanup.append({'scenario':case['scenario'],'case_id':case['case_id'],'fixture':fixture.name,'evidence':prefix,'service_stopped':True,'resources_settled':True,'signature_verified':True})
 manifest=read(fixture/'manifest.json')
 query=subprocess.run(['gh','pr','list','--repo','erranliu/SteerlineTest','--base',manifest['base_branch'],'--state','all','--json','url,state,headRefName,headRefOid,mergedAt'],capture_output=True,text=True,encoding='utf-8',timeout=30)
 assert query.returncode==0;found=json.loads(query.stdout);write(dst/prefix/'test-prs.json',found)
 prs.extend({'fixture':fixture.name,**p} for p in found)
for probe in probes:
 ev=pathlib.Path(probe['evidence']);prefix='probes/'+probe['name']
 result=read(ev/'result.json');assert result['passed'] and result['resources_cleaned'] and result['real_codex'] and result['business_inputs']==0
 signature=read(ev/'native-signature-proof.json');assert signature['signature_verified'] and signature['writers_absent'] and signature['authentication_closed']
 control=read(ev/'native-control-proof.json');assert control['control']['closed'] and control['zero_business_turns']
 for p in ev.glob('*.json'):copy(p,prefix+'/'+p.name)
 cleanup.append({'probe':probe['name'],'evidence':prefix,'resources_cleaned':True,'signature_verified':True,'zero_business_turns':True})
write(dst/'cleanup-summary.json',cleanup);write(dst/'created-test-prs.json',prs);write(dst/'raw-origins.json',origins)
files=[]
for name in subprocess.check_output(['git','ls-tree','-r','--name-only','HEAD'],text=True).splitlines():
 if name.startswith(('internal/','cmd/','scripts/','tests/')) or name in ['go.mod','go.sum']:files.append({'path':name,'sha256':hashlib.sha256((root/name).read_bytes()).hexdigest()})
write(dst/'source-files.json',{'source_commit':subprocess.check_output(['git','rev-parse','HEAD'],text=True).strip(),'test_execution_source_commit':s['source_commit'],'files':files})
write(dst/'verification.json',{'source_commit':s['source_commit'],'main':'2aa17cce7da6fd75e59db76e18f2521d9e9777db','helper':read(src/'helper-build-r3.json'),'l1':next(x for x in s['results'] if x['level']=='L1'),'controlled':next(x for x in s['results'] if x['level']=='Controlled'),'native_cases_passed':3,'excluded_admin_cases':4,'native_stop_probes_passed':2,'capture_test_source_commit':read(src/'code-equivalence.json')['capture_test_source_commit'],'production_and_controlled_sources_identical':True,'all_signatures_and_cleanup_verified':True,'prior_failed_batches_preserved':True})
print(json.dumps({'raw_files':len(origins),'source_files':len(files),'cleanup':cleanup,'test_prs':prs},ensure_ascii=False))
