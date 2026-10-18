import { test } from "node:test";
import { EventEmitter } from "node:events";
import { PassThrough } from "node:stream";
import { createTerminalToolOperations } from "./terminal-tools.mjs";
import assert from "node:assert/strict";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { createHash, randomUUID } from "node:crypto";
import { attachEvidence, writeReceipt } from "./pi-evidence.mjs";

const hash = (value) => createHash("sha256").update(value).digest("hex");
function fixture(t, overrides = {}, options = {}) {
  const directory = fs.mkdtempSync(path.join(os.tmpdir(), "runner-pi-evidence-test-"));
  t.after(() => {
    const resolved = fs.realpathSync(directory);
    assert.equal(resolved.toLowerCase(), path.resolve(directory).toLowerCase());
    assert.equal(path.dirname(resolved), fs.realpathSync(os.tmpdir()));
    assert.ok(path.basename(resolved).startsWith("runner-pi-evidence-test-"));
    fs.rmSync(resolved, { recursive: true });
  });
  const configFile = path.join(directory, "config.json");
  const config = { version: 1, directory, cwd: directory, run_id: "a".repeat(32), session_id: randomUUID() };
  fs.writeFileSync(configFile, JSON.stringify(config));
  const handlers = new Map(), branch = [];
  let tools = options.tools ?? [];
  let thinking = "low";
  const pi = { registerFlag() {}, getFlag: () => configFile, getActiveTools: () => tools, getThinkingLevel: () => thinking,
    on: (name, fn) => handlers.set(name, fn) };
  const ctx = { mode: "tui", cwd: directory, model: { provider: "test", id: "test" }, hasPendingMessages: () => false,
    sessionManager: { getSessionId: () => config.session_id, getBranch: () => branch }, ...overrides };
  const gate = attachEvidence(pi, options);
  const emit = (name, value = {}) => handlers.get(name)?.(value, ctx);
  const read = (name) => JSON.parse(fs.readFileSync(path.join(directory, name + ".json"), "utf8"));
  const wire = (id = "b".repeat(32), body = "fixed probe") => `[SteerRunner command=${id} input=${hash(body)}]\n${body}`;
  const input = (value = wire()) => emit("input", { text: value, source: "interactive" });
  const commit = (value = wire()) => {
    emit("before_agent_start", { prompt: value });
    const user = { type: "message", id: "native-user", message: { role: "user", content: [{ type: "text", text: value }] } };
    emit("message_start", { message: user.message });
    branch.push(user);
  };
  const end = (body = "fixed reply", outcome = "completed", stopReason = "stop") => {
    branch.push({ type: "message", id: "native-reply", message: { role: "assistant", content: [{ type: "text", text: body }], stopReason } });
    emit("agent_before_settle", { outcome });
    emit("agent_settled");
  };
  return { gate, config, configFile, emit, read, wire, input, commit, end, directory, branch, ctx, setThinking: (value) => { thinking = value; }, setTools: (value) => { tools = value; } };
}

test("an observed input or agent_end cannot certify start or completion", (t) => {
  const f = fixture(t); f.emit("session_start");
  assert.equal(f.read("readiness").mode, "tui");
  f.input();
  assert.equal(f.read("b".repeat(32)).stage, "input_observed");
  f.commit(); f.emit("agent_end");
  assert.equal(f.read("b".repeat(32)).stage, "started");
  f.end();
  const final = f.read("b".repeat(32));
  assert.equal(final.stage, "settled");
  assert.equal(final.native_input_id, "native-user");
  assert.equal(final.reply_sha256, hash("fixed reply"));
});

test("same prompt with distinct command IDs retains separate native inputs", (t) => {
  const f = fixture(t); f.emit("session_start"); f.input(); f.commit(); f.end();
  const second = f.wire("c".repeat(32));
  f.input(second); f.commit(second); f.branch.at(-1).id = "second-native-user"; f.end();
  assert.equal(f.read("c".repeat(32)).native_input_id, "second-native-user");
  assert.equal(f.read("b".repeat(32)).native_input_id, "native-user");
});

test("duplicate, manual and altered input are fenced without a second execution", (t) => {
  for (const unexpected of ["manual input", `[SteerRunner command=${"c".repeat(32)} input=${"0".repeat(64)}]\nchanged`]) {
    const f = fixture(t); f.emit("session_start"); f.input(); f.commit(); f.end();
    const frozen = fs.readFileSync(path.join(f.directory, "b".repeat(32) + ".json"));
    assert.equal(f.input(unexpected).action, "handled");
    assert.equal(f.read("readiness").stage, "unverified");
    assert.deepEqual(fs.readFileSync(path.join(f.directory, "b".repeat(32) + ".json")), frozen);
  }
  const f = fixture(t); f.emit("session_start"); f.input(); f.commit(); f.end();
  assert.equal(f.input().action, "handled");
});

