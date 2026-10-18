import test from "node:test";
import assert from "node:assert/strict";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { createHash } from "node:crypto";
import { attachDSHEvidence } from "./dsh-managed.mjs";

const hash = (value) => createHash("sha256").update(value).digest("hex");
const command = "a".repeat(32);
const prompt = "Original multiline\nrequest";
const wire = `[SteerRunner command=${command} input=${hash(prompt)}]\n${prompt}`;

function fixture(t, { flush = true, finish = "stop" } = {}) {
  const directory = fs.mkdtempSync(path.join(os.tmpdir(), "steerrunner-dsh-evidence-"));
  t.after(() => fs.rmSync(directory, { recursive: true, force: true })); // This exact fresh temporary directory only.
  const handlers = new Map(), scoped = new Map(), events = [], locals = new Map(), denied = new Set();
  const callConfig = {provider:"deepseek-official",model:"model",reasoningEffort:"low"};
  let guard, flushes = 0, cancelled = 0;
  const schemas = (scope) => [...(!scope || !denied.has("ask_user_question") ? [{ name: "ask_user_question" }] : []), ...(scope ? [...locals.values()] : [])];
  const session = { requestHeader: () => ({config:callConfig}), id: "native-dsh-session", header: { cwd: directory, isSeeded: false }, snapshotEvents: (from, to) => events.filter((event) => event.seq >= from && event.seq < to) };
  const agent = { id: session.id, session, options: { provider: "deepseek-official", model: "model", reasoningEffort: "low" },
    ctx: { on:(name,fn)=>scoped.set(name,fn), tools: { schemas, register: (definition) => locals.set(definition.name, definition), restrict: ({ deny }) => deny.forEach((name) => denied.add(name)) } },
    whenIdle: async () => {}, cancel: () => { cancelled++; } };
  const ctx = { tools: { schemas, guard: (value) => { guard = value; } }, sessions: { flush: async () => { flushes++; return flush; } },
    on: (name, callback) => handlers.set(name, callback) };
  attachDSHEvidence(ctx, { version: 1, run_id: "b".repeat(32), session_id: "frozen-binding", cwd: directory, directory },
    { callConfig, operations: { verified: () => true, execute: async () => ({ exitCode: 0, stdout: Buffer.from("ok"), stderr: Buffer.alloc(0) }) }, preparation: { verified: true } },
    { defineTool: (value) => value, lastAssistantStreamChunk: () => ({ reason: { kind: finish } }) });
  const emit = (name, ...args) => handlers.get(name)?.(...args);
  emit("agent/created", { agent, source: "startup" });
  const event = (type, data, target = session) => { const value = { type, seq: events.length, data }; if (target === session) events.push(value); emit("session/event", target, value); return value; };
  const start = (text = wire) => { event("turn/start", { turn: 1 }); event("user/message", { id: "user-original", source: { kind: "user" }, content: [{ type: "text", text }] }); };
  const assistant = (text = "Full native reply", turn = 1) => event("assistant/message", { turn, message: { id: "assistant-original", content: [{ type: "text", text }] }, stream: [] });
  const end = (reason = "completed", turn = 1) => event("turn/end", { turn, reason: { kind: reason } });
  const idle = async () => { emit("agent/status", { agent, status: "idle" }); await new Promise(setImmediate); };
  const read = (name = command) => JSON.parse(fs.readFileSync(path.join(directory, name + ".json"), "utf8"));
  return { callConfig, scoped, agent, session, event, emit, start, assistant, end, idle, read, guard: (name = "powershell", actor = agent) => guard({ name, agent: actor }), flushes: () => flushes, cancelled: () => cancelled };
}

