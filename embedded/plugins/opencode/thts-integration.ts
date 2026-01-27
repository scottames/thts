// thts integration plugin for OpenCode
// Injects thoughts/ instructions at session start

import type { Plugin } from "@opencode-ai/plugin";
import { execSync } from "child_process";

// Get instructions from thts CLI
function getInstructions(): string | null {
  try {
    execSync("which thts", { stdio: "ignore" });
  } catch {
    return null;
  }

  try {
    return execSync("thts agent-instructions", {
      encoding: "utf-8",
      stdio: ["ignore", "pipe", "ignore"],
    });
  } catch {
    return null;
  }
}

// Check if thts is enabled for a directory
function isThtsEnabled(cwd: string): boolean {
  try {
    execSync("thts init --check", { cwd, stdio: "ignore" });
    return true;
  } catch {
    return false;
  }
}

export const ThtsIntegration: Plugin = async ({ directory }) => {
  const cwd = directory || process.cwd();

  return {
    "experimental.chat.system.transform": async (input, output) => {
      if (!isThtsEnabled(cwd)) {
        return;
      }
      const content = getInstructions();
      if (content) {
        output.system.push(`## thts Integration\n\n${content}`);
      }
    },

    "experimental.session.compacting": async (input, output) => {
      if (!isThtsEnabled(cwd)) {
        return;
      }
      const content = getInstructions();
      if (content) {
        output.context.push(`## thts Integration (Preserved)\n\n${content}`);
      }
    },
  };
};
