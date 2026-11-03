// Acceptance reader only: public Orca IPC, read-only status/history/item/options.
// Credentials stay in memory and never enter evidence. No provider connection.
const fs=require('node:fs'),path=require('node:path'),net=require('node:net'),crypto=require('node:crypto');
const [label,sessionId]=process.argv.slice(2);
if(!/^[a-z0-9-]+$/.test(label||'')||!/^oas_[a-z0-9]+$/.test(sessionId||''))throw Error('explicit original label/session required');
const expected=JSON.parse(fs.readFileSync(path.join(__dirname,'local-status.json'),'utf8'))._meta.runtimeId;
const meta=JSON.parse(fs.readFileSync(path.join(process.env.ORCA_USER_DATA_PATH||path.join(process.env.APPDATA,'orca'),'orca-runtime.json'),'utf8'));
if(meta.runtimeId!==expected)throw Error('original runtime changed');
const endpoint=(meta.transports||[meta.transport]).find(t=>t.kind==='named-pipe')?.endpoint;
if(!endpoint)throw Error('original pipe unavailable');
const capabilities=['agent-session.structured.v1','agent-session.host-authority.v1','agent-session.session-boundary.v1','agentSession.create.message.v1','agent-session.send-selection.v1','agent-session.permission-modes.v1','agent-session.permission-own.v1','agent-session.turn-item.v1','agent-session.item-detail.v1','agent-session.current-requests.v1'];
const redact=v=>Array.isArray(v)?v.map(redact):v&&typeof v==='object'?Object.fromEntries(Object.entries(v).filter(([k])=>!/authToken|deviceToken|secret|password|apiKey|accessToken|refreshToken|authorization|launchEnv|accountHome/i.test(k)).map(([k,v])=>[k,redact(v)])):v;
const save=(name,value)=>fs.writeFileSync(path.join(__dirname,label+'-'+name+'.json'),JSON.stringify(redact(value),null,2)+'\n');
async function main(){
 const socket=net.connect(endpoint);socket.setEncoding('utf8');let pending='',waiter;
 socket.on('data',chunk=>{pending+=chunk;if(Buffer.byteLength(pending)>4<<20){waiter?.reject(Error('frame limit'));socket.destroy();return}let at;while((at=pending.indexOf('\n'))>=0){const line=pending.slice(0,at);pending=pending.slice(at+1);if(!line.trim())continue;const value=JSON.parse(line);if(value._keepalive)continue;waiter?.resolve(value)}});
 socket.on('error',err=>waiter?.reject(Error(err.code||'pipe failure')));
 await new Promise((resolve,reject)=>{socket.once('connect',resolve);socket.once('error',reject)});
 async function request(method,params){
  const id=crypto.randomUUID();let timer;
  const response=new Promise((resolve,reject)=>{timer=setTimeout(()=>reject(Error('read deadline; no mutation')),20000);waiter={resolve,reject}});
  socket.write(JSON.stringify({id,method,params,authToken:meta.authToken,clientCapabilities:capabilities})+'\n');
  try{const r=await response;if(r.id!==id||r._meta?.runtimeId!==expected||!r.ok)throw Error('public response identity/refusal');return r}finally{clearTimeout(timer);waiter=null}
 }
 try{
  save('status',await request('status.get',null));
  const pages=[];let cursor,bytes=0;const items=new Map();
  for(let i=0;i<64;i++){
   const r=await request('agentSession.history',{sessionId,direction:cursor?'before':'tail',...(cursor?{cursor}:{}),limit:200});save('history-page-'+i,r);
   if(!r.result.ok||r.result.page.sessionId!==sessionId)throw Error('original history refused');
   const page=r.result.page;pages.push(page);if(page.hasNewer||pages[0].epoch!==page.epoch)throw Error('history gap');
   for(let item of page.items){if(item.shortened){const detail=await request('agentSession.item',{sessionId,itemId:item.itemId});save('detail-'+items.size,detail);if(!detail.result.item||detail.result.item.shortened)throw Error('incomplete item');item=detail.result.item}bytes+=Buffer.byteLength(JSON.stringify(item));if(bytes>8<<20)throw Error('history byte limit');items.set(item.itemId,item)}
   if(!page.hasOlder)break;
   if(!page.window?.nextCursor||cursor&&page.window.nextCursor.sequence>=cursor.sequence)throw Error('cursor failed');cursor=page.window.nextCursor;
   if(i===63)throw Error('page limit');
  }
  const options=await request('agentSession.options',{sessionId});save('options',options);
  const ordered=[...items.values()].sort((a,b)=>a.sequence-b.sequence||(a.sequenceIndex||0)-(b.sequenceIndex||0));
  const users=ordered.filter(i=>i.body.kind==='message'&&i.body.role==='user'),assistants=ordered.filter(i=>i.body.kind==='message'&&i.body.role==='assistant');
  save('summary',{sessionId,epoch:pages[0].epoch,working:pages[0].working,latestTurn:pages[0].latestTurn,userCount:users.length,userBodySHA256:users.map(i=>crypto.createHash('sha256').update(i.body.blocks.map(b=>b.text||'').join('')).digest('hex')),submissions:pages[0].submissions,assistantItems:assistants.map(i=>({itemId:i.itemId,sequence:i.sequence,turnScope:i.turnScope,characters:i.body.blocks.map(b=>b.text||'').join('').length})),toolItems:ordered.filter(i=>i.body.kind==='tool-call').length,options:options.result.current});
  process.stdout.write(JSON.stringify({sessionId,working:pages[0].working,state:pages[0].latestTurn?.turn?.state,userCount:users.length,assistantCount:assistants.length,tools:ordered.filter(i=>i.body.kind==='tool-call').length})+'\n');
 }finally{socket.destroy()}
}
main().catch(err=>{process.stderr.write(String(err.message)+'\n');process.exitCode=1});