test("DSH requires root quiescence and a real persistence checkpoint before settlement", async (t) => {
  const f = fixture(t);
  assert.ok(f.guard());
  f.start(); assert.equal(f.guard(), undefined);
  assert.equal(f.read("readiness").active_command_id, command);
  f.assistant(); f.end();
  assert.equal(f.read().stage, "started"); assert.equal(f.flushes(), 0);
  await f.idle();
  const result = f.read();
  assert.equal(result.stage, "settled"); assert.equal(result.outcome, "completed");
  assert.equal(result.native_input_id, "user-original"); assert.equal(result.native_reply_id, "assistant-original");
  assert.equal(result.reply, "Full native reply"); assert.equal(result.reply_sha256, hash(result.reply));
  assert.equal(f.flushes(), 1); assert.ok(f.guard());
});

test("child session events and unrelated native turn endings cannot complete the root", async (t) => {
  const f = fixture(t); f.start();
  f.event("turn/end", { turn: 1, reason: { kind: "completed" } }, { id: "child" });
  await f.idle(); assert.equal(f.read().stage, "started");
  f.assistant("Wrong turn", 2);
  assert.equal(f.read().stage, "unverified"); assert.ok(f.cancelled());
});

test("an empty or failed native flush never becomes a successful native receipt", async (t) => {
  const f = fixture(t, { flush: false }); f.start(); f.assistant(); f.end(); await f.idle();
  assert.equal(f.read().stage, "unverified"); assert.equal(f.read("readiness").stage, "unverified");
});

test("a native failure without an assistant reply is a settled failure with no invented final", async (t) => {
  const f = fixture(t); f.start(); f.end("error"); await f.idle();
  assert.equal(f.read().stage, "settled"); assert.equal(f.read().outcome, "error");
  assert.equal(f.read().reply, ""); assert.equal(f.read().native_reply_id, undefined);
});

test("manual, altered and duplicate inputs fence later tools without rewriting a settled result", async (t) => {
  const f = fixture(t); f.start(); f.assistant(); f.end(); await f.idle();
  const original = f.read(); f.start();
  assert.deepEqual(f.read(), original); assert.equal(f.read("readiness").stage, "unverified"); assert.ok(f.guard());
  const other = fixture(t); other.start(wire.replace(prompt, "changed")); assert.equal(other.read("readiness").stage, "unverified");
});

test("session replacement and replies over the UTF-8 bound cannot remain verified", (t) => {
  const f = fixture(t); f.start(); f.assistant("中".repeat(10923)); assert.equal(f.read().stage, "unverified");
  const other = fixture(t);
  assert.throws(() => other.emit("agent/created", { agent: other.agent, source: "resume" }), /unexpected_agent/);
  assert.equal(other.read("readiness").stage, "unverified");
  const model = fixture(t); model.start(); model.agent.options.model = "changed"; model.assistant();
  assert.equal(model.read("readiness").stage, "unverified"); assert.ok(model.guard());
});

test("DSH binds the native reasoning effort before tools and settlement", (t) => {
 const f=fixture(t);assert.equal(f.read("readiness").reasoning_effort,"low");f.start();f.agent.options.reasoningEffort="high";assert.ok(f.guard());f.agent.options.reasoningEffort="low";assert.ok(f.guard());f.assistant();assert.equal(f.read("readiness").stage,"unverified");
});

test("DSH maps frozen effort at the official request seam and verifies the durable header", async(t)=>{
 const f=fixture(t);const request=f.scoped.get("agent/request");
 const actual=await request({},async()=>({provider:"deepseek-official",model:"model"}));assert.equal(actual.reasoningEffort,"low");
 await assert.rejects(request({},async()=>({provider:"deepseek-official",model:"model",reasoningEffort:"high"})),/native_request_binding_changed/);
 assert.equal(f.read("readiness").stage,"unverified");
 const changed=fixture(t);changed.start();changed.callConfig.reasoningEffort="high";assert.ok(changed.guard());assert.equal(changed.read("readiness").stage,"unverified");
 const missing=fixture(t);missing.start();missing.assistant();missing.end();missing.session.requestHeader=()=>undefined;await missing.idle();assert.equal(missing.read("readiness").stage,"unverified");
});
