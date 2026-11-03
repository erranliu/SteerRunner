// P0 diagnostic only. Uses public Orca runtime IPC/RPC; no Runner runtime wiring.
const fs = require('node:fs');
const path = require('node:path');
const net = require('node:net');
const crypto = require('node:crypto');
const ROOT = 'C:/Users/lmas1/AppData/Local/Programs/orca/resources';
const WS = require(path.join(ROOT, 'node_modules/ws'));
const nacl = require(path.join(ROOT, 'node_modules/tweetnacl'));
const evidence = __dirname;
const mutation = require(path.join(ROOT,'app.asar.unpacked/out/shared/agent-session/lifecycle/structured-agent-session-mutation.js'));
function canonical(value) {
  if(value===null||typeof value!=='object')return JSON.stringify(value??null);
  if(Array.isArray(value))return '['+value.map(canonical).join(',')+']';
  return '{'+Object.entries(value).filter(([,v])=>v!==undefined).sort(([a],[b])=>a<b?-1:a>b?1:0)
    .map(([k,v])=>JSON.stringify(k)+':'+canonical(v)).join(',')+'}';
}
function fingerprint(method,sessionId,fields){
  const input={method,sessionId,fields};
  const digest=crypto.createHash('sha256').update(canonical(input),'utf8').digest('hex');
  if(digest!==mutation.structuredAgentSessionPayloadFingerprint(input))throw new Error('P0 fingerprint vector mismatch');
  return digest;
}
const caps = ['agent-session.structured.v1', 'agent-session.host-authority.v1',
  'agent-session.session-boundary.v1', 'agent-session.structured.registered-agents.v1',
  'agent-session.structured.client-launch-mode.v1', 'agentSession.create.message.v1',
  'agentSession.create.options-mode.v1', 'agent-session.send-selection.v1',
  'agent-session.send-options.v1', 'agent-session.accepted-send.v1',
  'agent-session.pending-send-result.v1', 'agent-session.send-answers-proof.v1',
  'agent-session.current-requests.v1', 'agent-session.turn-item.v1',
  'agent-session.conversation-stop.v1', 'agent-session.question-answers.v1',
  'agent-session.structured.pi-dialogs.v1', 'agent-session.item-detail.v1',
  'agent-session.coherent-model-catalog.v1', 'agent-session.permission-modes.v1',
  'agent-session.permission-own.v1', 'agent-session.snapshot-catch-up.v1'];
