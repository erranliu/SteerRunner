// Official Cordis plugin inside the original DSH TUI. No agent factory, SDK,
// ACP, remote driver, transcript parser, or model-generated marker is used.
import fs from "node:fs";
import path from "node:path";
import { pathToFileURL, fileURLToPath } from "node:url";
import { createHash, randomUUID } from "node:crypto";
import { writeReceipt } from "./receipt.mjs";
import { prepareTerminalTools } from "./terminal-tools.mjs";

export const name = "steerrunner-dsh-native";
export const inject = ["agents", "sessions", "tools", "llm"];
const hash = (value) => createHash("sha256").update(value).digest("hex");
const body = (content) => content.filter((part) => part.type === "text").map((part) => part.text).join("");

export async function apply(ctx) {
  const location = process.env.SR_TERMINAL_CONFIG;
  if (!location || fs.statSync(location).size > 65536 || !process.stdin.isTTY || !process.stdout.isTTY) throw new Error("native_tui_configuration_missing");
  const config = JSON.parse(fs.readFileSync(location, "utf8"));
  if (config.version !== 1 || !/^[a-f0-9]{32}$/.test(config.run_id) || path.resolve(process.cwd()) !== config.cwd) throw new Error("native_tui_configuration_changed");
  const { defineTool } = await import(pathToFileURL(config.dsh_tools_module));
  const { lastAssistantStreamChunk } = await import(pathToFileURL(config.dsh_llm_module));
  const prepared = await prepareTerminalTools(config, JSON.parse(process.env.SR_TERMINAL_TOOL_ENV ?? "null"));
  // Official adapter capability resolution: validates an exact effort without
  // provider I/O, clamping, creating an agent, or fabricating a native request.
  prepared.callConfig = await ctx.llm.resolveCallConfig({ provider: "deepseek-official", model: config.model || "deepseek-flash", reasoningEffort: config.reasoning_effort || "low" });
  if (prepared.callConfig.reasoningEffort !== (config.reasoning_effort || "low")) throw new Error("native_thinking_level_mismatch");
  attachDSHEvidence(ctx, config, prepared, { defineTool, lastAssistantStreamChunk });
}

