// thts integration plugin for OpenCode
// Makes thoughts/ instructions available once per model request.

import type { Plugin } from "@opencode-ai/plugin";

const THTS_MARKER = "<!-- thts-integration -->";
const THTS_HEADING = "# thts Integration Instructions";

export const ThtsIntegration: Plugin = async ({ directory, $ }) => {
  let instructions: Promise<string | null> | undefined;

  const getInstructions = async () => {
    instructions ??= (async () => {
      const result = await $`thts agent-instructions`
        .cwd(directory)
        .quiet()
        .nothrow();
      if (result.exitCode !== 0) {
        return null;
      }

      return result.text().trim() || null;
    })();

    const content = await instructions;
    if (!content) {
      instructions = undefined;
    }
    return content;
  };

  return {
    "experimental.chat.system.transform": async (_input, output) => {
      const alreadyPresent = output.system.some(
        (content) =>
          content.includes(THTS_MARKER) || content.includes(THTS_HEADING),
      );
      if (alreadyPresent) {
        return;
      }

      const enabled = await $`thts init --check`
        .cwd(directory)
        .quiet()
        .nothrow();
      if (enabled.exitCode !== 0) {
        instructions = undefined;
        return;
      }

      const content = await getInstructions();
      if (content) {
        output.system.push(`${THTS_MARKER}\n${content}`);
      }
    },
  };
};
