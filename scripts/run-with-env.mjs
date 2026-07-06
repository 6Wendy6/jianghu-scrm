#!/usr/bin/env node
import { readFileSync, existsSync } from "node:fs";
import { spawn } from "node:child_process";

function parseEnvFile(path) {
  if (!existsSync(path)) return {};
  const values = {};
  const lines = readFileSync(path, "utf8").split(/\r?\n/);
  for (const rawLine of lines) {
    const line = rawLine.trim();
    if (!line || line.startsWith("#")) continue;
    const match = line.match(/^([A-Za-z_][A-Za-z0-9_]*)=(.*)$/);
    if (!match) continue;
    let value = match[2].trim();
    if (
      (value.startsWith('"') && value.endsWith('"')) ||
      (value.startsWith("'") && value.endsWith("'"))
    ) {
      value = value.slice(1, -1);
    }
    values[match[1]] = value;
  }
  return values;
}

const splitIndex = process.argv.indexOf("--");
const command = splitIndex === -1 ? process.argv.slice(2) : process.argv.slice(splitIndex + 1);
if (command.length === 0) {
  console.error("usage: node scripts/run-with-env.mjs -- <command> [args...]");
  process.exit(2);
}

const env = { ...process.env };
for (const [key, value] of Object.entries(parseEnvFile(".env"))) {
  if (!Object.prototype.hasOwnProperty.call(env, key)) {
    env[key] = value;
  }
}

const child = spawn(command[0], command.slice(1), {
  stdio: "inherit",
  env,
});

child.on("exit", (code, signal) => {
  if (signal) {
    process.kill(process.pid, signal);
    return;
  }
  process.exit(code ?? 0);
});
