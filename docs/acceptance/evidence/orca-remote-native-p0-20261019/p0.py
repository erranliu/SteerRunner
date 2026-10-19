from pathlib import Path
import json,subprocess,sys,datetime,os,hashlib
E=Path(__file__).resolve().parent
M=json.loads((E/'identity.json').read_text(encoding='utf-8'))
def now():return datetime.datetime.now(datetime.timezone.utc).isoformat()
def save(name,obj): (E/name).write_text(json.dumps(obj,ensure_ascii=False,indent=2),encoding='utf-8')
def call(label,args,timeout=55):
 command=[M['orca'],*args,'--environment',M['environment'],'--json']
 env={k:v for k,v in os.environ.items() if k not in ['ORCA_ENVIRONMENT','ORCA_PAIRING_CODE','ORCA_REMOTE_PAIRING']}
 row={'at':now(),'label':label,'command':command}
 with (E/'calls.jsonl').open('a',encoding='utf-8') as f:f.write(json.dumps(row,ensure_ascii=False)+'\n')
 try:r=subprocess.run(command,capture_output=True,text=True,encoding='utf-8',env=env,timeout=timeout)
 except subprocess.TimeoutExpired as exc:
  save(label+'-timeout.json',{'at':now(),'error':'timeout','seconds':timeout});raise
 (E/(label+'.json')).write_text(r.stdout,encoding='utf-8');(E/(label+'.stderr')).write_text(r.stderr,encoding='utf-8')
 try:v=json.loads(r.stdout)
 except: print(r.stdout,r.stderr);raise
 if r.returncode or not v.get('ok'):print(json.dumps(v,ensure_ascii=True));raise RuntimeError(label+' failed '+str(r.returncode))
 if v.get('_meta',{}).get('runtimeId')!=M['runtime']:raise RuntimeError('foreign runtime')
 return v
if __name__=='__main__':
 mode=sys.argv[1]
 if mode=='orca':print(json.dumps(call(sys.argv[2],sys.argv[3:]),ensure_ascii=True,indent=2))
 elif mode=='query':
  label=sys.argv[2];command=[str(E/'state-probe.exe'),str(E/'resource.json'),*sys.argv[3:]]
  with (E/(label+'.jsonl')).open('w',encoding='utf-8') as f:r=subprocess.run(command,stdout=f,stderr=subprocess.PIPE,text=True,encoding='utf-8',timeout=55)
  (E/(label+'.stderr')).write_text(r.stderr,encoding='utf-8')
  rows=[json.loads(x) for x in (E/(label+'.jsonl')).read_text(encoding='utf-8').splitlines()]
  print(json.dumps([x for x in rows if x.get('kind')!='tabs'],ensure_ascii=True,indent=2))