function clean(value) {
  if (Array.isArray(value)) return value.map(clean);
  if (value && typeof value === 'object') return Object.fromEntries(Object.entries(value)
    .filter(([k]) => !/authToken|deviceToken|secret|password|launchEnv|accountHome|apiKey|accessToken|refreshToken|authorization/i.test(k))
    .map(([k,v]) => [k, clean(v)]));
  return value;
}
function save(name, value) { fs.writeFileSync(path.join(evidence, name), JSON.stringify(clean(value), null, 2) + '\n'); }
class Queue {
  constructor() { this.items=[]; this.waiters=[]; this.error=null; }
  push(x) { const w=this.waiters.shift(); if(w) w.resolve(x); else this.items.push(x); }
  fail(e) { this.error=e; for(const w of this.waiters.splice(0))w.reject(e); }
  pop(ms=15000) {
    if(this.items.length)return Promise.resolve(this.items.shift());
    if(this.error)return Promise.reject(this.error);
    return new Promise((resolve,reject)=>{
      const w={resolve:x=>{clearTimeout(t);resolve(x)},reject:e=>{clearTimeout(t);reject(e)}};
      const t=setTimeout(()=>{this.waiters=this.waiters.filter(x=>x!==w);reject(new Error('P0 read deadline; no mutation replay'));},ms);
      this.waiters.push(w);
    });
  }
}
async function connect(target) {
  const dataDir=process.env.ORCA_USER_DATA_PATH||path.join(process.env.APPDATA,'orca');
  const expected=JSON.parse(fs.readFileSync(path.join(evidence,target+'-status.json'),'utf8'))._meta.runtimeId;
  const q=new Queue(); let socket,token,key;
  if(target==='local') {
    const meta=JSON.parse(fs.readFileSync(path.join(dataDir,'orca-runtime.json'),'utf8'));
    if(meta.runtimeId!==expected)throw new Error('P0 original local runtime changed');
    token=meta.authToken;
    const transport=(meta.transports||[meta.transport]).find(x=>x.kind==='named-pipe');
    if(!transport)throw new Error('P0 named pipe unavailable');
    socket=net.connect(transport.endpoint); let pending='';
    socket.setEncoding('utf8');
    socket.on('data',chunk=>{pending+=chunk; if(Buffer.byteLength(pending)>4*1024*1024){socket.destroy();q.fail(new Error('P0 frame limit'));return;}
      let at;while((at=pending.indexOf('\n'))>=0){const row=pending.slice(0,at);pending=pending.slice(at+1);if(row.trim())try{q.push(JSON.parse(row))}catch{q.fail(new Error('invalid IPC JSON'))}}});
    socket.on('error',e=>q.fail(new Error(e.code||'IPC error')));
    await new Promise((resolve,reject)=>{socket.once('connect',resolve);socket.once('error',reject)});
  } else {
    const store=JSON.parse(fs.readFileSync(path.join(dataDir,'orca-environments.json'),'utf8'));
    const env=store.environments.find(x=>x.id==='e68f4edf-8a83-42c2-a9da-6d404d3aa3d8');
    if(!env)throw new Error('P0 fixed environment not found');
    const endpoint=env.endpoints.find(x=>x.id===env.preferredEndpointId);token=endpoint.deviceToken;
    socket=new WS(endpoint.endpoint,{handshakeTimeout:5000,maxPayload:4*1024*1024});
    socket.on('message',data=>q.push(String(data)));
    socket.on('error',e=>q.fail(new Error(e.code||'WebSocket error')));
    await new Promise((resolve,reject)=>{socket.once('open',resolve);socket.once('error',reject)});
    const pair=nacl.box.keyPair();key=nacl.box.before(Buffer.from(endpoint.publicKeyB64,'base64'),pair.secretKey);
    socket.send(JSON.stringify({type:'e2ee_hello',publicKeyB64:Buffer.from(pair.publicKey).toString('base64')}));
    if(JSON.parse(await q.pop()).type!=='e2ee_ready')throw new Error('P0 remote handshake refused');
    const nonce=nacl.randomBytes(24);
    const cipher=nacl.secretbox(Buffer.from(JSON.stringify({type:'e2ee_auth',deviceToken:token,clientCapabilities:caps})),nonce,key);
    socket.send(Buffer.concat([Buffer.from(nonce),Buffer.from(cipher)]).toString('base64'));
    const encoded=Buffer.from(await q.pop(),'base64');const auth=nacl.secretbox.open(encoded.subarray(24),encoded.subarray(0,24),key);
    if(!auth||JSON.parse(Buffer.from(auth).toString('utf8')).type!=='e2ee_authenticated')throw new Error('P0 remote auth refused');
  }
  async function receive(ms) {
    let x=await q.pop(ms);
    if(key){const encoded=Buffer.from(x,'base64');const plain=nacl.secretbox.open(encoded.subarray(24),encoded.subarray(0,24),key);
      if(!plain)throw new Error('P0 remote identity decryption failed');x=JSON.parse(Buffer.from(plain).toString('utf8'));}
    if(x._keepalive)return receive(ms);
    if(x._meta?.runtimeId!==expected)throw new Error('P0 original response runtime changed');
    return x;
  }
  const client={target,expected,async request(method,params,ms=15000){
    const id=crypto.randomUUID();const envelope={id,method,params,clientCapabilities:caps,...(key?{deviceToken:token}:{authToken:token})};
    if(key){const nonce=nacl.randomBytes(24);const cipher=nacl.secretbox(Buffer.from(JSON.stringify(envelope)),nonce,key);
      socket.send(Buffer.concat([Buffer.from(nonce),Buffer.from(cipher)]).toString('base64'));}
    else socket.write(JSON.stringify(envelope)+'\n');
    const response=await receive(ms);
    if(response.id!==id)throw new Error('P0 response request mismatch');
    return response;
  },receive,close(){if(key)socket.terminate();else socket.destroy();}};
  const status=await client.request('status.get',null);
  if(!status.ok){client.close();throw new Error('P0 status refused: '+status.error?.code)}
  return client;
}
async function main(){
  const label='local-codex-atomic-complex';
  if(fs.existsSync(path.join(evidence,label+'-intent.json')))throw Error('Original probe already attempted; no replay');
  const created=JSON.parse(fs.readFileSync(path.join(evidence,'local-worktree-create.json'),'utf8').replace(/^\uFEFF/,''));
  if(!created.ok)throw Error('Isolated worktree unavailable');
  const worktree=created.result.worktree;
  const client=await connect('local');
  let sessionId,createAccepted=false;
  try{
    const support=await client.request('agentSession.createSupport',{worktree:'id:'+worktree.id,agent:'codex'});save('local-codex-createSupport.json',support);
    if(!support.ok||!support.result.supported)throw Error('Codex structured creation unsupported');
    sessionId='oas_mvp_'+crypto.randomUUID().replaceAll('-','');
    const clientOperationId=mutation.createStructuredAgentSessionOperationId(crypto.randomUUID);
    const clientMessageId=mutation.createStructuredAgentSessionOperationId(crypto.randomUUID);
    const text='This is an isolated structured-input integration check. Work only in the current workspace; do not modify repository files, commit, publish, or inspect credentials.\r\nFirst send a short commentary saying STAGE_ONE, then use the shell tool to print MVP_TOOL_ONE. Next send a second commentary saying STAGE_TWO and use the shell tool again to print MVP_TOOL_TWO. Finally return exactly the 300 lines MVP_FINAL_LINE_0001 through MVP_FINAL_LINE_0300, with one line per number. Do not abbreviate the final output.\r\n\r\nLiteral input preservation only: 中文 😀 "quotes" docs\\spec\\example.md\r\n```text\r\n<&> a\\b\r\n```\r\n  ';
    const body={kind:'message',role:'user',blocks:[{type:'text',text}]};
    const fields={worktree:'id:'+worktree.id,agent:'codex',options:{permissionMode:'full-access'},firstMessage:{clientMessageId,body,selection:{model:'gpt-6.1-sol',effort:'medium'}}};
    const params={envelope:{sessionId,clientOperationId,expectedRuntimeFence:null,payloadFingerprint:fingerprint('agentSession.create',sessionId,fields)},...fields};
    save(label+'-intent.json',{runtime:client.expected,worktree,params,input_sha256:crypto.createHash('sha256').update(text,'utf8').digest('hex')});
    const response=await client.request('agentSession.create',params,30000);save(label+'-create.json',response);
    if(!response.ok||!response.result.ok)throw Error('Atomic creation refused; no resend');
    createAccepted=true;
    let history;const observations=[];const until=Date.now()+180000;
    while(Date.now()<until){
      history=await client.request('agentSession.history',{sessionId,direction:'tail',limit:200},20000);
      if(!history.ok||!history.result.ok)throw Error('History unavailable; no replay');
      const page=history.result.page;observations.push({at:Date.now(),fence:page.fence,working:page.working,turn:page.latestTurn?.turn,items:page.items.length});
      if(page.latestTurn?.turn?.completedAt)break;
      await new Promise(r=>setTimeout(r,1000));
    }
    save(label+'-observations.json',observations);save(label+'-history.json',history);
    const options=await client.request('agentSession.options',{sessionId},20000);save(label+'-options.json',options);
    const pages=[];let cursor;
    for(let i=0;i<30;i++){
      const page=await client.request('agentSession.history',{sessionId,direction:cursor?'before':'tail',limit:2,...cursor?{cursor}:{}},20000);pages.push(page);
      if(!page.ok||!page.result.ok)break;
      if(!page.result.page.hasOlder)break;
      const next=page.result.page.window?.nextCursor;
      if(!next||JSON.stringify(next)===JSON.stringify(cursor))throw Error('History cursor stalled');cursor=next;
    }
    save(label+'-pages.json',pages);
    const items=history.result.page.items;const assistants=items.filter(i=>i.body?.kind==='message'&&i.body.role==='assistant');
    for(let i=0;i<assistants.length;i++)save(label+'-assistant-item-'+i+'.json',await client.request('agentSession.item',{sessionId,itemId:assistants[i].itemId},20000));
    const user=items.filter(i=>i.body?.kind==='message'&&i.body.role==='user');
    const summary={sessionId,atomic_selection:fields.firstMessage.selection,permission_options:fields.options,original_body_equal:user.length===1&&JSON.stringify(user[0].body)===JSON.stringify(body),user_rows:user.length,turn:history.result.page.latestTurn?.turn,options:options.result?.current,assistant_items:assistants.map(i=>({itemId:i.itemId,sequence:i.sequence,turnScope:i.turnScope,state:i.body.state,blocks:i.body.blocks.map(b=>({type:b.type,characters:b.text?.length,name:b.name}))})),tool_items:items.filter(i=>i.body?.blocks?.some(b=>b.type==='tool-call'||b.type==='tool-result')).length,pages:pages.length,has_full_final:assistants.some(i=>i.body.blocks.some(b=>b.text?.includes('MVP_FINAL_LINE_0001')&&b.text?.includes('MVP_FINAL_LINE_0300')))};
    save(label+'-summary.json',summary);console.log(JSON.stringify(summary));
  }finally{
    if(createAccepted){save(label+'-close-intent.json',{sessionId});save(label+'-close.json',await client.request('agentSession.close',{sessionId},30000));save(label+'-after-close.json',await client.request('agentSession.history',{sessionId,direction:'tail',limit:3},20000));}
    client.close();
  }
}
main().catch(e=>{console.error(e.message);process.exitCode=1});
