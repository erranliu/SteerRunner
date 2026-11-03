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
  const [mode,target='local',...rest]=process.argv.slice(2);
  if(mode==='read') {
    const method=rest[0],params=JSON.parse(rest[1]||'{}');
    const allowed=['agentSession.agents','agentSession.createSupport','agentSession.history','agentSession.item',
      'agentSession.options','agentSession.handoffStatus','agentSession.currentRequests','agentSession.modelCatalog',
      'session.tabs.list','status.get'];
    if(!allowed.includes(method))throw new Error('P0 read mode method not allowlisted');
    const client=await connect(target);
    try{const response=await client.request(method,params,20000);save(target+'-'+method.replaceAll('.','-')+'-'+(rest[2]||'result')+'.json',response);console.log(JSON.stringify(clean(response)));}
    finally{client.close()}
  }else if(mode==='discovery') {
    const worktree=JSON.parse(fs.readFileSync(path.join(evidence,target+'-worktree-create.json'),'utf8')).result.worktree;
    const client=await connect(target);const results=[];
    try {
      for(const agent of ['codex','pi']) {
        for(const [method,params] of [
          ['agentSession.createSupport',{worktree:'id:'+worktree.id,agent}],
          ['agentSession.modelCatalog',{worktree:'id:'+worktree.id,agent,savedOnly:true}]
        ]) {
          const result=await client.request(method,params,20000);
          save(target+'-'+agent+'-'+method.split('.').at(-1)+'.json',result);
          results.push({agent,method,response:result});
        }
      }
      const tabs=await client.request('session.tabs.list',{worktree:'id:'+worktree.id});
      save(target+'-tabs-before.json',tabs);
      console.log(JSON.stringify(clean({target,results,tabs:tabs.result})));
    }finally{client.close()}
  }else if(mode==='blank') {
    const agent=rest[0];if(!['codex','pi'].includes(agent))throw new Error('P0 agent not allowlisted');
    const label=target+'-'+agent+'-blank';
    if(fs.existsSync(path.join(evidence,label+'-intent.json')))throw new Error('P0 evidence already attempted; do not replay');
    const support=JSON.parse(fs.readFileSync(path.join(evidence,target+'-'+agent+'-createSupport.json'),'utf8'));
    if(!support.ok||!support.result.supported)throw new Error('P0 preflight refused; zero create');
    const worktree=JSON.parse(fs.readFileSync(path.join(evidence,target+'-worktree-create.json'),'utf8')).result.worktree;
    const sessionId='oas_p0_'+crypto.randomUUID().replaceAll('-','');
    const fields={worktree:'id:'+worktree.id,agent,...support.result.seedOptions?{options:support.result.seedOptions}:{}};
    const params={envelope:{sessionId,clientOperationId:mutation.createStructuredAgentSessionOperationId(crypto.randomUUID),expectedRuntimeFence:null,payloadFingerprint:fingerprint('agentSession.create',sessionId,fields)},...fields};
    save(label+'-intent.json',{runtime:target,sessionId,params});
    const client=await connect(target);let created=false;
    try {
      const result=await client.request('agentSession.create',params,30000);save(label+'-create.json',result);
      if(!result.ok){console.log(JSON.stringify(clean({label,created:false,response:result})));return}
      created=true;
      const reads={};
      for(const [method,p] of [['agentSession.history',{sessionId,direction:'tail',limit:30}],['agentSession.options',{sessionId}],['agentSession.handoffStatus',{sessionId}]]){
        const r=await client.request(method,p,20000);save(label+'-'+method.split('.').at(-1)+'.json',r);reads[method]=r;
      }
      const sub=await connect(target);
      try{const r=await sub.request('agentSession.subscribe',{sessionId},20000);save(label+'-snapshot.json',r);reads.snapshot=r}finally{sub.close()}
      console.log(JSON.stringify(clean({label,created,result,reads})));
    } finally {
      if(created){const closed=await client.request('agentSession.close',{sessionId},20000);save(label+'-close.json',closed);
        const after=await client.request('agentSession.handoffStatus',{sessionId},20000);save(label+'-after-close.json',after);
        console.log(JSON.stringify(clean({label,close:closed,after})));}
      client.close();
    }
  }else if(mode==='atomic'||mode==='split') {
    const agent=rest[0];if(!['codex','pi'].includes(agent))throw new Error('P0 agent not allowlisted');
    const label=target+'-'+agent+'-'+mode;
    if(fs.existsSync(path.join(evidence,label+'-intent.json')))throw new Error('P0 evidence already attempted; do not replay');
    const support=JSON.parse(fs.readFileSync(path.join(evidence,target+'-'+agent+'-createSupport.json'),'utf8'));
    if(!support.ok||!support.result.supported)throw new Error('P0 preflight refused; zero create');
    const worktree=JSON.parse(fs.readFileSync(path.join(evidence,target+'-worktree-create.json'),'utf8')).result.worktree;
    const sessionId='oas_p0_'+crypto.randomUUID().replaceAll('-','');
    const clientMessageId=mutation.createStructuredAgentSessionOperationId(crypto.randomUUID);
    const marker='P0_OAS_'+target.toUpperCase()+'_'+agent.toUpperCase()+'_OK';
    const text='This is an API input fidelity check. Do not use tools or read/write files. Reply exactly '+marker+'.\r\n'+
      'Literal path: docs\\spec\\example.md\r\n\r\nChinese: 中文; Unicode: 😀; quotes: "x"\r\n```text\r\na\\b\r\n```\r\n  ';
    const body={kind:'message',role:'user',blocks:[{type:'text',text}]};
    const fields={worktree:'id:'+worktree.id,agent,...support.result.seedOptions?{options:support.result.seedOptions}:{},
      ...(mode==='atomic'?{firstMessage:{clientMessageId,body}}:{})};
    const params={envelope:{sessionId,clientOperationId:mutation.createStructuredAgentSessionOperationId(crypto.randomUUID),expectedRuntimeFence:null,payloadFingerprint:fingerprint('agentSession.create',sessionId,fields)},...fields};
    save(label+'-intent.json',{runtime:target,sessionId,marker,params});
    const client=await connect(target);let created=false;const observations=[];const summary={label,sessionId,marker};
    try {
      const result=await client.request('agentSession.create',params,30000);save(label+'-create.json',result);
      if(!result.ok||result.result?.ok===false){summary.create_refused=result;return}
      created=true;summary.create=result.result;
      if(mode==='split'){
        const prior=JSON.parse(fs.readFileSync(path.join(evidence,target+'-'+agent+'-atomic-options.json'),'utf8')).result.current;
        const selection={model:prior.model,effort:prior.effort};
        const sendFields={body,selection};
        const send={envelope:{sessionId,clientOperationId:clientMessageId,expectedRuntimeFence:result.result.fence,
          payloadFingerprint:fingerprint('agentSession.send',sessionId,sendFields)},...sendFields};
        save(label+'-send-intent.json',send);
        const sent=await client.request('agentSession.send',send,30000);save(label+'-send.json',sent);summary.send=sent;
        if(!sent.ok||sent.result?.ok===false)return;
      }
      const deadline=Date.now()+60000;let sequence=-1;let last;
      while(Date.now()<deadline){
        const r=await client.request('agentSession.history',{sessionId,direction:'tail',limit:100},15000);
        const page=r.result?.page;const cursor=page?.liveCursor?.sequence;
        if(cursor!==sequence||!page){observations.push(r);sequence=cursor;save(label+'-observations.json',observations)}
        last=r;
        if(page?.latestTurn&&page.latestTurn.endedAt!==undefined&&page.latestTurn.endedAt!==null)break;
        if(page?.latestTurn&&['completed','failed','cancelled','interrupted','ended'].includes(page.latestTurn.state))break;
        if(page?.items?.some(i=>i.body?.kind==='message'&&i.body.role==='assistant')&&page.working===false)break;
        await new Promise(resolve=>setTimeout(resolve,1000));
      }
      save(label+'-history.json',last);
      for(const method of ['agentSession.options','agentSession.handoffStatus']){
        const r=await client.request(method,{sessionId},15000);save(label+'-'+method.split('.').at(-1)+'.json',r);
        summary[method]=method.endsWith('options')?{current:r.result?.current,modelsKnown:r.result?.modelsKnown}:r.result;
      }
      if(target==='remote'){
        const sub=await connect(target);try{const r=await sub.request('agentSession.subscribe',{sessionId},15000);save(label+'-snapshot.json',r);
          const version=r.result?.currentRequests?.version;if(version){const current=await client.request('agentSession.currentRequests',{sessionId,version});save(label+'-currentRequests.json',current)}
        }finally{sub.close()}
      }
      summary.latestTurn=last?.result?.page?.latestTurn;summary.working=last?.result?.page?.working;
      summary.items=last?.result?.page?.items;summary.submissions=last?.result?.page?.submissions;
    } finally {
      if(created){
        save(label+'-close-intent.json',{sessionId,method:'agentSession.close',noMutationEnvelope:true});
        const closed=await client.request('agentSession.close',{sessionId},25000);save(label+'-close.json',closed);summary.close=closed;
        const after=await client.request('agentSession.history',{sessionId,direction:'tail',limit:100},15000);save(label+'-after-close.json',after);
        summary.afterCloseLatestTurn=after.result?.page?.latestTurn;
      }
      client.close();save(label+'-summary.json',summary);console.log(JSON.stringify(clean(summary)));
    }
  }else if(mode==='active') {
    const agent=rest[0];if(!['codex','pi'].includes(agent))throw new Error('P0 agent not allowlisted');
    const label=target+'-'+agent+'-active';
    if(fs.existsSync(path.join(evidence,label+'-intent.json')))throw new Error('P0 evidence already attempted; do not replay');
    const support=JSON.parse(fs.readFileSync(path.join(evidence,target+'-'+agent+'-createSupport.json'),'utf8'));
    if(!support.ok||!support.result.supported)throw new Error('P0 preflight refused; zero create');
    const worktree=JSON.parse(fs.readFileSync(path.join(evidence,target+'-worktree-create.json'),'utf8')).result.worktree;
    const sessionId='oas_p0_'+crypto.randomUUID().replaceAll('-','');
    const clientMessageId=mutation.createStructuredAgentSessionOperationId(crypto.randomUUID);
    const body={kind:'message',role:'user',blocks:[{type:'text',text:'Cancellation API probe. Do not use tools, read files, or change anything. Generate 3000 numbered lines of short original arithmetic examples. Keep producing lines until the list is complete.'}]};
    const fields={worktree:'id:'+worktree.id,agent,...support.result.seedOptions?{options:support.result.seedOptions}:{},firstMessage:{clientMessageId,body}};
    const params={envelope:{sessionId,clientOperationId:mutation.createStructuredAgentSessionOperationId(crypto.randomUUID),expectedRuntimeFence:null,payloadFingerprint:fingerprint('agentSession.create',sessionId,fields)},...fields};
    save(label+'-intent.json',{runtime:target,sessionId,params});
    const client=await connect(target);let created=false;const observations=[];const summary={label,sessionId};
    try {
      const result=await client.request('agentSession.create',params,30000);save(label+'-create.json',result);
      if(!result.ok||result.result?.ok===false){summary.create_refused=result;return}
      created=true;let page;const deadline=Date.now()+25000;
      while(Date.now()<deadline){const r=await client.request('agentSession.history',{sessionId,direction:'tail',limit:30},15000);
        observations.push(r);save(label+'-before-observations.json',observations);page=r.result?.page;
        const t=page?.latestTurn?.turn;
        if(t?.startedAt&&page.working===true&&['running','working','in_progress'].includes(t.state))break;
        if(t&&['completed','failed','interrupted'].includes(t.state))break;
        await new Promise(resolve=>setTimeout(resolve,400));
      }
      const turn=page?.latestTurn?.turn;summary.before={fence:page?.fence,working:page?.working,turn};
      if(!turn?.startedAt||page.working!==true)throw new Error('P0 active state not established; zero cancel');
      const stopFields={turnId:turn.turnId};
      const cancel={envelope:{sessionId,clientOperationId:mutation.createStructuredAgentSessionOperationId(crypto.randomUUID),expectedRuntimeFence:page.fence,payloadFingerprint:fingerprint('agentSession.cancel',sessionId,stopFields)},...stopFields};
      save(label+'-cancel-intent.json',cancel);
      const cancelled=await client.request('agentSession.cancel',cancel,20000);save(label+'-cancel.json',cancelled);summary.cancel=cancelled;
      const afters=[];const until=Date.now()+10000;
      do{const r=await client.request('agentSession.history',{sessionId,direction:'tail',limit:50},15000);afters.push(r);save(label+'-after-cancel.json',afters);
        if(r.result?.page?.working===false)break;await new Promise(resolve=>setTimeout(resolve,500));}while(Date.now()<until);
      const final=afters.at(-1)?.result?.page;summary.afterCancel={fence:final?.fence,working:final?.working,turn:final?.latestTurn,backgroundTasks:final?.backgroundTasks};
      const requests=await client.request('agentSession.currentRequests',{sessionId,version:crypto.randomUUID()});save(label+'-currentRequests.json',requests);
    } finally {
      if(created){save(label+'-close-intent.json',{sessionId,method:'agentSession.close',noMutationEnvelope:true});
        const closed=await client.request('agentSession.close',{sessionId},25000);save(label+'-close.json',closed);summary.close=closed;
        const after=await client.request('agentSession.history',{sessionId,direction:'tail',limit:50},15000);save(label+'-after-close.json',after);
        summary.afterClose={fence:after.result?.page?.fence,working:after.result?.page?.working};}
      client.close();save(label+'-summary.json',summary);console.log(JSON.stringify(clean(summary)));
    }
  }else if(mode==='negative') {
    const label=target+'-negative';
    if(fs.existsSync(path.join(evidence,label+'-intent.json')))throw new Error('P0 negative already attempted');
    const worktree=JSON.parse(fs.readFileSync(path.join(evidence,target+'-worktree-create.json'),'utf8')).result.worktree;
    const original=JSON.parse(fs.readFileSync(path.join(evidence,target+'-codex-atomic-intent.json'),'utf8'));
    const originalId=original.sessionId;
    const badId='oas_p0_'+crypto.randomUUID().replaceAll('-','');
    const body={kind:'message',role:'user',blocks:[{type:'text',text:'P0 invalid request: this input must be rejected without dispatch.'}]};
    const requests=[
      ['bad-create-fingerprint','agentSession.create',{envelope:{sessionId:badId,clientOperationId:mutation.createStructuredAgentSessionOperationId(crypto.randomUUID),expectedRuntimeFence:null,payloadFingerprint:'0'.repeat(64)},worktree:'id:'+worktree.id,agent:'codex'}],
      ['stale-send-fence','agentSession.send',{envelope:{sessionId:originalId,clientOperationId:mutation.createStructuredAgentSessionOperationId(crypto.randomUUID),expectedRuntimeFence:1,payloadFingerprint:fingerprint('agentSession.send',originalId,{body})},body}],
      ['close-extra-envelope','agentSession.close',{sessionId:originalId,envelope:{expectedRuntimeFence:1}}]
    ];
    save(label+'-intent.json',{requests});
    const client=await connect(target);const results=[];
    try{for(const [name,method,params] of requests){const r=await client.request(method,params,15000);save(label+'-'+name+'.json',r);results.push({name,response:r});}
      const after=await client.request('agentSession.history',{sessionId:originalId,direction:'tail',limit:50});save(label+'-history-after.json',after);
      save(label+'-summary.json',results);console.log(JSON.stringify(clean(results)));
    }finally{client.close()}
  }else if(mode==='negative-cleanup') {
    const label=target+'-negative-cleanup';
    if(fs.existsSync(path.join(evidence,label+'-intent.json')))throw new Error('P0 cleanup already attempted; reconcile only');
    const original=JSON.parse(fs.readFileSync(path.join(evidence,target+'-codex-atomic-intent.json'),'utf8'));
    const receipt=JSON.parse(fs.readFileSync(path.join(evidence,target+'-negative-stale-send-fence.json'),'utf8'));
    if(!receipt.ok||receipt.result?.ok!==true)throw new Error('P0 no accepted negative send to clean');
    const sessionId=original.sessionId;const client=await connect(target);
    try{
      const before=await client.request('agentSession.history',{sessionId,direction:'tail',limit:100});save(label+'-before.json',before);
      save(label+'-intent.json',{sessionId,observedFence:before.result?.page?.fence,reason:'cleaning a new acquisition caused by accepted stale-fence test; not replaying an unknown close'});
      const closed=await client.request('agentSession.close',{sessionId},25000);save(label+'-close.json',closed);
      const after=await client.request('agentSession.history',{sessionId,direction:'tail',limit:100});save(label+'-after.json',after);
      console.log(JSON.stringify({target,sessionId,closed:closed.ok&&closed.result?.ok,afterFence:after.result?.page?.fence,working:after.result?.page?.working}));
    }finally{client.close()}
  }else if(mode==='inventory') {
    const worktree=JSON.parse(fs.readFileSync(path.join(evidence,target+'-worktree-create.json'),'utf8')).result.worktree;
    const client=await connect(target);
    try{const tabs=await client.request('session.tabs.list',{worktree:'id:'+worktree.id});save(target+'-tabs-after.json',tabs);
      console.log(JSON.stringify({target,ok:tabs.ok,worktree:tabs.result?.worktree,tabs:tabs.result?.tabs?.map(t=>({id:t.id,type:t.type,terminal:t.terminal,sessionId:t.sessionId}))}));
      const sub=await connect(target);try{const r=await sub.request('agentSession.subscribeStatus',null,15000);
        if(r.ok){const own=new Set(fs.readdirSync(evidence).filter(n=>n.startsWith(target+'-')&&n.endsWith('-intent.json')).flatMap(n=>{
          const d=JSON.parse(fs.readFileSync(path.join(evidence,n),'utf8'));return d.sessionId?[d.sessionId]:[];}));
          const raw=r.result||{};const filtered={type:raw.type,sessionsTotal:raw.sessions?.length,sessions:raw.sessions?.filter(s=>own.has(s.sessionId))};
          save(target+'-own-status-after.json',{...r,result:filtered});console.log(JSON.stringify(clean({target,ownStatus:filtered})));
        }else save(target+'-own-status-after.json',r);
      }finally{sub.close()}
    }finally{client.close()}
  }else throw new Error('P0 mode not implemented');
}
module.exports={connect,clean,save,caps};
if(require.main===module)main().catch(e=>{console.error(JSON.stringify({p0_error:e.message}));process.exitCode=1});
