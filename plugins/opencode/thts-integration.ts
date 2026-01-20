// thts integration plugin for OpenCode
// Injects thoughts/ instructions at session start

import { Plugin } from "opencode";
import * as fs from "fs";
import * as path from "path";

const plugin: Plugin = {
  name: "thts-integration",
  version: "1.0.0",

  hooks: {
    "session.created": async (context) => {
      const cwd = context.cwd || process.cwd();
      const instructionsPath = path.join(
        cwd,
        ".opencode",
        "thts-instructions.md",
      );

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

    "experimental.session.compacting": async (context) => {
      // Re-inject instructions during context compaction to preserve them
      const cwd = context.cwd || process.cwd();
      const instructionsPath = path.join(
        cwd,
        ".opencode",
        "thts-instructions.md",
      );

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
  },
};

export default plugin;
