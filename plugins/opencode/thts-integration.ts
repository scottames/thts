// thts integration plugin for OpenCode
// Injects thoughts/ instructions at session start

import type { Plugin } from "@opencode-ai/plugin";
import * as fs from "fs";
import * as path from "path";

export const ThtsIntegration: Plugin = async ({ directory }) => {
  const cwd = directory || process.cwd();
  const instructionsPath = path.join(cwd, ".opencode", "thts-instructions.md");

  return {
    "session.created": async () => {
      if (fs.existsSync(instructionsPath)) {
        const content = fs.readFileSync(instructionsPath, "utf-8");
        return {
          context: [
            {
              type: "text",
              content: `## thts Integration\n\n${content}`,
            },
          ],
        };
      }
      return {};
    },

    "session.compacting": async () => {
      // Re-inject instructions during context compaction to preserve them
      if (fs.existsSync(instructionsPath)) {
        const content = fs.readFileSync(instructionsPath, "utf-8");
        return {
          context: [
            {
              type: "text",
              content: `## thts Integration (Preserved)\n\n${content}`,
            },
          ],
        };
      }
      return {};
    },
  };
};
