import fs from 'node:fs';
import path from 'node:path';
import crypto from 'node:crypto';
import assert from 'node:assert/strict';

// Offline, read-only corroboration of recorded original calls and evidence.
// Runtime query fences are asserted by the real Go driver, not reconstructed
// from missing private fields. Remote stop proves only orca_terminal scope.
const read = (root, file) => fs.readFileSync(path.join(root, file));
const json = (root, file) => JSON.parse(read(root, file).toString('utf8').replace(/^\uFEFF/, ''));
const sha = value => crypto.createHash('sha256').update(value).digest('hex');
const payload = value => typeof value === 'string' ? JSON.parse(Buffer.from(value, 'base64').toString('utf8')) : value;
const results = [];
for (const root of process.argv.slice(2)) {
  const r = json(root, 'native-result.json'), source = json(root, 'result.json');
  const target = json(root, 'target.json'), snapshot = json(root, 'snapshot.json');
  const input = json(root, 'original-input.json'), intent = json(root, 'submit-intent.json');
  const receipt = json(root, 'submit-receipt.json'), stop = json(root, 'stop-receipt.json');
  const logical = read(root, 'original-input.txt'), wire = read(root, 'wire-input.txt');
  assert.equal(source.passed, true); assert.equal(source.exit_code, 0);
  assert.equal(source.source_digest,'c21d915a6d5ce1d03e7b437b94048614781052312279076ac2cb678f2c3ad894'); assert.equal(r.initial_start_confirmed,false);
  assert.equal(r.completed, true); assert.equal(r.cleaned_up, true);
  assert.notEqual(r.emergency_cleanup, true);
  assert.equal(input.Prompt, logical.toString('utf8'));
  assert.equal(input.input_budget.sha256, sha(logical));
  assert.equal(wire.toString('utf8'), input.Prompt.replaceAll('\r\n', '\n').replace(/[\r\n]+$/, ''));
  assert.equal(r.prompt_sha256, sha(logical));
  assert.equal(snapshot.Ref.run_id, r.run_id); assert.equal(snapshot.Ref.attempt_id, r.attempt_id);
  assert.equal(snapshot.Ref.external.runtime_id, target.Runtime);
  assert.deepEqual(snapshot.Ref.external, r.process);
  assert.equal(intent.request.Operation.id, r.operation_id);
  assert.equal(intent.request.Input.turn.Prompt, input.Prompt);
  assert.equal(receipt.operation.id, r.operation_id);
  const calls = fs.readdirSync(root).filter(n => /^actual-cli-\d+\.json$/.test(n)).map(n => json(root, n));
  const sends = calls.filter(c => c.command.Args[0] === 'terminal' && c.command.Args[1] === 'send');
  assert.equal(sends.length, 1);
  const call = sends[0], args = call.command.Args, arg = key => args[args.indexOf(key) + 1];
  assert.ok(args.includes('--enter')); assert.equal(arg('--text'), wire.toString('utf8').replaceAll('\n','\r')); assert.equal(arg('--text').includes('\n'),false);
  assert.equal(arg('--retry-request'), r.request_id); assert.equal(arg('--terminal'), r.process.terminal);
  assert.equal(arg('--environment'), target.Environment);
  assert.equal(call.exit_error, ''); assert.equal(call.context_error, '');
  assert.equal(call.stdout._meta.runtimeId, target.Runtime);
  const send = call.stdout.result.send;
  assert.equal(send.handle, r.process.terminal);
  assert.equal(call.stdout.result.mutation.requestId, r.request_id);
  assert.equal(call.stdout.result.mutation.replayed, false);
  assert.deepEqual(payload(r.submit_receipt).send, send);
  assert.equal(send.accepted, true); assert.equal(send.prompt.requestId, r.request_id);
  assert.equal(send.prompt.provider, 'codex'); assert.equal(send.prompt.observation, 'supported');
  assert.equal(send.prompt.processIncarnation, r.process.incarnation);
  assert.ok(JSON.stringify(send.prompt.stages) === JSON.stringify(['input_accepted']) || JSON.stringify(send.prompt.stages) === JSON.stringify(['input_accepted','turn_started']));
  assert.equal(calls.filter(c => c.command.Args[0] === 'terminal' && c.command.Args[1] === 'read').length, 0);
  assert.equal(r.submit_calls, 1); assert.equal(r.business_inputs, 1);
  for (const field of ['text_only_calls', 'bare_enter_calls', 'screen_reads', 'business_request_mismatches']) assert.equal(r[field], 0);
  const original = r.verified_original_input, query = r.original_query;
  assert.equal(original.Text, wire.toString('utf8'));
  assert.ok(original.Timestamp > original.Baseline);
  assert.equal(original.Baseline, r.ready.mainAgent.stateStartedAt);
  assert.equal(query.state, 'done'); assert.equal(query.mainAgent.state, 'done');
  assert.ok(query.mainAgent.stateStartedAt > original.Baseline);
  assert.equal(query.providerSession.id, r.ready.providerSession.id);
  const pages = json(root, 'original-input-pages.json');
  const users = pages.flatMap(p => p.messages).filter(m => m.role === 'user' && m.timestamp > original.Baseline);
  assert.equal(users.length, 1); assert.equal(users[0].id, original.ID);
  assert.equal(users[0].source, 'transcript'); assert.notEqual(users[0].shortened, true);
  assert.equal(users[0].blocks.length, 1); assert.equal(users[0].blocks[0].text, original.Text);
  const marker = `SR_NATIVE_INPUT_${input.CommandID}`;
  assert.equal(query.lastAssistantMessage, marker); assert.equal(r.final.LastMessage, marker);
  for (const field of ['Started', 'Completed', 'Succeeded', 'FinalVerified']) assert.equal(r.final[field], true);
  assert.equal(r.final.CommandID, input.CommandID);
  assert.equal(stop.disposition, 'confirmed'); assert.equal(stop.state.stopped, true);
  assert.deepEqual(stop.operation.ref.external, r.process);
  assert.equal(stop.operation.run_id, r.run_id); assert.equal(stop.operation.attempt_id, r.attempt_id);
  assert.equal(stop.state.worker_absent, true); assert.equal(payload(stop.operation.receipt).phase, 'confirmed');
  assert.equal(r.stop_error, ''); assert.equal(r.verify_stopped_error, '');
  assert.equal(r.stop_scope, 'orca_terminal');
  for (const name of ['before_census', 'after_census']) {
    const c = r[name]; assert.equal(c.RuntimeID, target.Runtime); assert.equal(c.CallUncertain, false);
    assert.equal(c.ErrorCode, ''); assert.equal(c.Payload.truncated, false);
    assert.equal(c.Payload.totalCount, 0); assert.equal(c.Payload.terminals.length, 0);
    assert.ok(c.Payload.hostScope.hostIds.length); assert.equal(c.Payload.hostScope.omittedHostIds.length, 0);
  }
  const summary = {run_id:r.run_id, attempt_id:r.attempt_id, request_id:r.request_id, operation_id:r.operation_id,
    runtime_id:target.Runtime, terminal:r.process.terminal, incarnation:r.process.incarnation,
    provider_session:query.providerSession.id, user_item:original.ID, logical_bytes:logical.length,
    logical_sha256:sha(logical), actual_submit_sha256:sha(arg('--text')), actual_submit_cr:arg('--text').split('\r').length-1, logical_wire_lf:wire.toString('utf8').split('\n').length-1, wire_bytes:wire.length, wire_sha256:sha(wire), submit_calls:1,
    accepted:true, initial_business_start:r.initial_start_confirmed, host_start_stage:send.prompt.stages.includes('turn_started'), started:true, completed:true, final_sha256:sha(marker), formal_stop_verified:true,
    stop_scope:r.stop_scope, initialization_elapsed_ms:r.initialization_elapsed_ms,
    business_elapsed_ms:Date.parse(r.completed_at)-Date.parse(r.business_started_at),
    source_digest:source.source_digest, test_exe_sha256:source.test_exe_sha256, passed:true};
  results.push(summary);
}
assert.ok(results.length);
for (const key of ['run_id','attempt_id','request_id','operation_id','terminal','incarnation','provider_session','user_item','logical_sha256']) {
  assert.equal(new Set(results.map(r => r[key])).size, results.length, `independent ${key}`);
}
console.log(JSON.stringify({schema:'steerrunner-remote-native-evidence-audit/v1', sessions:results}, null, 2));
