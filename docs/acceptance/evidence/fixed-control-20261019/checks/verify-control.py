import json,pathlib,base64,hashlib,re,sys
root=pathlib.Path(__file__).parent
batch=root/(sys.argv[1] if len(sys.argv)>1 else 'native-r3')
def read(p):return json.loads(p.read_text(encoding='utf-8-sig'))
def decode(s):return json.loads(base64.urlsafe_b64decode(s+'='*((-len(s))%4)))
def arg(args,key):return args[args.index(key)+1] if key in args else ''
def response(row):
 r=row.get('response',{})
 return json.loads(r) if isinstance(r,str) else r
summary=read(batch/'summary.json')
assert summary['status']=='passed_with_exclusions',summary['status']
cases=[x for x in summary['results'] if x.get('level')=='L3' and x.get('status')!='excluded']
assert len(cases)==3 and all(x['status']=='passed' for x in cases)
facts=[]
for case in cases:
 ev=pathlib.Path(case['evidence']);raw=(ev/'native-calls.jsonl').read_bytes()
 rows=[json.loads(l) for l in raw.decode('utf-8-sig').splitlines()]
 creations=[(i,r) for i,r in enumerate(rows) if r.get('kind')=='intent' and r.get('args',[])[:2]==['terminal','create'] and "'--native-control'" in arg(r['args'],'--command')]
 assert len(creations)==1
 index,creation=creations[0];cid=creation['call_id']
 receipt=next(r for r in rows if r.get('kind')=='receipt' and r.get('call_id')==cid)
 native=response(receipt);terminal=native['result']['terminal'];handle=terminal['handle'];inc=terminal['incarnationId'];runtime=native['_meta']['runtimeId']
 command=arg(creation['args'],'--command');setup=decode(re.search(r"'--native-control' '([^']+)'$",command).group(1))
 main_sends=[(i,r) for i,r in enumerate(rows) if r.get('kind')=='intent' and r.get('args',[])[:2]==['terminal','send'] and arg(r['args'],'--text').startswith('[SteerRunner command=')]
 assert main_sends
 main=arg(main_sends[0][1]['args'],'--terminal')
 first_main=min(i for i,r in enumerate(rows) if r.get('kind')=='intent' and r.get('args',[])[:2]==['terminal','send'] and arg(r['args'],'--terminal')==main)
 ready_indices=[];turns=[];auth=[];shows=0;control_rows=[]
 for i,r in enumerate(rows):
  args=r.get('args',[])
  if arg(args,'--terminal')==handle or r.get('call_id')==cid:control_rows.append(r)
  if r.get('kind')!='receipt' or arg(args,'--terminal')!=handle:continue
  value=response(r)
  if not value.get('ok'):continue
  assert value['_meta']['runtimeId']==runtime
  if args[:2]==['terminal','show']:
   t=value['result']['terminal'];assert t['handle']==handle and t['incarnationId']==inc;shows+=1
  if args[:2]!=['terminal','read']:continue
  for line in value['result']['terminal'].get('tail',[]):
   line=line.strip()
   if line.startswith('SR_NATIVE_CONTROL_READY:'):
    assert decode(line.split(':',1)[1])==setup;ready_indices.append(i)
   if line.startswith('SR_NATIVE_CONTROL:'):
    result=decode(line.split(':',1)[1]);assert result['binding']==setup['binding']
    if result.get('turn') and result['turn'].get('completed'):turns.append(result['turn'])
    if result.get('authentication'):
     proof=result['authentication'];assert proof['revocation_confirmed'] and proof['broker_closed'] and proof['socket_cleanup_confirmed'];auth.append(proof)
 assert ready_indices and min(ready_indices)<first_main
 assert auth and shows>1
 if case['scenario']=='orca-remote.service-restart':assert turns and any(t.get('final_text') for t in turns)
 assert not any(r.get('kind')=='intent' and r.get('args',[])[:2]==['terminal','create'] and any(x in arg(r['args'],'--command') for x in ["'--original-turn'","'--authentication-stop'"]) for r in rows)
 audit=read(ev/'native-resource-audit.json');assert audit['verified'] and audit['inspections']==1 and audit['closed_inspections']==1
 fact={'scenario':case['scenario'],'case_id':case['case_id'],'fixture':str(ev.parent),'control':{'runtime':runtime,'terminal':handle,'incarnation':inc,'worktree':terminal['worktreeId']},'native_create_index':index,'control_ready_index':min(ready_indices),'first_main_send_index':first_main,'identity_checks':shows,'completed_receipt_observations':len(turns),'authentication_proof_observations':len(auth),'auxiliary_creations':1,'per_query_or_revocation_creations':0,'audit':audit,'raw_native_calls_sha256':hashlib.sha256(raw).hexdigest()}
 facts.append(fact)
 (ev/'fixed-control-native-proof.json').write_text(json.dumps(fact,indent=2)+'\n',encoding='utf-8')
 with (ev/'fixed-control-native-calls.jsonl').open('w',encoding='utf-8',newline='\n') as f:
  for r in control_rows:f.write(json.dumps(r,ensure_ascii=False)+'\n')
 driver=(ev/'faults.jsonl').read_bytes()
 fact['raw_driver_faults_sha256']=hashlib.sha256(driver).hexdigest()
 retained={'stage','service-instance','service-stop-intent','service-stopped','explicit-driver-authorization','observed-original-service','run-snapshot'}
 with (ev/'fixed-control-driver-events.jsonl').open('w',encoding='utf-8',newline='\n') as f:
  for line in driver.decode('utf-8-sig').splitlines():
   row=json.loads(line)
   if row.get('kind') in retained:f.write(json.dumps(row,ensure_ascii=False)+'\n')
 (ev/'fixed-control-native-proof.json').write_text(json.dumps(fact,indent=2)+'\n',encoding='utf-8')
(root/'control-verification.json').write_text(json.dumps({'verified':True,'source':summary['source_commit'],'cases':facts},indent=2)+'\n',encoding='utf-8')
print(json.dumps({'verified':True,'cases':len(facts)}))
