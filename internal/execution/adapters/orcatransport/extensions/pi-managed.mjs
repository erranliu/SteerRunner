// Formal in-process Pi extension. These are tool-definition builders, not an
// SDK session/agent runner. The original Orca TUI owns the model and all turns.
import { createReadToolDefinition, createWriteToolDefinition, createEditToolDefinition, createPowerShellToolDefinition } from "@earendil-works/pi-coding-agent";
import { attachEvidence } from "./pi-evidence.mjs";
import { prepareTerminalTools } from "./terminal-tools.mjs";

export default function managed(pi) {
  let operations, evidenceState;
  const files = Object.fromEntries(["access", "readFile", "writeFile", "mkdir"].map((key) => [key, (...args) => {
    if (!operations) throw new Error("native_tool_preparation_pending");
    return operations.files[key](...args);
  }]));
  const shell = { exec: (...args) => {
    if (!operations) throw new Error("native_tool_preparation_pending");
    return operations.shell.exec(...args);
  } };
  const cwd = process.cwd();
  pi.registerTool(createReadToolDefinition(cwd, { operations: files }));
  pi.registerTool(createWriteToolDefinition(cwd, { operations: files }));
  pi.registerTool(createEditToolDefinition(cwd, { operations: files }));
  pi.registerTool(createPowerShellToolDefinition(cwd, { operations: shell, exposeSessionEnvironment: false }));
  pi.on("tool_call", (_event, ctx) => {
    if (!operations?.verified() || !evidenceState?.mayExecuteTools(ctx)) return { block: true, reason: "Native tool host is unverified" };
  });
  evidenceState = attachEvidence(pi, {
    source: "pi-extension-v1", tools: ["read", "write", "edit", "powershell"], healthy: () => !!operations?.verified(),
    initialize: async (config) => {
      const prepared = await prepareTerminalTools(config, JSON.parse(process.env.SR_TERMINAL_TOOL_ENV ?? "null"));
      operations = prepared.operations;
      return prepared.preparation;
    },
  });
}
