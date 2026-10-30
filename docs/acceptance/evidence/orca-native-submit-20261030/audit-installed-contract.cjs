// Read-only characterization of the named installed bundle, not a Runner/L3 test.
// Does not import/start Orca. Evaluates extracted methods with isolated dependencies.
// Usage: node audit-installed-contract.cjs <orcad-server.js> <new-result.json>
'use strict';
const fs = require('node:fs');
const vm = require('node:vm');
const assert = require('node:assert/strict');
const crypto = require('node:crypto');
const [bundle, output] = process.argv.slice(2);
if (!bundle || !output || fs.existsSync(output)) throw Error('bundle and new result required');
const bytes = fs.readFileSync(bundle);
const source = bytes.toString('utf8');
const sha = x => crypto.createHash('sha256').update(x).digest('hex');
const extracts = {};
function extract(name, start, end, trim = 0) {
  const a = source.indexOf(start), b = source.indexOf(end, a + start.length);
  assert(a >= 0 && b > a, `missing ${name}; do not adapt silently to another bundle`);
  const code = source.slice(a, b - trim);
  extracts[name] = { start_utf16: a, end_utf16: b - trim, sha256: sha(code) };
  return code;
}
const gateCode = extract('renderGate', 'createAgentPromptRenderGate(t,r){', 'waitForTerminal(t,r){');
const writeCode = extract('writePrompt', 'async writeTerminalAgentPrompt(t,r,n,o,i){', 'var $dr,', 4);
const retryCode = extract('resubmit', 'async resubmitHeldAgentPrompt(t,r,n,o){', 'var Udr,', 4);
const matchCode = extract('heldDraftMatch', 'function Fas(e,t,r){', 'function Dwn(');
const observeCode = extract('observeStart', 'async function Uie(e){', 'function JWi(');
const match = vm.runInNewContext(`${matchCode}; Fas`, {
  Has: x => x, // Equivalent only for these fixtures, which contain no ESC.
  tld: /\[Pasted Content (\d+) chars\](?: #\d+)?/g,
  rld: 2, nld: 8, old: 32,
});
function noOutputGate() {
  let now = 0, serial = 0;
  const timers = new Map();
  const method = vm.runInNewContext(`({${gateCode}}).createAgentPromptRenderGate`, {
    Date: { now: () => now },
    setTimeout: (f, delay) => { const id = ++serial; timers.set(id, { f, at: now + delay }); return id; },
    clearTimeout: id => timers.delete(id),
    // Arbitrary positive constants: the assertion concerns timer-only release,
    // not the installed real-time timeout values.
    mWi: 7, pWi: 11, Sln: 'never-observed-marker',
  });
  const state = { ptysById: new Map([['pty', { launchAgent: 'codex' }]]),
    subscribeToTerminalData: () => () => {} };
  const gate = method.call(state, 'pty', 3);
  return { arm: () => gate.arm(), dispose: () => gate.dispose(), wait: async () => {
    for (let n = 0; timers.size && n < 10; n++) {
      const [id, timer] = [...timers].sort((a, b) => a[1].at - b[1].at)[0];
      timers.delete(id); now = timer.at; timer.f();
    }
    await gate.wait();
  } };
}
async function writeProbe(mode) {
  const logical = '原输入\n\nkeep exact bytes';
  const payload = '\x1b[200~' + logical + '\x1b[201~';
  const writes = [], accepted = [];
  let consumed = mode === 'existing-input' ? 'human draft ' : '';
  let gateReleased = false;
  const method = vm.runInNewContext(`({${writeCode}}).writeTerminalAgentPrompt`, {
    Buffer, Error, oy: signal => { if (signal?.aborted) throw Error('aborted'); },
    Las: agent => agent === 'omp', bzt: () => 0, Was: () => 0, YWi: () => 0,
    Hln: agent => agent === 'codex', VYe: '\r',
    gWi: p => p, bln: async () => { throw Error('unexpected sleep'); },
    Uie: async () => { throw Error('agent_prompt_stalled'); },
  });
  const activity = { generation: 1, workingSequence: 0, permissionSequence: 0, explicitWorkingStartedAt: null };
  const host = {
    ptysById: new Map([['pty', { launchAgent: 'codex' }]]),
    assertAgentPromptGeneration() {}, assertAgentPromptPermissionSafe() {}, assertAgentPromptExpectedBinding() {},
    getPtyWriteHostPlatform: () => 'win32', getPtyAgent: () => 'codex',
    getAgentPromptActivity: () => activity,
    getTerminalPromptRequestBinding: () => ({ processIncarnation: 'original-incarnation', generation: 1 }),
    registerAgentPromptRequest() {}, forgetAgentPromptRequest() {},
    // Exact extracted gate, driven by virtual time without any output or input ACK.
    createAgentPromptRenderGate: () => {
      const gate = noOutputGate();
      return { arm: () => gate.arm(), dispose: () => gate.dispose(), wait: async () => { await gate.wait(); gateReleased = true; } };
    },
    // Isolate the first CR. Supplemental CRs are tested with the actual method below.
    resubmitHeldAgentPrompt: async () => 0,
    ptyController: { write: (pty, text) => {
      writes.push(text);
      if (text === payload) {
        if (mode === 'truncated') consumed += logical.slice(0, -3);
        else if (mode === 'changed-byte') consumed += logical.replace('keep', 'weep');
        else if (mode !== 'not-consumed') consumed += logical;
        if (mode === 'concurrent-input') consumed += ' human concurrent input';
      }
      return true; // Transport accepted, independently of the consumer.
    } },
  };
  const result = await method.call(host, 'terminal', 'pty', 1, payload, {
    inputKind: 'driving', acceptQueued: true, requestId: 'original-request',
    promptForSchedule: logical, observationTimeoutMs: 0, onInputAccepted: x => accepted.push(x),
  });
  assert(gateReleased);
  assert.equal(writes.length, 2);
  assert.equal(writes[1], '\r');
  assert.equal(accepted.length, 1);
  assert.equal(accepted[0].accepted, true);
  assert.deepEqual(Array.from(result.prompt.stages), ['input_accepted']);
  assert.equal(result.prompt.requestId, 'original-request');
  assert.notEqual(consumed, logical);
  return { writes: writes.length, submit_cr: 1, accepted: true, consumed_matches: false,
    request_id: result.prompt.requestId, consumed_sha256: sha(consumed), expected_sha256: sha(logical) };
}
async function supplementalEnter() {
  let now = 0, writes = 0;
  const method = vm.runInNewContext(`({${retryCode}}).resubmitHeldAgentPrompt`, {
    Date: { now: () => now }, nr: { codex: { submitRetryDelayMs: 1 } },
    MAa: async (options, delay) => { now += delay; return writes > 0; },
    Fas: match, VYe: '\r',
  });
  const host = { getPtyAgent: () => 'codex', assertAgentPromptGeneration() {},
    readCurrentTerminalWaitScreen: async () => ({ draft: 'original prompt' }),
    ptyController: { write: (pty, text) => { assert.equal(text, '\r'); writes++; return true; } } };
  const result = await method.call(host, 'pty', 1, { inputKind: 'driving' }, {
    prompt: 'original prompt', heldBudgetMs: 100,
  });
  assert.equal(result, 1); assert.equal(writes, 1);
  return { supplemental_cr: writes, source: 'screen-draft + held-retry', virtual_time: true };
}
async function startWithoutText() {
  let reads = 0;
  const observe = vm.runInNewContext(`${observeCode}; Uie`, {
    Date: { now: () => 0 }, pVu() {}, Fln() {}, JWi() {},
    mVu: async () => { throw Error('unexpected wait'); }, pv: 10,
  });
  await observe({ baseline: { workingSequence: 0 },
    readActivity: () => { reads++; return { workingSequence: 1 }; }, timeoutMs: 1 });
  assert.equal(reads, 1);
  return { start_observed: true, prompt_text_or_digest_supplied: false };
}
const cases = {
  'render-timeout-no-consumption-ack': async () => { const gate = noOutputGate(); gate.arm(); await gate.wait(); return { released_without_output: true }; },
  'accepted-before-consumption': () => writeProbe('not-consumed'),
  'transport-accepted-truncation': () => writeProbe('truncated'),
  'transport-accepted-byte-change': () => writeProbe('changed-byte'),
  'existing-composer-input': () => writeProbe('existing-input'),
  'concurrent-writer-interleaving': () => writeProbe('concurrent-input'),
  'screen-held-supplemental-enter': supplementalEnter,
  'start-observer-does-not-verify-text': startWithoutText,
};
(async () => {
  const results = [];
  for (const [id, run] of Object.entries(cases)) {
    let failures = 0, sample;
    for (let n = 0; n < 30; n++) {
      try { sample = await run(); } catch (error) { failures++; sample = { error: String(error) }; }
    }
    results.push({ id, executed: 30, failed: failures, skipped: 0, sample });
  }
  const result = { schema: 'orca-installed-submit-characterization/v1',
    audit_script_sha256: sha(fs.readFileSync(__filename)),
    bundle_basename: require('node:path').basename(bundle), bundle_sha256: sha(bytes), extracts,
    scope: 'isolated installed methods; virtual timing and dependency fixtures; no live terminal or historical reproduction',
    excluded: ['Runner L3 qualification', 'live native reproduction', 'remote bundle identity', 'provider correctness'],
    cases: results, executed: results.reduce((n, x) => n + x.executed, 0),
    failed: results.reduce((n, x) => n + x.failed, 0), skipped: 0 };
  fs.writeFileSync(output, JSON.stringify(result, null, 2) + '\n', { flag: 'wx' });
  process.stdout.write(JSON.stringify({ executed: result.executed, failed: result.failed, skipped: result.skipped }) + '\n');
  if (result.failed) process.exitCode = 1;
})().catch(error => { process.stderr.write(String(error)); process.exitCode = 1; });