test("non-TUI or enabled tools cannot become ready", (t) => {
  const rpc = fixture(t, { mode: "rpc" });
  assert.throws(() => rpc.emit("session_start"), /probe_binding_mismatch/);
  const tools = fixture(t); tools.setTools(["bash"]);
  assert.throws(() => tools.emit("session_start"), /probe_binding_mismatch/);
});

test("a changed expanded or committed input cannot settle successfully", (t) => {
  for (const point of ["before_agent_start", "message_start"]) {
    const f = fixture(t); f.emit("session_start"); f.input();
    f.emit(point, point === "message_start" ? { message: { role: "user", content: "changed" } } : { prompt: "changed" });
    assert.equal(f.read("readiness").stage, "unverified");
    f.end();
    assert.equal(f.read("b".repeat(32)).stage, "unverified");
  }
});

test("retry outcomes are observed until the real settled boundary", (t) => {
  const f = fixture(t); f.emit("session_start"); f.input(); f.commit();
  f.emit("agent_before_settle", { outcome: "error" });
  assert.equal(f.read("b".repeat(32)).stage, "started");
  f.end("after retry", "completed");
  assert.equal(f.read("b".repeat(32)).outcome, "completed");
  assert.equal(f.read("b".repeat(32)).reply, "after retry");
});

test("a provider error remains a settled failure, never a successful reply", (t) => {
  const f = fixture(t); f.emit("session_start"); f.input(); f.commit(); f.end("", "error", "error");
  assert.equal(f.read("b".repeat(32)).outcome, "error");
  assert.equal(f.read("b".repeat(32)).stop_reason, "error");
});

test("missing native entries, pending continuations and oversized replies remain unverified", (t) => {
  const missing = fixture(t); missing.emit("session_start"); missing.input(); missing.commit(); missing.branch.length = 0; missing.end();
  assert.equal(missing.read("readiness").failure, "native_entries_missing_or_ambiguous");
  const queued = fixture(t, { hasPendingMessages: () => true }); queued.emit("session_start"); queued.input(); queued.commit(); queued.end();
  assert.equal(queued.read("readiness").failure, "settlement_binding_mismatch");
  const large = fixture(t); large.emit("session_start"); large.input(); large.commit(); large.end("a".repeat(32769));
  assert.equal(large.read("readiness").failure, "reply_too_large");
});

test("writer failures fence input and incomplete temporary files are never a final receipt", (t) => {
  const f = fixture(t); f.emit("session_start");
  fs.writeFileSync(path.join(f.directory, "b".repeat(32) + ".json.tmp"), "interrupted write");
  assert.equal(f.input().action, "handled");
  assert.equal(fs.existsSync(path.join(f.directory, "b".repeat(32) + ".json")), false);
  assert.throws(() => writeReceipt(f.directory, "too-large", { data: "a".repeat(65536) }), /receipt_too_large/);
});

test("native shell shortcut is replaced even though no built-in tools are active", (t) => {
  const f = fixture(t); f.emit("session_start");
  const result = f.emit("user_bash", { command: "unexpected" });
  assert.equal(result.result.exitCode, 1);
  assert.equal(f.read("readiness").failure, "unexpected_user_shell");
});

test("the fixed-text probe caps the native payload without changing the provider or model", (t) => {
  const f = fixture(t); f.emit("session_start");
  const payload = { model: "original", max_tokens: 943222, messages: [] };
  const bounded = f.emit("before_provider_request", { payload });
  assert.equal(bounded.model, "original");
  assert.equal(bounded.max_tokens, 256);
  assert.equal(payload.max_tokens, 943222);
});

test("shutdown or identity change cannot rewrite a frozen result or certify a new session", (t) => {
  const f = fixture(t); f.emit("session_start"); f.input(); f.commit(); f.end();
  const target = path.join(f.directory, "b".repeat(32) + ".json");
  const frozen = fs.readFileSync(target);
  f.emit("session_shutdown", { reason: "resume" });
  assert.deepEqual(fs.readFileSync(target), frozen);
  assert.equal(f.read("readiness").stage, "unverified");
  assert.equal(f.input(f.wire("c".repeat(32))).action, "handled");
  const changed = fixture(t); changed.emit("session_start"); changed.input(); changed.commit();
  changed.ctx.sessionManager.getSessionId = () => randomUUID(); changed.end();
  assert.equal(changed.read("readiness").failure, "settlement_binding_mismatch");
  const model = fixture(t); model.emit("session_start"); model.input(); model.commit();
  model.ctx.model = { provider: "changed", id: "changed" }; model.end();
  assert.equal(model.read("readiness").failure, "settlement_binding_mismatch");
});

