import pathlib,json,hashlib,subprocess,shutil,datetime,sys
root=pathlib.Path.cwd();src=root/'.evidence/fixed-control-20261019';dst=root/'docs/acceptance/evidence/fixed-control-20261019'
batch=src/(sys.argv[1] if len(sys.argv)>1 else 'native-final')
l1=src/'native-r5';controlled=src/'controlled-r5-25'
def read(p):return json.loads(p.read_text(encoding='utf-8-sig'))
def write(p,v):p.parent.mkdir(parents=True,exist_ok=True);p.write_text(json.dumps(v,ensure_ascii=False,indent=2)+'\n',encoding='utf-8',newline='\n')
origins={}
def copy(p,relative):
 target=dst/relative;target.parent.mkdir(parents=True,exist_ok=True);shutil.copyfile(p,target)
 origins[relative]={'original':str(p),'sha256':hashlib.sha256(p.read_bytes()).hexdigest()}
summary=read(batch/'summary.json');assert summary['status']=='passed_with_exclusions',summary['status']
verification=read(src/'control-verification.json');assert verification['verified'] and len(verification['cases'])==3
assert summary['source_commit']=='212ebee3abeae798c05fe53ede19f8b2a5e0f0f3'
dst.mkdir(parents=True,exist_ok=True)
for name in ['component-final.log','cleanup-budget-regression.log','helper-build-r5.json','environment.json','live-config-r5.json','interrupted-r1.json','interrupted-r2.json','probe-created.json','probe-ready.json','probe-send.json','probe-response.json','probe-retry-exact.json','probe-after-retry.json','probe-closed.json','prototype-helper-upload.json','helper-created-r5.json','helper-read-r5.json','helper-show-r5.json','helper-closed-r5.json','control-verification.json','verify-control.py','stage-helper.ps1','upload-helper.cjs','run-controlled-25.ps1','run-final-native.ps1']:
 copy(src/name,'checks/'+name)
copy(batch/'summary.json','prerequisites/summary.json')
for name in ['l1.jsonl','l1-stderr.log','terminal-extensions.tap','native-compile-stderr.log','framework/result.json']:
 copy(l1/name,'prerequisites/'+name)
for name in ['result.json','source-identity.json','integration.log']:
 copy(controlled/name,'prerequisites/controlled/'+name)
copy(l1/'controlled/steerrunner.exe.build.json','prerequisites/controlled/steerrunner.exe.build.json')
copy(l1/'controlled/source-identity.json','prerequisites/controlled/original-source-identity.json')
for name in ['summary.json','controlled/integration.log','controlled/source-identity.json']:
 copy(l1/name,'historical/native-r5-timeout/'+name)
for name in ['controlled-timeout-service-stop.json','controlled-timeout-service-status.json','r4-failed-final-inventory.json','r4-failed-final-service.json']:
 copy(src/name,'historical/'+name)
cleanup=[];prs=[]
cases=[x for x in summary['results'] if x.get('level')=='L3' and x.get('status')!='excluded']
for case in cases:
 assert case['status']=='passed'
 ev=pathlib.Path(case['evidence']);fixture=ev.parent;prefix='native/'+case['scenario']+'/'+case['case_id']+'/'+fixture.name
 for f in ev.glob('*.json'):
  if f.name.endswith('-events.json'):continue
  copy(f,prefix+'/'+f.name)
 for name in ['native.log','driver-commands.jsonl','fixed-control-native-calls.jsonl','fixed-control-driver-events.jsonl']:
  p=ev/name
  if p.is_file():copy(p,prefix+'/'+name)
 for name in ['manifest.json','bin/steerrunner.exe.build.json']:copy(fixture/name,prefix+'/'+pathlib.Path(name).name)
 manifest=read(fixture/'manifest.json');assert manifest['source_commit']==summary['source_commit'] and not manifest['source_dirty']
 query=subprocess.run([str(fixture/'bin/steerrunner.exe'),'service','status','--state-dir',str(fixture/'s'),'--json'],capture_output=True,text=True,encoding='utf-8',timeout=30)
 proof={'at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'fixture':str(fixture),'exit_code':query.returncode,'stdout':query.stdout,'stderr':query.stderr};write(dst/prefix/'service-status-final.json',proof)
 data=json.loads(query.stdout)['data'];assert query.returncode==0 and data['status']=='not_running' and data['last_shutdown']['resources_settled']
 cleanup.append({'scenario':case['scenario'],'case_id':case['case_id'],'fixture':fixture.name,'status':data['status'],'resources_settled':True,'evidence':prefix+'/service-status-final.json'})
 query=subprocess.run(['gh','pr','list','--repo','erranliu/SteerlineTest','--base',manifest['base_branch'],'--state','all','--json','url,state,headRefName,headRefOid,mergedAt'],capture_output=True,text=True,encoding='utf-8',timeout=30)
 assert query.returncode==0;found=json.loads(query.stdout);write(dst/prefix/'test-prs.json',found)
 prs.extend({'fixture':fixture.name,'scenario':case['scenario'],'case_id':case['case_id'],**p} for p in found)
for old in ['native-r3','native-r4']:
 for name in ['summary.json']:
  copy(src/old/name,'historical/'+old+'/'+name)
 for path in (src/old).glob('*-r1-case.json'):
  item=read(path)
  if item.get('status')!='excluded':
   copy(path,'historical/'+old+'/'+path.name)
   if item.get('evidence'):
    ev=pathlib.Path(item['evidence'])
    for name in ['scenario.json','cleanup.json','native-resource-audit.json','native.log','runner-stop-still-unverified.json']:
     if (ev/name).is_file():copy(ev/name,'historical/'+old+'/'+ev.parent.name+'/'+name)
copy(src/'post-cleanup-audit-recheck.jsonl','historical/post-cleanup-audit-recheck.jsonl')
write(dst/'cleanup-summary.json',cleanup);write(dst/'created-test-prs.json',prs);write(dst/'raw-origins.json',origins)
source=summary['source_commit'];names=subprocess.check_output(['git','ls-tree','-r','--name-only',source],text=True).splitlines();files=[]
for name in names:
 if not name.startswith(('internal/','cmd/','scripts/','tests/')) and name not in ['go.mod','go.sum']:continue
 raw=(root/name).read_bytes();files.append({'path':name,'sha256':hashlib.sha256(raw).hexdigest()})
write(dst/'source-files.json',{'commit':source,'dirty':False,'files':files})
write(dst/'verification.json',{'source_commit':source,'helper':read(src/'helper-build-r5.json'),'scope':summary['scope'],'actual_native_cases':3,'excluded_admin_cases':4,'control_verification':True,'final_cleanup_verified':True,'l1':next(x for x in summary['results'] if x['level']=='L1'),'controlled':next(x for x in summary['results'] if x['level']=='Controlled')})
print(json.dumps({'raw_files':len(origins),'cleanup':cleanup,'test_prs':[p['url'] for p in prs]},ensure_ascii=False))
