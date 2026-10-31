import fs from 'node:fs';
import path from 'node:path';
import crypto from 'node:crypto';
import zlib from 'node:zlib';
import assert from 'node:assert/strict';
import {execFileSync} from 'node:child_process';
import {fileURLToPath} from 'node:url';
const root=path.dirname(fileURLToPath(import.meta.url)),out=path.resolve(process.argv[2]||'');
const sha=b=>crypto.createHash('sha256').update(b).digest('hex');
const json=f=>JSON.parse(fs.readFileSync(path.join(root,f),'utf8').replace(/^\uFEFF/,''));
const events=f=>zlib.gunzipSync(fs.readFileSync(path.join(root,f))).toString('utf8').trimEnd().split('\n').map(JSON.parse);
assert(process.argv[2]&&!fs.existsSync(out),'Specify a new absent extraction directory');
const m=json('manifest.json');
for(const f of m.files){assert.equal(path.basename(f.path),f.path);const b=fs.readFileSync(path.join(root,f.path));assert.equal(b.length,f.bytes);assert.equal(sha(b),f.sha256);}
fs.mkdirSync(out,{recursive:true});let records=0;
for(const f of m.files.filter(x=>x.path.endsWith('-records.jsonl.gz'))){
 const dir=path.join(out,f.path.slice(0,-'-records.jsonl.gz'.length));fs.mkdirSync(dir);
 for(const r of events(f.path)){assert.equal(path.basename(r.path),r.path);const b=Buffer.from(r.original_base64,'base64');assert.equal(sha(b),r.sha256);fs.writeFileSync(path.join(dir,r.path),b);records++;}
}
const c=json('controlled-summary.json'),e=events('controlled-tests.jsonl.gz');
assert.equal(c.executed,7170);assert.equal(c.passed,7170);assert.equal(c.failed,0);assert.equal(c.skipped,0);assert.equal(c.cases.length,239);assert.equal(c.required_roots.length,25);assert.deepEqual(c.missing_roots,[]);assert.equal(json('controlled-result.json').passed,true);assert.equal(json('controlled-result.json').source_digest,m.source_digest);
for(const r of c.cases){assert.equal(r.executed,30);assert.equal(r.passed,30);assert.equal(r.failed,0);assert.equal(r.skipped,0);assert.equal(e.filter(x=>x.Test===r.id&&x.Action==='pass').length,30);}
const q=events('constructor-original-query-30.jsonl.gz'),names=[...new Set(q.filter(x=>x.Test).map(x=>x.Test))],leaves=names.filter(n=>!names.some(x=>x.startsWith(n+'/')));
assert.equal(leaves.length,15);for(const n of leaves)assert.equal(q.filter(x=>x.Test===n&&x.Action==='pass').length,30);assert(!q.some(x=>x.Action==='fail'||x.Action==='skip'));
const red=events('final-red.jsonl.gz');assert.equal(red.filter(x=>x.Action==='fail'&&x.Test?.includes('/')).length,4);assert.equal(red.filter(x=>x.Output?.includes('original input query calls=0')).length,4);assert.equal(fs.readFileSync(path.join(root,'final-red-exit.txt'),'utf8').trim(),'1');
const l1=json('l1-audit.json'),lr=json('l1-summary.json'),le=events('l1-tests.jsonl.gz');assert.equal(lr.status,'passed');assert.equal(l1.status,'passed');assert.equal(le.filter(x=>x.Action==='fail').length,0);assert.equal(le.filter(x=>x.Action==='pass'&&x.Test&&!x.Test.includes('/')).length,l1.top_pass);assert.equal(le.filter(x=>x.Action==='pass'&&x.Test).length,l1.pass_including_subtests);
const local=JSON.parse(execFileSync(process.execPath,[path.join(root,'audit-local-sessions.mjs'),path.join(root,'submitted-input-records.jsonl'),...['01','02','03'].map(i=>path.join(out,'local-'+i))],{encoding:'utf8'}));
const remote=JSON.parse(execFileSync(process.execPath,[path.join(root,'audit-remote-evidence.mjs'),...['01','02','03'].map(i=>path.join(out,'remote-'+i))],{encoding:'utf8'}));
for(const s of json('sessions.json').sessions){assert.equal(s.source_digest,m.source_digest);assert.equal(s.passed,true);assert.equal(s.uses_helper,s.side==='local');assert.equal(s.actual_model_verified,s.side==='local');assert.equal(s.wire_format,s.side==='local'?'native-model-command-v1':'');}
for(const p of json('submitted-input-provenance.json').records){const rows=fs.readFileSync(path.join(root,'submitted-input-records.jsonl'),'utf8').trimEnd().split('\n').filter(l=>JSON.parse(l).session_id===p.session_id);assert.equal(rows.length,1);assert.equal(sha(Buffer.from(rows[0])),p.original_line_sha256);}
for(const side of ['local','remote'])for(let i=1;i<=3;i++){
 const label=`${side}-0${i}`,read=n=>JSON.parse(fs.readFileSync(path.join(out,'workspaces',label+'-'+n+'.json'),'utf8'));
 const create=read('create'),absent=read('absence'),census=read('empty-census');assert.equal(absent.ok,false);assert.equal(absent.error.code,'selector_not_found');assert.equal(absent.error.data.selector,'id:'+create.result.worktree.id);assert.equal(absent._meta.runtimeId,create._meta.runtimeId);assert.equal(census.result.totalCount,0);assert.equal(census.result.truncated,false);assert.equal(census._meta.runtimeId,create._meta.runtimeId);
}
const files=json('source-file-hashes.json').files.map(x=>({...x})),mapping=json('source-index-mapping.json');assert.equal(mapping.checked_source_files,files.length);
assert.equal(sha(Buffer.from(files.map(x=>x.path+':'+x.sha256).join('\n'))),m.source_digest);
for(const n of mapping.normalizations){const row=files.find(x=>x.path===n.path);assert.equal(row.sha256,n.raw_sha256);row.sha256=n.git_blob_sha256;}
assert.equal(sha(Buffer.from(files.map(x=>x.path+':'+x.sha256).join('\n'))),mapping.git_source_content_digest);
console.log(JSON.stringify({files:m.files.length,decoded_records:records,local:local.sessions.length,remote:remote.sessions.length,controlled:7170,query_regressions:450,l1:l1.status,source_digest:m.source_digest}));
