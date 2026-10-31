// Official Pi tool operations, bound to Runner's ephemeral native tool host.
// No agent session, RPC client, retry queue or background receiver is created.
import { spawn } from "node:child_process";
import path from "node:path";
import { writeReceipt } from "./receipt.mjs";

const literal = (value) => "'" + value.replaceAll("'", "''") + "'";
const executableLimit = 16 * 1024 * 1024;

export function createTerminalToolOperations({ executable, policy, environment = process.env }, launch = spawn) {
  if (!path.isAbsolute(executable) || policy.access_mode !== "full_access") {
    throw new Error("invalid_frozen_tool_policy");
  }
  const configuration = Buffer.from(JSON.stringify(policy)).toString("base64url");
  let failed = false;
  async function execute(body, options = {}) {
    if (failed) throw new Error("native_tool_host_unverified");
    if (options.signal?.aborted) throw new Error("tool_cancelled_before_start");
    if (Buffer.byteLength(body) > 1024 * 1024) throw new Error("tool_input_too_large");
    return new Promise((resolve, reject) => {
      const child = launch(executable, ["--terminal-tool", configuration], {
        cwd: policy.workspace, env: environment, windowsHide: true,
        stdio: ["pipe", "pipe", "pipe"],
      });
      const chunks = [], errors = [];
      let size = 0;
      const receive = (list) => (chunk) => {
        size += chunk.length;
        if (size > executableLimit) {
          // Do not kill only the helper root and orphan its Job. The host has
          // its own bounded tool lifetime and will revoke/prove the entire tree.
          failed = true;
          return;
        }
        list.push(chunk);
        options.onData?.(chunk);
      };
      child.stdout.on("data", receive(chunks)); child.stderr.on("data", receive(errors));
      child.on("error", () => { failed = true; reject(new Error("native_tool_host_unavailable")); });
      child.on("close", (code) => {
        if (code === 2 || code === null || failed) {
          failed = true; reject(new Error("native_tool_host_unverified")); return;
        }
        resolve({ exitCode: code, stdout: Buffer.concat(chunks), stderr: Buffer.concat(errors) });
      });
      // A cancellation does not pretend the root PID is all writers. Stop
      // authorizing calls; the existing host deadline revokes the owned Job.
      const cancel = () => { failed = true; };
      options.signal?.addEventListener("abort", cancel, { once: true });
      child.once("close", () => options.signal?.removeEventListener("abort", cancel));
      child.stdin.on("error", () => { failed = true; });
      child.stdin.end(body);
    });
  }
  async function checked(body) {
    const result = await execute("$ErrorActionPreference='Stop';\n" + body);
    if (result.exitCode !== 0) throw new Error("native_filesystem_operation_failed");
    return result.stdout;
  }
  const files = {
    access: async (target) => { await checked(`$f=[IO.File]::OpenRead(${literal(target)});$f.Dispose()`); },
    readFile: async (target) => {
      const data = await checked(`$f=Get-Item -LiteralPath ${literal(target)};if($f.Length -gt 8388608){throw 'file_read_limit'};[Console]::Write([Convert]::ToBase64String([IO.File]::ReadAllBytes(${literal(target)})))`);
      return Buffer.from(data.toString("utf8").trim(), "base64");
    },
    writeFile: async (target, content) => {
      const bytes = Buffer.from(content).toString("base64");
      await checked(`[IO.File]::WriteAllBytes(${literal(target)},[Convert]::FromBase64String('${bytes}'))`);
    },
    mkdir: async (target) => { await checked(`[IO.Directory]::CreateDirectory(${literal(target)}) | Out-Null`); },
  };
  return {
    shell: { exec: async (command, cwd, options) => {
      if (path.resolve(cwd).toLowerCase() !== path.resolve(policy.workspace).toLowerCase()) throw new Error("tool_workspace_changed");
      const result = await execute(command, options);
      return { exitCode: result.exitCode };
    } },
    files,
    execute,
    verified: () => !failed,
  };
}

export async function prepareTerminalTools(config, environment, launch = spawn) {
  let failure = "native_tool_preparation_failed", stage = "unverified";
  try {
      if (!config.tool_policy || !environment) throw new Error("native_tool_preparation_missing");
      const operations = createTerminalToolOperations({ executable: config.tool_host, policy: config.tool_policy, environment: environment }, launch);
      const checked = await operations.execute("$ErrorActionPreference='Stop';if($env:OPENROUTER_API_KEY -or $env:DEEPSEEK_API_KEY -or $env:OPENAI_API_KEY -or $env:ANTHROPIC_API_KEY){throw 'provider_environment_leak'};[void](Get-Item -Force -LiteralPath '.')");
      if (checked.exitCode !== 0) throw new Error("native_tool_preparation_failed");
      return { operations, preparation: { contract: "windows-terminal-tools-v3:full-access:host-native-auth:never", access_mode: config.tool_policy.access_mode,
        verified: true, provider_environment_absent: true } };
  } catch (error) {
      // A plugin manager may keep its TUI alive after a rejected initializer.
      // Preserve a bounded failure fact, never raw output/env or success proof.
      if (path.isAbsolute(config.directory ?? "") && /^[a-f0-9]{32}$/.test(config.run_id ?? "") &&
          typeof config.session_id === "string" && /^[a-f0-9]{64}$/.test(config.extension_bundle_sha256 ?? "")) {
        writeReceipt(config.directory, "preparation-failure", { version: 1, run_id: config.run_id,
          session_id: config.session_id, extension_bundle_sha256: config.extension_bundle_sha256, pid: process.pid, failure, probe_stage: stage });
      }
      throw error;
  }
}