export function attachDSHEvidence(ctx, config, prepared, { defineTool, lastAssistantStreamChunk }) {
  let root, identity, current, failure, openTurn, settling = Promise.resolve();
  const seen = new Set();
  const sourceDigest = hash(fs.readFileSync(fileURLToPath(import.meta.url)));
  const save = (name, value) => {
    try { writeReceipt(config.directory, name, { ...identity, ...value }); }
    catch (error) { failure ||= "receipt_write_failed"; throw error; }
  };
  const fail = (reason) => {
    failure ||= reason;
    root?.cancel({ kind: "user" });
    if (current && current.stage !== "settled") { current.stage = "unverified"; current.failure = failure; save(current.command_id, current); }
    if (identity) save("readiness", { stage: "unverified", failure });
  };
  const toolsMatch = (agent) => JSON.stringify(agent.ctx.tools.schemas(agent).map((tool) => tool.name).sort()) === '["powershell"]';
  const bound = (agent) => agent === root && agent.session.id === identity?.native_session_id &&
    `${agent.options.provider}/${agent.options.model}` === identity?.model &&
    (agent.options.reasoningEffort === undefined || agent.options.reasoningEffort === identity?.reasoning_effort) &&
    (!agent.session.requestHeader() ||
      `${agent.session.requestHeader().config.provider}/${agent.session.requestHeader().config.model}` === identity?.model &&
      agent.session.requestHeader().config.reasoningEffort === identity?.reasoning_effort) &&
    path.resolve(agent.session.header.cwd) === config.cwd && toolsMatch(agent) && prepared.operations.verified() && !failure;

  ctx.tools.guard((execution) => {
    if (root && execution.agent === root && !bound(root)) fail("native_tool_binding_changed");
    return !root || execution.agent !== root || execution.name !== "powershell" || !bound(root) || current?.stage !== "started" ? "Native tool binding unverified" : undefined;
  });
  ctx.on("agent/created", ({ agent, source }) => {
    if (root || source !== "startup" || agent.session.header.parentSession || agent.session.header.isSeeded || path.resolve(agent.session.header.cwd) !== config.cwd) {
      fail("unexpected_agent_or_session"); throw new Error("unexpected_agent_or_session");
    }
    root = agent;
    agent.ctx.tools.register(defineTool({
      name: "powershell", description: "Execute Windows PowerShell 5.1 in the fixed workspace. Use it for files, Git, tests and publication. Commands use the frozen native permissions and repository authentication; approval escalation is unavailable.",
      parameters: { command: { type: "string", required: true, description: "PowerShell script to execute" } },
      output: { schema: { type: "json" }, render: (_args, value) => [{ type: "text", text: JSON.stringify(value) }] },
      async execute(args, execution) {
        if (!bound(agent)) throw new Error("native_tool_binding_unverified");
        const result = await prepared.operations.execute(args.command, { signal: execution.signal });
        return { exit_code: result.exitCode, stdout: result.stdout.toString("utf8"), stderr: result.stderr.toString("utf8") };
      },
    }));
    // The TUI contributes a global ask-user tool independently of presets.
    // Runner's business needs_input/reply contract owns that interaction.
    const globals = ctx.tools.schemas().map((tool) => tool.name);
    if (globals.length) agent.ctx.tools.restrict({ deny: globals });
    if (!toolsMatch(agent)) { writeReceipt(config.directory,"load-diagnostic",{stage:"unexpected_native_tools",global:ctx.tools.schemas().map(t=>t.name),agent:agent.ctx.tools.schemas(agent).map(t=>t.name)}); throw new Error("unexpected_native_tools"); }
    identity = { version: 2, source: "dsh-plugin-v1", source_sha256: sourceDigest, run_id: config.run_id,
      session_id: config.session_id, native_session_id: agent.session.id, instance_id: randomUUID(), pid: process.pid,
      process_started_at: new Date(performance.timeOrigin).toISOString(), mode: "tui", tools: ["powershell"],
      model: `${agent.options.provider}/${agent.options.model}`, reasoning_effort: prepared.callConfig.reasoningEffort, preparation: prepared.preparation };
    if ((config.reasoning_effort || "low") !== identity.reasoning_effort) { fail("native_thinking_level_mismatch"); return; }
    if (`${prepared.callConfig.provider}/${prepared.callConfig.model}` !== identity.model) { fail("native_model_mismatch"); return; }
    // Scope the frozen parameter mapping to this original agent. TUI selection
    // lives outside Agent.options; the official waterfall is its request seam.
    agent.ctx.on("agent/request", async (_payload, next) => {
      const selected = await next();
      if (!bound(agent) || `${selected.provider}/${selected.model}` !== identity.model ||
          selected.reasoningEffort !== undefined && selected.reasoningEffort !== identity.reasoning_effort) {
        fail("native_request_binding_changed"); throw new Error("native_request_binding_changed");
      }
      return { ...selected, reasoningEffort: identity.reasoning_effort };
    }, { prepend: true });
    save("readiness", { stage: "ready" });
  });
  ctx.on("session/event", (session, event) => {
    if (!identity || session !== root.session || failure) return;
    try {
      if (!bound(root)) { fail("native_binding_changed"); return; }
      if (event.type === "turn/start") { openTurn = event.data.turn; return; }
      if (event.type === "user/message" && event.data.source.kind === "user") {
        const wire = body(event.data.content);
        const match = /^\[SteerRunner command=([a-f0-9]{32}) input=([a-f0-9]{64})\]\n([\s\S]*)$/.exec(wire);
        if (!match || hash(match[3]) !== match[2] || current && current.stage !== "settled" || seen.has(match[1]) || fs.existsSync(path.join(config.directory, match[1] + ".json"))) {
          writeReceipt(config.directory, "input-diagnostic", { stage: "input_rejected", prefix: wire.slice(0, 180), wire_sha256: hash(wire), length: wire.length, lines: wire.split("\n").length });
          fail("unassociated_or_duplicate_input"); return;
        }
        seen.add(match[1]);
        current = { command_id: match[1], input_sha256: match[2], wire_sha256: hash(wire), stage: "started",
          native_input_id: event.data.id, native_input_seq: event.seq, native_turn_index: openTurn };
        save(current.command_id, current);
        save("readiness", { stage: "ready", active_command_id: current.command_id });
      } else if (current && current.stage === "started" && event.type === "assistant/message") {
        if (event.data.turn !== current.native_turn_index) { fail("root_turn_changed"); return; }
        const message = event.data.message;
        const reply = body(message.content);
        if (Buffer.byteLength(reply) > 32768) { fail("reply_too_large"); return; }
        current.native_reply_id = message.id;
        current.native_reply_seq = event.seq;
        current.native_turn_index = event.data.turn;
        current.reply = reply;
        current.reply_sha256 = hash(reply);
        current.stop_reason = lastAssistantStreamChunk(event.data.stream, "finish")?.reason.kind ?? "unknown";
        if (event.data.interrupted || message.content.some((part) => !["text", "reasoning"].includes(part.type))) current.stop_reason = "incomplete";
      } else if (current && current.stage === "started" && event.type === "turn/end") {
        if (event.data.turn !== current.native_turn_index) { fail("root_turn_changed"); return; }
        current.outcome = event.data.reason.kind;
        current.native_end_seq = event.seq;
      }
    } catch { fail("native_event_evidence_failed"); }
  });
  ctx.on("agent/status", ({ agent, status }) => {
    if (agent !== root || status !== "idle" || !current || current.native_end_seq === undefined || current.stage !== "started") return;
    const pending = current;
    settling = (async () => {
      await agent.whenIdle();
      if (!await ctx.sessions.flush(agent.session)) throw new Error("native_session_flush_unverified");
      if (!bound(agent) || current !== pending) throw new Error("native_settlement_changed");
      if (pending.outcome === "completed" && !agent.session.requestHeader()) throw new Error("native_request_header_missing");
      const user = agent.session.snapshotEvents(pending.native_input_seq, pending.native_input_seq + 1)[0];
      const end = agent.session.snapshotEvents(pending.native_end_seq, pending.native_end_seq + 1)[0];
      if (user?.type !== "user/message" || hash(body(user.data.content)) !== pending.wire_sha256 || user.data.id !== pending.native_input_id ||
          end?.type !== "turn/end" || end.data.reason.kind !== pending.outcome) throw new Error("native_durable_entries_changed");
      if (pending.native_reply_seq !== undefined) {
        const reply = agent.session.snapshotEvents(pending.native_reply_seq, pending.native_reply_seq + 1)[0];
        if (reply?.type !== "assistant/message" || reply.data.message.id !== pending.native_reply_id || hash(body(reply.data.message.content)) !== pending.reply_sha256) throw new Error("native_durable_reply_changed");
      } else {
        if (pending.outcome === "completed") throw new Error("native_durable_reply_missing");
        pending.reply = ""; pending.reply_sha256 = hash(""); pending.stop_reason = pending.outcome;
      }
      pending.stage = "settled";
      save(pending.command_id, pending);
    })().catch(() => fail("native_settlement_unverified"));
  });
  ctx.on("agent/disposed", ({ agent }) => { if (agent === root) fail("original_agent_disposed"); });
  ctx.on("dispose", async () => { await settling; if (identity) fail("session_shutdown"); });
}
