// Native, model-free check of the exact operation objects used by Pi tools.
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { createTerminalToolOperations } from "../internal/execution/adapters/orcatransport/extensions/terminal-tools.mjs";

const [host, sandbox, destination] = process.argv.slice(2);
if (!host || !sandbox || !destination) throw new Error("usage: node probe-terminal-tools.mjs <host.exe> <codex.exe> <new-evidence-directory>");
const directory = path.resolve(destination);
fs.mkdirSync(directory); // Never reuse another attempt or overwrite evidence.
const workspace = path.join(directory, "workspace"), state = path.join(directory, "state");
fs.mkdirSync(workspace); fs.mkdirSync(state);
const shell = path.join(process.env.SystemRoot, "System32", "WindowsPowerShell", "v1.0", "powershell.exe");
const records = [];
for (const mode of ["sandbox", "full_access"]) {
  const operations = createTerminalToolOperations({ executable: path.resolve(host), environment: { ...process.env, STEERRUNNER_SECRET_CANARY: "must-not-reach-tools" }, policy: {
    access_mode: mode, workspace, state_directory: state, write_roots: [], sandbox_executable: path.resolve(sandbox), shell,
  } });
  const token = await operations.execute(`Add-Type -TypeDefinition 'using System; using System.Runtime.InteropServices; public static class SRPiTokenProbe { [DllImport("advapi32.dll", SetLastError=true)] public static extern bool GetTokenInformation(IntPtr h,int c,out int v,int n,out int r); }'; $value=0;$size=0;if(-not [SRPiTokenProbe]::GetTokenInformation([Security.Principal.WindowsIdentity]::GetCurrent().Token,29,[ref]$value,4,[ref]$size)){throw 'token query'};[Console]::Write($value)`);
  if (token.exitCode !== 0 || token.stdout.toString().trim() !== (mode === "sandbox" ? "1" : "0")) throw new Error("native token does not match selected mode");
  const inside = path.join(workspace, mode + ".txt"), outside = path.join(directory, mode + "-outside.txt");
  await operations.files.writeFile(inside, "实际 Pi 文件操作：" + mode);
  if ((await operations.files.readFile(inside)).toString() !== "实际 Pi 文件操作：" + mode) throw new Error("inside readback failed");
  let denied = false;
  try { await operations.files.writeFile(outside, "outside canary"); } catch { denied = true; }
  if ((mode === "sandbox") !== denied || fs.existsSync(outside) !== (mode === "full_access")) throw new Error("native outside-write behavior differs from selected mode");
  const env = await operations.execute("if($env:STEERRUNNER_SECRET_CANARY -or $env:DEEPSEEK_API_KEY -or $env:OPENROUTER_API_KEY){exit 9}");
  if (env.exitCode !== 0) throw new Error("provider/caller secret crossed native tool boundary");
  const foreign=path.join(workspace,mode+"-foreign.ps1");
  const prefix=`& '${foreign.replaceAll("'","''")}'; `;
  await operations.files.writeFile(foreign,"\n".repeat(4)+" ".repeat(prefix.length)+`& $PSHOME\\powershell.exe -NoProfile -Command "[Console]::Error.WriteLine('progress');exit 0" 2>&1`);
  const statuses=[];
  for(const [name,body,expected] of [
    ["foreign_native_error_cannot_forgive_cmdlet_failure",prefix+`Get-Item missing-native-file -ErrorAction Ignore`,1],
    ["successful_native_stderr",`& $PSHOME\\powershell.exe -NoProfile -Command "[Console]::Error.WriteLine('native-progress');exit 0" 2>&1`,0],
    ["native_success_then_cmdlet_failure",`& $PSHOME\\powershell.exe -NoProfile -Command 'exit 0';Get-Item -LiteralPath './missing-native-file'`,1],
    ["handled_native_failure",`& $PSHOME\\powershell.exe -NoProfile -Command 'exit 7';if($LASTEXITCODE -eq 7){Write-Output 'handled'}`,0],
    ["ignored_cmdlet_failure_after_native_stderr",`& $PSHOME\\powershell.exe -NoProfile -Command "[Console]::Error.WriteLine('native-progress');exit 0" 2>&1;Get-Item -LiteralPath './missing-native-file' -ErrorAction Ignore`,1],
  ]) {
    const result=await operations.execute(body);
    if(result.exitCode!==expected)throw new Error(`${mode}:${name}:exit_${result.exitCode}`);
    statuses.push({name,exit_code:result.exitCode,expected});
  }
  records.push({ statuses, mode, native_app_container: mode === "sandbox", inside_read_write: true, outside_denied: denied, provider_secrets_excluded: true });
}
if (fs.readdirSync(state).length) throw new Error("native tool staging was not cleaned");
fs.writeFileSync(path.join(directory, "summary.json"), JSON.stringify({ status: "passed_native_tools", production_qualified: false, host: os.platform(), records }, null, 2));
console.log(JSON.stringify({ status: "passed_native_tools", records }));
