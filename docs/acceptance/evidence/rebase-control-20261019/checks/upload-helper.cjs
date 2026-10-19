const fs=require('node:fs');
const path=require('node:path');
const crypto=require('node:crypto');
const {RuntimeClient}=require('C:/Users/lmas1/AppData/Local/Programs/orca/resources/app.asar.unpacked/out/cli/runtime-client.js');
(async()=>{
 const root=__dirname;
 const artifact=fs.readFileSync(path.join(root,'steerrunner-agent.exe'));
 const sha=crypto.createHash('sha256').update(artifact).digest('hex');
 const staging=JSON.parse(fs.readFileSync(path.join(root,'staging-worktree.json'),'utf8'));
 const worktree='id:'+staging.result.worktree.id;
 const expected=staging._meta.runtimeId;
 const client=new RuntimeClient(undefined,60000,null,'e68f4edf-8a83-42c2-a9da-6d404d3aa3d8');
 const dir='.sr-fixed-control-20261019-'+sha.slice(0,16);
 const relativePath=dir+'/helper.exe';
 const call=async(method,params)=>{const r=await client.call(method,{worktree,expectedExecutionHostId:'local',...params});if(!r.ok||r._meta.runtimeId!==expected)throw Error('upload result identity mismatch');return r;};
 await call('files.createDirNoClobber',{relativePath:dir});
 for(let off=0;off<artifact.length;off+=384*1024){await call('files.writeBase64Chunk',{relativePath,contentBase64:artifact.subarray(off,off+384*1024).toString('base64'),append:off>0});}
 const result={sha256:sha,bytes:artifact.length,worktree:staging.result.worktree.id,runtime:expected,source:staging.result.worktree.path+'/'+relativePath,destination:'C:/sr-runtime/1c3c-fixed-control-20261019/'+sha.slice(0,16)+'/steerrunner-agent.exe'};
 fs.writeFileSync(path.join(root,'uploaded-helper.json'),JSON.stringify(result,null,2)+'\n');
 process.stdout.write(JSON.stringify({sha256:sha,bytes:artifact.length,uploaded:true})+'\n');
})().catch(e=>{process.stderr.write(String(e.message)+'\n');process.exitCode=1;});
