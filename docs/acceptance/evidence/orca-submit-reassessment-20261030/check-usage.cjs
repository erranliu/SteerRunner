// Isolated execution of installed CLI handlers/pure helpers and mutation runner.
// No Orca startup, model, network, real PTY, or production DB is used here.
'use strict';
const fs = require('node:fs'), vm = require('node:vm'), crypto = require('node:crypto');
const assert = require('node:assert/strict');
const root = 'C:/Users/lmas1/AppData/Local/Programs/orca/resources/';
const out = root + 'app.asar.unpacked/out/';
const sha = x => crypto.createHash('sha256').update(x).digest('hex');
const bundle = fs.readFileSync(root + 'orcad-template/orcad-server.js', 'utf8');
const pure = require(out + 'shared/agent-prompt-injection.js');
const contract = require(out + 'shared/orchestration-rpc-contract.js');
const format = require(out + 'cli/terminal-format.js');
class RpcError extends Error { constructor(code, message) { super(message); this.code = code; } }
const provenance = {};
function between(name, start, end) {
  const a = bundle.indexOf(start), b = bundle.indexOf(end, a + start.length);
  assert(a >= 0 && b > a, name);
  const code = bundle.slice(a, b); provenance[name] = { start_utf16: a, end_utf16: b, sha256: sha(code) }; return code;
}
function cliHandler() {
  const file = out + 'cli/handlers/terminal-send.js', code = fs.readFileSync(file, 'utf8');
  provenance.cli_handler_sha256 = sha(code);
  const exports = {};
  vm.runInNewContext(code, { exports, process: { exitCode: 0 }, require: name => {
    if (name.endsWith('protocol-version')) return { TERMINAL_PROMPT_DELIVERY_RUNTIME_CAPABILITY: 'terminal.prompt-delivery.v1' };
    if (name.endsWith('/flags')) return { getOptionalStringFlag: (f, k) => f.get(k), getOptionalPositiveIntegerFlag: (f, k) => f.get(k) ? Number(f.get(k)) : undefined };
    if (name.endsWith('retry-request-flag')) return { readRetryRequestFlag: f => f.get('retry-request') };
    if (name.endsWith('runtime-client')) return { RuntimeClientError: RpcError };
    if (name.endsWith('terminal-prompt-mutation-recovery')) return { attachUnverifiedTerminalPromptRecovery: e => e };
    if (name.endsWith('/selectors')) return { getTerminalHandle: async f => f.get('terminal') };
    if (name.endsWith('/format')) return { printResult() {}, terminalSendWarnings: format.terminalSendWarnings };
    throw Error(name);
  } }); return exports.terminalSendHandler;
}
async function cli(flags, capable = true) {
  let preflight = 0; const calls = [], handler = cliHandler();
  const client = { getCliStatus: async () => { preflight++; return { result: { runtime: { reachable: true, runtimeId: 'runtime', capabilities: capable ? ['terminal.prompt-delivery.v1'] : [] } } }; },
    call: async (method, params, options) => {
      calls.push({ method, params, options });
      return { result: { send: { handle: params.terminal, accepted: true, prompt: params.agentPrompt ? { requestId: 'original', stages: ['input_accepted'], provider: 'codex', observation: 'supported' } : undefined } } };
    } };
  let error; try { await handler({ flags: new Map([['terminal', 'terminal'], ...Object.entries(flags)]), client, cwd: '.', json: true }); } catch (e) { error = e; }
  return { preflight, calls, error };
}
const runCode = between('durable_mutation_runner', 'async run(t,r,n,o,i){', 'readTerminalPromptBindingHash(t){');
const helperCode = between('payload_and_replay_helpers', 'function Oqt(', 'function QSn(');
const canonicalCode = between('canonical_json', 'function JSn(', 'function mos(');
const receiptCode = between('receipt_metadata', 'function CM(', 'function pos(');
const observationCode = between('replay_observation', 'async function VKa(', 'function JKa(');
const context = {
  Error, U: RpcError, aos: crypto, Qid: '__orcaCallerOrcaSessionId',
  fe: () => null, mYe: () => undefined, ZSn: () => undefined,
  tTe: contract.isDurableMutation, eTe: contract.isTerminalPromptMutation,
  dos: () => false, pos: () => null, YSn: 'unused-effect-free', zfp: new Set(['terminal_handle_stale']),
};
vm.createContext(context);
vm.runInContext(helperCode + canonicalCode + receiptCode + observationCode + `; this.runner=({${runCode}}).run`, context);
function journalFixture() {
  const rows = new Map(); let writes = 0, observations = 0, incarnation = 'original';
  const db = {
    getMutationReceipt: (caller, id) => rows.get(caller + ':' + id),
    beginMutationReceipt: e => {
      const k = e.callerFingerprint + ':' + e.requestId;
      if (!rows.has(k)) { rows.set(k, { state: 'pending', method: e.method, payload_hash: e.payloadHash }); return { disposition: 'started', row: rows.get(k) }; }
      return { disposition: rows.get(k).state, row: rows.get(k) };
    },
    completeMutationReceipt: e => { const r = rows.get(e.callerFingerprint + ':' + e.requestId); Object.assign(r, { state: 'completed', receipt: e.receipt }); return r; },
    discardPendingMutationReceipt: (caller, id) => rows.delete(caller + ':' + id),
  };
  const runtime = { getOrchestrationDb: () => db, getTerminalPaneKey: () => 'original-pane',
    getTerminalPromptRequestBinding: () => ({ processIncarnation: incarnation }),
    observeTerminalAgentPrompt: async (handle, receipt) => { observations++; return { ...receipt, stages: ['input_accepted', 'turn_started'] }; } };
  const owner = () => ({ runtime, inFlight: new Map(), getLocalAuthenticatedCallerFingerprint: () => 'same-caller',
    readTerminalPromptBindingHash: () => sha(JSON.stringify(runtime.getTerminalPromptRequestBinding())) });
  let current = owner();
  const params = { terminal: 'terminal', text: '原文\n\nunchanged', enter: true, agentPrompt: true, client: { type: 'desktop' } };
  const call = async ({ text, wait, effectLost = false } = {}) => {
    const p = { ...params, ...(text === undefined ? {} : { text }), ...(wait === undefined ? {} : { waitSubmitMs: wait }) };
    return context.runner.call(current, { method: 'terminal.send', orchestrationRequestId: 'original-request' }, p, async scope => {
      if (scope.replayedReceipt) return await context.VKa(runtime, p.terminal, scope.replayedReceipt, p.waitSubmitMs);
      scope.markEffectPossible(); writes++;
      if (effectLost) throw Error('effect happened; receipt not recorded');
      return { send: { handle: p.terminal, accepted: true, prompt: { requestId: 'original-request', stages: ['input_accepted'], observation: 'supported', processIncarnation: incarnation } } };
    });
  };
  return { call, restart: () => { current = owner(); }, replace: () => { incarnation = 'replacement'; }, prune: () => rows.clear(),
    counts: () => ({ writes, observations, rows: rows.size }) };
}
const results = [];
async function check(id, f) { try { const evidence = await f(); results.push({ id, passed: true, evidence }); } catch (e) { results.push({ id, passed: false, error: String(e) }); } }
(async () => {
  await check('text-plus-enter-selects-durable-prompt', async () => {
    const x = await cli({ text: '中文\n\nbody', enter: true, 'retry-request': 'original-request', 'wait-submit': '20' });
    assert(!x.error); assert.equal(x.preflight, 1); assert(contract.isDurableMutation('terminal.send', x.calls[0].params));
    assert.equal(x.calls[0].options.orchestrationRequestId, 'original-request'); assert.equal(x.calls[0].params.waitSubmitMs, 20000);
    return { preflight: x.preflight, durable: true, exact_text: x.calls[0].params.text };
  });
  for (const [id, flags] of [['raw-paste-bypasses-prompt', { text: '\x1b[200~body\ntext\x1b[201~' }], ['bare-enter-bypasses-prompt', { enter: true }]]) {
    await check(id, async () => { const x = await cli(flags); assert(!x.error); assert.equal(x.preflight, 0); assert(!contract.isDurableMutation('terminal.send', x.calls[0].params)); return { preflight: 0, durable: false }; });
  }
  await check('default-does-not-request-start-observation', async () => { const x = await cli({ text: 'body', enter: true }); assert.equal(x.calls[0].params.waitSubmitMs, undefined); return { wait_submit_ms: null }; });
  await check('old-host-retry-refused-before-dispatch', async () => { const x = await cli({ text: 'body', enter: true, 'retry-request': 'original-request' }, false); assert.equal(x.error.code, 'incompatible_runtime'); assert.equal(x.calls.length, 0); return { calls: 0 }; });
  await check('accepted-without-start-has-warning', async () => { const w = format.terminalSendWarnings({ accepted: true, prompt: { provider: 'codex', observation: 'supported', requestId: 'original', stages: ['input_accepted'] } }); assert.equal(w.length, 1); return { warning_present: true }; });
  await check('generic-framing-requires-plain-text', async () => { const plain = pure.buildAgentPromptPasteBytes('A\nB'); const nested = pure.buildAgentPromptPasteBytes(plain); assert(nested.includes('<ESC>')); assert.notEqual(plain, nested); return { preframed_text_would_change: true }; });
  await check('completed-replay-three-times-no-new-write', async () => { const f = journalFixture(); await f.call(); for (let i = 0; i < 3; i++) { const r = await f.call(); assert(r.mutation.replayed); } assert.equal(f.counts().writes, 1); return f.counts(); });
  await check('completed-replay-with-wait-only-observes', async () => { const f = journalFixture(); await f.call(); const r = await f.call({ wait: 1000 }); assert(r.send.prompt.stages.includes('turn_started')); assert.equal(f.counts().writes, 1); assert.equal(f.counts().observations, 1); return f.counts(); });
  await check('pending-after-restart-refuses-resend', async () => { const f = journalFixture(); await assert.rejects(f.call({ effectLost: true })); f.restart(); await assert.rejects(f.call(), e => e.code === 'operation_unknown'); assert.equal(f.counts().writes, 1); return f.counts(); });
  await check('changed-payload-rejected', async () => { const f = journalFixture(); await f.call(); await assert.rejects(f.call({ text: 'changed' }), e => e.code === 'request_mismatch'); assert.equal(f.counts().writes, 1); return f.counts(); });
  await check('replacement-incarnation-replays-replacement-warning', async () => { const f = journalFixture(); await f.call(); f.replace(); const r = await f.call({ wait: 1000 }); assert.equal(r.send.prompt.observation, 'incarnation_replaced'); assert.equal(f.counts().writes, 1); return f.counts(); });
  await check('pruned-receipt-does-not-preserve-deduplication', async () => { const f = journalFixture(); await f.call(); f.prune(); await f.call(); assert.equal(f.counts().writes, 2); return { ...f.counts(), note: 'memory fixture deliberately removes retained receipt; absent must not authorize replay' }; });
  const result = { scope: 'installed CLI handlers and mutation runner with isolated RPC/DB/PTY dependencies; not live prompt delivery or historical reproduction',
    bundle_sha256: sha(fs.readFileSync(root + 'orcad-template/orcad-server.js')), script_sha256: sha(fs.readFileSync(__filename)), provenance,
    executed: results.length, failed: results.filter(x => !x.passed).length, skipped: 0, results };
  const target = process.argv[2]; if (!target) throw Error('new result path required'); fs.writeFileSync(target, JSON.stringify(result, null, 2) + '\n', { flag: 'wx' });
  console.log(JSON.stringify({ executed: result.executed, failed: result.failed, skipped: result.skipped })); if (result.failed) process.exitCode = 1;
})();
