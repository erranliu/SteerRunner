import pathlib,re,json,hashlib,urllib.parse,subprocess
root=pathlib.Path.cwd();dst=root/'docs/acceptance/evidence/fixed-control-20261019'
def read(p):return json.loads(p.read_text(encoding='utf-8-sig'))
def write(p,v):p.write_text(json.dumps(v,ensure_ascii=False,indent=2)+'\n',encoding='utf-8',newline='\n')
origins=read(dst/'raw-origins.json');errors=[];count=0
if not (dst/'hashes.json').exists():write(dst/'hashes.json',{})
for name,record in origins.items():
 if hashlib.sha256((dst/name).read_bytes()).hexdigest()!=record['sha256']:errors.append('raw mismatch '+name)
source=read(dst/'source-files.json')
for entry in source['files']:
 if hashlib.sha256((root/entry['path']).read_bytes()).hexdigest()!=entry['sha256']:errors.append('production source changed '+entry['path'])
for p in dst.rglob('*.json'):
 try:read(p)
 except Exception as e:errors.append('JSON invalid '+str(p.relative_to(dst)))
secret=re.compile(rb'ghp_[A-Za-z0-9]{20,}|github_pat_[A-Za-z0-9_]{20,}|sk-proj-[A-Za-z0-9_-]{20,}|-----BEGIN [A-Z ]*PRIVATE KEY-----')
for p in dst.rglob('*'):
 if not p.is_file():continue
 if p.suffix.lower() in ['.exe','.db','.sqlite','.pem','.key']:errors.append('private/binary file '+str(p.relative_to(dst)))
 if secret.search(p.read_bytes()):errors.append('secret pattern '+str(p.relative_to(dst)))
links=0
for p in [root/'README.md',*sorted((root/'docs').rglob('*.md'))]:
 count+=1;text=p.read_text(encoding='utf-8-sig')
 if re.search(r'^(<<<<<<< |=======\s*$|>>>>>>> )',text,re.M):errors.append('conflict marker '+str(p.relative_to(root)))
 body=re.sub(r'^```.*?^```[^\n]*','',text,flags=re.S|re.M);body=re.sub(r'`+[^`]*`+','',body)
 for target in re.findall(r'\]\((<[^>]+>|[^\s)]+)(?:\s+"[^\"]*")?\)',body):
  target=target.strip('<>')
  if ':' in target or target.startswith('#'):continue
  target=urllib.parse.unquote(target.split('#',1)[0])
  if not target:continue
  links+=1
  if not (p.parent/target).exists():errors.append('missing link '+str(p.relative_to(root))+' '+target)
for name in ['docs/spec/result-acquisition.spec.md','docs/spec/result-acquisition.plan.md','docs/acceptance/result-acquisition.md']:
 if 'RA-A08' not in (root/name).read_text(encoding='utf-8'):errors.append('RA-A08 missing '+name)
diff=subprocess.run(['git','diff','--check'],capture_output=True,text=True)
if diff.returncode:errors.append('git diff --check '+diff.stdout+diff.stderr)
result={'markdown_files':count,'local_links':links,'original_raw_files':len(origins),'source_files':len(source['files']),'json_valid':True,'secret_pattern_scan':'passed' if not any('secret pattern' in e for e in errors) else 'failed','errors':errors}
write(dst/'doc-check.json',result)
if errors:print(json.dumps(result,ensure_ascii=False));raise SystemExit(1)
hashes={str(p.relative_to(dst)).replace('\\','/'):hashlib.sha256(p.read_bytes()).hexdigest() for p in sorted(dst.rglob('*')) if p.is_file() and p.name!='hashes.json'}
write(dst/'hashes.json',hashes)
print(json.dumps(result,ensure_ascii=False))
