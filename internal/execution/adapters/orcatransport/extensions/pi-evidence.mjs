// P0 candidate: loaded explicitly into the original Pi TUI. No SDK/session driver.
import { createHash, randomUUID } from "node:crypto";
import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const hash = (value) => createHash("sha256").update(value).digest("hex");
const text = (content) => typeof content === "string" ? content :
  content.filter((part) => part.type === "text").map((part) => part.text).join("");
import { writeReceipt } from "./receipt.mjs";
export { writeReceipt } from "./receipt.mjs";

export default function evidence(pi) { attachEvidence(pi); }

export function attachEvidence(pi, options = {}) {
  pi.registerFlag("steerrunner-evidence-config", { type: "string", description: "Run-private P0 evidence configuration" });
  let config, identity, current, failure;
  const seen = new Set();
  const sourceDigest = hash(fs.readFileSync(fileURLToPath(import.meta.url)));
  const save = (name, value) => {
    try { writeReceipt(config.directory, name, { ...identity, ...value }); }
    catch (error) {
      // Pi reports handler exceptions and may keep the TUI alive. Fence every
      // later input even when the failing callback was outside the input hook.
      failure ||= "receipt_write_failed";
      throw error;
    }
  };
  const fail = (reason) => {
    failure ||= reason;
    if (current && current.stage !== "settled") {
      current.stage = "unverified"; current.failure = failure; save(current.command_id, current);
    }
    if (identity) save("readiness", { stage: "unverified", failure });
  };
  const toolsMatch = () => JSON.stringify([...pi.getActiveTools()].sort()) === JSON.stringify([...(options.tools ?? [])].sort());
  const bound = (ctx) => identity && ctx.mode === "tui" &&
    ctx.sessionManager.getSessionId() === identity.session_id &&
    (ctx.model ? `${ctx.model.provider}/${ctx.model.id}` : "") === identity.model &&
    (!options.source || pi.getThinkingLevel() === identity.reasoning_effort) &&
    path.resolve(ctx.cwd) === config.cwd && toolsMatch() && (options.healthy?.() ?? true);

  pi.on("session_start", (_event, ctx) => {
    const location = pi.getFlag("steerrunner-evidence-config");
    if (typeof location !== "string" || fs.statSync(location).size > (options.initialize ? 65536 : 4096)) throw new Error("invalid_probe_config");
    config = JSON.parse(fs.readFileSync(location, "utf8"));
    if (config.version !== 2 || !/^[a-f0-9]{32}$/.test(config.run_id) ||
        !path.isAbsolute(config.directory) || !path.isAbsolute(config.cwd) ||
        ctx.mode !== "tui" || !toolsMatch() ||
        ctx.sessionManager.getSessionId() !== config.session_id ||
        path.resolve(ctx.cwd) !== config.cwd) throw new Error("probe_binding_mismatch");
    const ready = (preparation) => {
      identity = { version: options.source ? 4 : 1, source: options.source ?? "pi-extension-p0", source_sha256: sourceDigest,
      run_id: config.run_id, session_id: config.session_id, instance_id: randomUUID(), pid: process.pid,
      process_started_at: new Date(performance.timeOrigin).toISOString(), mode: ctx.mode,
      tools: pi.getActiveTools(), model: ctx.model ? `${ctx.model.provider}/${ctx.model.id}` : "", preparation,
      ...(options.source ? { reasoning_effort: pi.getThinkingLevel() } : {}) };
      if (config.reasoning_effort && identity.reasoning_effort !== config.reasoning_effort) {
        fail("native_thinking_level_mismatch"); return;
      }
      save("readiness", { stage: "ready" });
    };
    if (options.initialize) return Promise.resolve(options.initialize(config, ctx)).then(ready);
    ready();
  });

  pi.on("input", (event, ctx) => {
    try {
      if (!bound(ctx) || failure) { fail("native_binding_changed"); return { action: "handled" }; }
      const match = /^\[SteerRunner command=([a-f0-9]{32}) input=([a-f0-9]{64})\]\n([\s\S]*)$/.exec(event.text);
      if (event.source !== "interactive" || event.images?.length || !match ||
          hash(match[3]) !== match[2] || current && current.stage !== "settled" ||
          seen.has(match[1]) || fs.existsSync(path.join(config.directory, match[1] + ".json"))) {
        fail("unassociated_or_duplicate_input");
        return { action: "handled" };
      }
      seen.add(match[1]);
      current = { command_id: match[1], input_sha256: match[2], wire_sha256: hash(event.text), stage: "input_observed" };
      save(current.command_id, current);
      save("readiness", { stage: "ready", active_command_id: current.command_id });
      return { action: "continue" };
    } catch {
      failure = "receipt_write_failed";
      return { action: "handled" };
    }
  });
  pi.on("before_agent_start", (event, ctx) => {
    if (!bound(ctx) || !current || hash(event.prompt) !== current.wire_sha256 || failure) {
      fail("expanded_input_mismatch");
    }
  });
  pi.on("message_start", (event, ctx) => {
    if (event.message.role !== "user") return;
    if (!bound(ctx) || !current || hash(text(event.message.content)) !== current.wire_sha256) {
      fail("committed_input_mismatch"); return;
    }
    current.stage = "started";
    save(current.command_id, current);
  });
  pi.on("turn_start", (event) => {
    if (current) current.native_turn_index = event.turnIndex;
  });
  pi.on("before_provider_request", (event) => {
    // Bound generation through the official hook: 256 tokens for the tiny P0
    // probe, 8192 for the managed deployment. Do not change global models.
    const payload = { ...event.payload };
    for (const key of ["max_tokens", "max_completion_tokens"]) {
      if (typeof payload[key] === "number") payload[key] = Math.min(payload[key], options.source ? 8192 : 256);
    }
    return payload;
  });
  pi.on("agent_before_settle", (event) => {
    if (current) current.outcome = event.outcome;
  });
  pi.on("agent_settled", (_event, ctx) => {
    if (!current) return;
    if (!bound(ctx) || failure || current.stage !== "started" || ctx.hasPendingMessages()) {
      fail("settlement_binding_mismatch"); return;
    }
    // Official read-only session API, after message_end persistence has finished.
    const branch = ctx.sessionManager.getBranch();
    const inputs = branch.filter((entry) => entry.type === "message" && entry.message.role === "user" &&
      hash(text(entry.message.content)) === current.wire_sha256);
    const last = [...branch].reverse().find((entry) => entry.type === "message" && entry.message.role === "assistant");
    if (inputs.length !== 1 || !last || branch.indexOf(last) <= branch.indexOf(inputs[0])) {
      fail("native_entries_missing_or_ambiguous"); return;
    }
    if (last.message.content.some((part) => part.type !== "text" && part.type !== "thinking")) {
      fail("non_text_final_reply"); return;
    }
    const body = text(last.message.content);
    if (Buffer.byteLength(body) > 32 * 1024) { fail("reply_too_large"); return; }
    current.native_input_id = inputs[0].id;
    current.native_reply_id = last.id;
    current.stop_reason = last.message.stopReason;
    if (last.message.stopReason === "error") {
      // Export only a category, never provider payloads, headers or credentials.
      const message = last.message.errorMessage ?? "";
      current.error_kind = /401|403|unauthorized|authentication|api.key/i.test(message) ? "provider_auth" :
        /404|model.*not.found|no.endpoints/i.test(message) ? "provider_model" :
        /402|credits|max_tokens/i.test(message) ? "provider_token_budget" :
        /429|rate.limit/i.test(message) ? "provider_rate_limit" :
        /fetch|connect|timeout|ENOTFOUND|ECONN/i.test(message) ? "provider_network" : "provider_error";
    }
    current.reply = body;
    current.reply_sha256 = hash(body);
    current.stage = "settled";
    // An error/abort stays a factual settled failure; the consumer must require success.
    save(current.command_id, current);
  });
  pi.on("session_shutdown", (event) => { fail("session_shutdown_" + event.reason); });
  // Even with tools disabled, Pi's user shell shortcut is a separate native entry.
  pi.on("user_bash", () => {
    fail("unexpected_user_shell");
    return { result: { output: "Shell disabled in the SteerRunner P0 probe", exitCode: 1, cancelled: false, truncated: false } };
  });
  pi.on("model_select", () => { if (identity) fail("native_model_changed"); });
  pi.on("thinking_level_select", () => { if (identity) fail("native_thinking_level_changed"); });
  return { mayExecuteTools: (ctx) => {
    if (!bound(ctx) || failure) { if (identity) fail("native_tool_binding_changed"); return false; }
    return current?.stage === "started";
  } };
}
