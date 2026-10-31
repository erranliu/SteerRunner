import fs from 'node:fs';
import path from 'node:path';
import crypto from 'node:crypto';
import zlib from 'node:zlib';
import assert from 'node:assert/strict';
import {execFileSync} from 'node:child_process';
import {fileURLToPath} from 'node:url';
const root=path.dirname(fileURLToPath(import.meta.url));
const sha=b=>crypto.createHash('sha256').update(b).digest('hex');
const json=f=>JSON.parse(fs.readFileSync(path.join(root,f),'utf8').replace(/^\uFEFF/,''));
const out=path.resolve(process.argv[2]||'');
assert(process.argv[2]&&!fs.existsSync(out),'Specify a new, absent extraction directory');
const manifest=json('manifest.json');
for(const f of manifest.files){const b=fs.readFileSync(path.join(root,f.path));assert.equal(b.length,f.bytes);assert.equal(sha(b),f.sha256);}
fs.mkdirSync(out,{recursive:true});
let records=0;
for(const f of manifest.files.filter(x=>x.path.endsWith('-records.jsonl.gz'))){
  const dir=path.join(out,f.path.slice(0,-'-records.jsonl.gz'.length));fs.mkdirSync(dir);
  const rows=zlib.gunzipSync(fs.readFileSync(path.join(root,f.path))).toString('utf8').trimEnd().split('\n').map(JSON.parse);
  for(const r of rows){assert.equal(path.basename(r.path),r.path);const b=Buffer.from(r.original_base64,'base64');assert.equal(sha(b),r.sha256);fs.writeFileSync(path.join(dir,r.path),b);records++;}
}
const local=execFileSync(process.execPath,[path.join(root,'audit-local-sessions.mjs'),path.join(root,'submitted-input-records.jsonl'),...['01','02','03'].map(i=>path.join(out,'local-'+i))],{encoding:'utf8'});
const remote=execFileSync(process.execPath,[path.join(root,'audit-remote-evidence.mjs'),...['01','02','03'].map(i=>path.join(out,'remote-'+i))],{encoding:'utf8'});
const c=json('controlled-summary.json');assert.equal(c.executed,7050);assert.equal(c.passed,7050);assert.equal(c.failed,0);assert.equal(c.skipped,0);assert.deepEqual(c.missing_roots,[]);assert.equal(json('controlled-result.json').passed,true);
for(const x of c.cases){assert.equal(x.executed,30);assert.equal(x.passed,30);assert.equal(x.failed,0);assert.equal(x.skipped,0);}
assert.equal(c.cases.length,235);
// The formal entry already appends the bridge stream to tests.jsonl.
const events=zlib.gunzipSync(fs.readFileSync(path.join(root,'controlled-tests.jsonl.gz'))).toString('utf8').trimEnd().split('\n').map(JSON.parse);
for(const x of c.cases)assert.equal(events.filter(e=>e.Test===x.id&&e.Action==='pass').length,30);
for(const p of json('submitted-input-provenance.json').records){const lines=fs.readFileSync(path.join(root,'submitted-input-records.jsonl'),'utf8').trimEnd().split('\n');const selected=lines.filter(l=>JSON.parse(l).session_id===p.session_id);assert.equal(selected.length,1);assert.equal(sha(Buffer.from(selected[0])),p.original_line_sha256);assert.equal(sha(Buffer.from(JSON.parse(selected[0]).text)),p.text_sha256);}
const delta=json('fixture-fix-source.json');assert.equal(delta.changes.length,1);assert.equal(delta.changes[0].path,'internal/execution/adapters/orcatransport/terminal_query_windows_test.go');
const before=json('source-file-hashes.json');assert.equal(before.source_digest,delta.native_source_digest);const after=before.files.map(x=>({...x}));
const changed=after.find(x=>x.path===delta.changes[0].path);assert.equal(changed.sha256,delta.changes[0].before_sha256);changed.sha256=delta.changes[0].sha256;
assert.equal(sha(Buffer.from(after.map(x=>x.path+':'+x.sha256).join('\n'))),delta.final_test_source_digest);
const mapping=json('source-index-mapping.json'),canonical=after.map(x=>({...x}));assert.equal(mapping.checked_source_files,canonical.length);assert.equal(mapping.final_test_source_digest,delta.final_test_source_digest);
for(const n of mapping.normalizations){const row=canonical.find(x=>x.path===n.path);assert.equal(row.sha256,n.raw_sha256);row.sha256=n.git_blob_sha256;}
assert.equal(sha(Buffer.from(canonical.map(x=>x.path+':'+x.sha256).join('\n'))),mapping.git_source_content_digest);
const agent=json('rebuilt-agent.json');assert.equal(agent.identical,true);assert.equal(agent.original_agent_sha256,agent.rebuilt_agent_sha256);
const final=json('controlled-final-summary.json'),finalResult=json('controlled-final-result.json');assert.equal(final.executed,7050);assert.equal(final.passed,7050);assert.equal(final.failed,0);assert.equal(final.skipped,0);assert.deepEqual(final.missing_roots,[]);assert.equal(finalResult.passed,true);assert.equal(finalResult.source_digest,delta.final_test_source_digest);
const finalEvents=zlib.gunzipSync(fs.readFileSync(path.join(root,'controlled-final-tests.jsonl.gz'))).toString('utf8').trimEnd().split('\n').map(JSON.parse);
assert.equal(final.cases.length,235);for(const x of final.cases){assert.equal(x.executed,30);assert.equal(x.passed,30);assert.equal(finalEvents.filter(e=>e.Test===x.id&&e.Action==='pass').length,30);}
const rpc=json('rpc-cleanup-30-summary.json'),rpcEvents=zlib.gunzipSync(fs.readFileSync(path.join(root,'rpc-cleanup-30.jsonl.gz'))).toString('utf8').trimEnd().split('\n').map(JSON.parse);
assert.equal(rpc.executed,990);assert.equal(rpc.failed,0);assert.equal(rpc.skipped,0);for(const x of rpc.cases){assert.equal(x.executed,30);assert.equal(x.passed,30);assert.equal(rpcEvents.filter(e=>e.Test===x.id&&e.Action==='pass').length,30);}
assert.equal(json('l1-before-fixture-summary.json').status,'failed');
const l1=fs.existsSync(path.join(root,'l1-summary.json'))?json('l1-summary.json').status:'pending';assert(['passed','pending'].includes(l1));
if(l1==='passed'){
  const audit=json('l1-audit.json'),invocation=json('l1-invocation.json');assert.equal(invocation.source_digest,delta.final_test_source_digest);
  const raw=zlib.gunzipSync(fs.readFileSync(path.join(root,'l1-tests.jsonl.gz'))).toString('utf8').trimEnd().split('\n').map(JSON.parse);
  assert.equal(raw.filter(x=>x.Action==='pass'&&x.Test&&!x.Test.includes('/')).length,audit.top_level_passed);assert.equal(raw.filter(x=>x.Action==='pass'&&x.Test).length,audit.passed_including_subtests);
  assert.equal(raw.filter(x=>x.Action==='fail').length,0);assert.equal(raw.filter(x=>x.Action==='skip'&&x.Test).length,audit.explicit_test_skips);
}
process.stdout.write(JSON.stringify({archive_files:manifest.files.length,decoded_records:records,local:JSON.parse(local).sessions.length,remote:JSON.parse(remote).sessions.length,controlled_executed:c.executed,final_controlled_executed:final.executed,rpc_executed:rpc.executed,l1,native_source_digest:manifest.native_source_digest,final_test_source_digest:manifest.final_test_source_digest})+'\n');
