import fs from "node:fs";
import path from "node:path";

const limit = 64 * 1024;

// One local channel. Every replacement is fsynced before becoming visible.
export function writeReceipt(directory, name, value) {
  const bytes = Buffer.from(JSON.stringify(value));
  if (bytes.length > limit) throw new Error("receipt_too_large");
  const target = path.join(directory, name + ".json");
  const temporary = target + ".tmp";
  const fd = fs.openSync(temporary, "wx", 0o600);
  try {
    fs.writeFileSync(fd, bytes);
    fs.fsyncSync(fd);
  } finally {
    fs.closeSync(fd);
  }
  // Windows readers (including the Runner) can briefly deny replacement.
  // Retry only publication of these same fsynced bytes, never agent work/input.
  const until = Date.now() + 1000;
  for (;;) {
    try {
      fs.renameSync(temporary, target);
      return;
    } catch (error) {
      if (process.platform !== "win32" || !["EPERM", "EACCES", "EBUSY"].includes(error.code) || Date.now() >= until) throw error;
      Atomics.wait(new Int32Array(new SharedArrayBuffer(4)), 0, 0, 10);
    }
  }
}