test("reply boundaries count UTF-8 bytes and never truncate", (t) => {
  const exact = fixture(t); exact.emit("session_start"); exact.input(); exact.commit(); exact.end("a".repeat(32768));
  assert.equal(Buffer.byteLength(exact.read("b".repeat(32)).reply), 32768);
  const unicode = fixture(t); unicode.emit("session_start"); unicode.input(); unicode.commit(); unicode.end("字".repeat(10923));
  assert.equal(unicode.read("readiness").failure, "reply_too_large");
});

test("a failed final receipt write fences later input even if Pi catches the callback error", (t) => {
  const f = fixture(t); f.emit("session_start"); f.input(); f.commit();
  fs.writeFileSync(path.join(f.directory, "b".repeat(32) + ".json.tmp"), "interrupted writer");
  assert.throws(() => f.end(), /EEXIST/);
  assert.equal(f.read("b".repeat(32)).stage, "started");
  assert.equal(f.input(f.wire("c".repeat(32))).action, "handled");
  assert.equal(fs.existsSync(path.join(f.directory, "c".repeat(32) + ".json")), false);
});

test("non-text final content cannot masquerade as a complete text reply", (t) => {
  const f = fixture(t); f.emit("session_start"); f.input(); f.commit();
  f.branch.push({ type: "message", id: "native-reply", message: { role: "assistant", stopReason: "stop",
    content: [{ type: "text", text: "answer" }, { type: "image", data: "opaque" }] } });
  f.emit("agent_before_settle", { outcome: "completed" }); f.emit("agent_settled");
  assert.equal(f.read("readiness").failure, "non_text_final_reply");
});

test("reinitializing an extension on the same session changes its frozen load identity", (t) => {
  const f = fixture(t); f.emit("session_start");
  const original = f.read("readiness");
  f.emit("session_start");
  assert.equal(f.read("readiness").session_id, original.session_id);
  assert.notEqual(f.read("readiness").instance_id, original.instance_id);
});

const managedOptions = {source: "pi-extension-v1",tools:["read","write","edit","powershell"],initialize:async()=>({verified:true})};
test("managed tools revalidate the live model, tools, thinking and workspace before execution", async (t) => {
  for (const change of [f=>{f.ctx.model.id="changed";},f=>f.setTools(["powershell"]),f=>f.setThinking("high"),f=>{f.ctx.cwd=path.dirname(f.directory);},f=>{f.ctx.sessionManager.getSessionId=()=>"replacement";}]) {
    const f=fixture(t,{},managedOptions);await f.emit("session_start");f.input();f.commit();
    assert.equal(f.gate.mayExecuteTools(f.ctx),true);change(f);
    assert.equal(f.gate.mayExecuteTools(f.ctx),false);assert.equal(f.read("readiness").stage,"unverified");
  }
});
test("managed thinking is read from Pi and an explicit clamped request blocks readiness", async (t) => {
  const f=fixture(t,{},managedOptions);f.config.reasoning_effort="xhigh";fs.writeFileSync(f.configFile,JSON.stringify(f.config));
  await f.emit("session_start");assert.equal(f.read("readiness").reasoning_effort,"low");
  assert.equal(f.read("readiness").failure,"native_thinking_level_mismatch");assert.equal(f.input().action,"handled");
  const ok=fixture(t,{},managedOptions);await ok.emit("session_start");assert.equal(ok.read("readiness").version,2);assert.equal(ok.read("readiness").reasoning_effort,"low");
});
test("native selection changes fence the instance before a later tool or input",async(t)=>{
  for(const event of ["model_select","thinking_level_select"]){const f=fixture(t,{},managedOptions);await f.emit("session_start");f.input();f.commit();f.emit(event);assert.equal(f.read("readiness").stage,"unverified");assert.equal(f.gate.mayExecuteTools(f.ctx),false);}
});

test("unknown native host outcomes fence later tools while ordinary command failures remain retryable",async(t)=>{
 const directory=fs.mkdtempSync(path.join(os.tmpdir(),"sr-tool-status-"));t.after(()=>fs.rmSync(directory,{recursive:true}));
 for(const code of [1,2,null]){
  let calls=0;const launch=()=>{calls++;const child=new EventEmitter();child.stdout=new PassThrough();child.stderr=new PassThrough();child.stdin=new PassThrough();queueMicrotask(()=>child.emit("close",code));return child;};
  const ops=createTerminalToolOperations({executable:path.join(directory,"host.exe"),policy:{access_mode:"full_access",workspace:directory}},launch);
  if(code===1){assert.equal((await ops.execute("command")).exitCode,1);assert.equal(ops.verified(),true);await ops.execute("another");assert.equal(calls,2);}
  else {await assert.rejects(ops.execute("command"),/native_tool_host_unverified/);assert.equal(ops.verified(),false);await assert.rejects(ops.execute("another"),/native_tool_host_unverified/);assert.equal(calls,1);}
 }
});
