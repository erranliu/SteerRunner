import fs from 'node:fs';
import path from 'node:path';
import crypto from 'node:crypto';
import assert from 'node:assert/strict';

const sha = value => crypto.createHash('sha256').update(value).digest('hex');
const read = file => JSON.parse(fs.readFileSync(file, 'utf8'));
const arg = (args, key) => { const i = args.indexOf(key); return i < 0 ? undefined : args[i + 1]; };
function bodyBytes(text) {
  const start = text.indexOf('{', text.indexOf('"body"') + 6);
  assert(start >= 0, 'signed body absent');
  let depth = 0, quoted = false, escape = false;
  for (let i = start; i < text.length; i++) {
    const c = text[i];
    if (quoted) { if (escape) escape = false; else if (c === '\\') escape = true; else if (c === '"') quoted = false; }
    else if (c === '"') quoted = true;
    else if (c === '{') depth++;
    else if (c === '}' && --depth === 0) return Buffer.from(text.slice(start, i + 1), 'utf8');
  }
  throw new Error('signed body incomplete');
}
for (const root of process.argv.slice(2)) {
  const result = read(path.join(root, 'result.json'));
  const native = read(path.join(root, 'native-result.json'));
  const input = read(path.join(root, 'original-input.json'));
  const snapshot = read(path.join(root, 'snapshot.json'));
  const state = snapshot.Implementation.find(x => x.implementation_key === 'orca.terminal-state').payload;
  const ready = state.remote_readiness;
  const turn = state.turns.find(x => x.command === input.CommandID);
  const recorded = turn.recorded;
  const wire = fs.readFileSync(path.join(root, 'wire-input.txt'), 'utf8');
  assert(result.passed && result.exit_code === 0 && native.completed && native.cleaned_up);
  assert.equal(native.submit_calls, 1);
  for (const key of ['text_only_calls','bare_enter_calls','screen_reads','business_request_mismatches']) assert.equal(native[key], 0, key);
  assert.equal(native.business_inputs, 1);
  assert.equal(native.logical_sha256, sha(input.Prompt));
  assert.equal(input.input_budget.sha256, sha(input.Prompt));
  assert.equal(recorded.input_digest, sha(wire));
  assert(recorded.started && recorded.completed && recorded.status === 'completed' && recorded.reader_writers_absent);
  assert.equal(recorded.model, input.model);
  assert.equal(recorded.reasoning_effort, input.reasoning_effort);
  assert.equal(recorded.final_text, `SR_LOCAL_NATIVE_${input.CommandID}`);
  assert.equal(recorded.final_digest, sha(recorded.final_text));
  assert(native.final.FinalVerified && native.final.Started && native.final.Completed);
  assert.equal(native.final.FinalNativeSessionID, ready.session_id);
  assert.equal(native.final.FinalNativeTurnID, recorded.native_turn_id);
  assert.equal(native.final.LastMessage.trim(), recorded.final_text);
  const [header, ...lines] = wire.split('\n');
  assert(header.startsWith('[SteerRunner model=') && header.endsWith(']'));
  const model = JSON.parse(Buffer.from(header.slice('[SteerRunner model='.length, -1), 'base64url'));
  assert.equal(model.command, input.CommandID);
  assert.equal(model.input_digest, turn.input_digest);
  assert.equal(model.model, input.model);
  assert.equal(model.reasoning_effort, input.reasoning_effort);
  assert.equal(model.body_digest, sha(lines.join('\n')));
  assert.equal(lines.join('\n'), input.Prompt.replaceAll('\r\n','\n').replace(/[\r\n]+$/u,''));
  const calls = fs.readdirSync(root).filter(x => /^actual-cli-\d+\.json$/u.test(x)).sort().map(x => read(path.join(root,x)));
  const business = calls.filter(x => x.command.Args[0] === 'terminal' && x.command.Args[1] === 'send' && arg(x.command.Args,'--retry-request') === native.request_id);
  assert.equal(business.length, 1);
  const send = business[0];
  assert.equal(arg(send.command.Args,'--terminal'), native.process.terminal);
  assert(send.command.Args.includes('--enter'));
  assert.equal(arg(send.command.Args,'--text'), wire);
  assert.equal(send.stdout._meta.runtimeId, native.process.runtime_id);
  assert.equal(send.stdout.result.send.prompt.requestId, native.request_id);
  assert.equal(send.stdout.result.send.prompt.processIncarnation, native.process.incarnation);
  assert.equal(send.stdout.result.send.prompt.provider, 'codex');
  assert.equal(send.stdout.result.send.prompt.observation, 'supported');
  assert(send.stdout.result.send.prompt.stages.includes('input_accepted'));
  const afterSubmit = calls.slice(calls.indexOf(send)).filter(x => arg(x.command.Args,'--terminal') === native.process.terminal);
  assert.equal(afterSubmit.filter(x => x.command.Args[1] === 'send').length, 1);
  assert.equal(afterSubmit.filter(x => x.command.Args[1] === 'read').length, 0);
  // The original workspace export was collected before owned-resource cleanup.
  // Reverification uses only that immutable public artifact, never a live Run.
  const rawExit = fs.readFileSync(path.join(root,'signed-exit-original.json'));
  assert(rawExit.length <= 65536);
  const signed = JSON.parse(rawExit.toString('utf8'));
  if (native.signed_exit != null) assert.deepEqual(native.signed_exit,signed);
  const key = crypto.createPublicKey({key:Buffer.concat([Buffer.from('302a300506032b6570032100','hex'),Buffer.from(ready.exit_public_key,'base64')]),format:'der',type:'spki'});
  assert(crypto.verify(null,bodyBytes(rawExit.toString('utf8')),key,Buffer.from(signed.signature,'base64')),'original exit signature');
  assert.deepEqual(signed.body.readiness, ready);
  assert(signed.body.writers_absent && signed.body.authentication_closed);
  assert(Date.parse(signed.body.closed_at) >= Date.parse(ready.checked_at));
  assert(native.stop_receipt.state.stopped && native.stop_receipt.state.worker_absent && native.stop_receipt.disposition === 'confirmed');
  assert(native.authentication_stop.revocation_confirmed && native.authentication_stop.broker_closed && native.authentication_stop.socket_cleanup_confirmed);
  assert.deepEqual(native.authentication_stop.readiness, ready);
  assert(native.control_closure.closed);
  const census = native.after_census;
  assert.equal(census.RuntimeID,native.process.runtime_id);
  assert(!census.CallUncertain && census.ErrorCode === '' && census.Payload.totalCount === 0 && !census.Payload.truncated);
  assert(census.Payload.hostScope.hostIds.includes('local') && !census.Payload.hostScope.omittedHostIds.includes('local'));
  const audit = {schema:'steerrunner-native-evidence-audit/v1',run_id:native.run_id,request_id:native.request_id,operation_id:native.operation_id,runtime_id:native.process.runtime_id,terminal:native.process.terminal,incarnation:native.process.incarnation,provider_session:ready.session_id,native_turn:recorded.native_turn_id,user_item:recorded.user_item_id,logical_bytes:Buffer.byteLength(input.Prompt),logical_sha256:sha(input.Prompt),wire_bytes:Buffer.byteLength(wire),wire_sha256:sha(wire),submit_calls:1,accepted:true,started_receipt:send.stdout.result.send.prompt.stages.includes('turn_started'),started_history:true,completed:true,final_sha256:recorded.final_digest,model:recorded.model,reasoning_effort:recorded.reasoning_effort,original_exit_signature_verified:true,exit_sha256:sha(rawExit),formal_stop_verified:true,authentication_closed:true,control_closed:true,original_host_census_empty:true,passed:true};
  assert.deepEqual(read(path.join(root,'evidence-audit.json')),audit,'stored audit differs from original evidence');
  process.stdout.write(JSON.stringify(audit)+'\n');
}
