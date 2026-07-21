// thts integration extension for Pi.
// Makes thoughts instructions available once per agent start.

import type { ExtensionAPI } from "@earendil-works/pi-coding-agent";

const THTS_MARKER = "<!-- thts-integration -->";
const THTS_HEADING = "# thts Integration Instructions";
const TIMEOUT_MS = 10_000;

type CommandResult = {
  code: number;
  stdout: string;
};

export default function thtsIntegration(pi: ExtensionAPI) {
  const policies = new Map<string, string>();

  pi.on("before_agent_start", async (event, ctx) => {
    if (
      event.systemPrompt.includes(THTS_MARKER) ||
      event.systemPrompt.includes(THTS_HEADING)
    ) {
      return;
    }

    const options = { cwd: ctx.cwd, signal: ctx.signal, timeout: TIMEOUT_MS };
    let initialized: CommandResult | undefined;
    try {
      initialized = await pi.exec("thts", ["init", "--check"], options);
    } catch {
      policies.delete(ctx.cwd);
      return;
    }
    if (!initialized || initialized.code !== 0) {
      policies.delete(ctx.cwd);
      return;
    }

    let policy = policies.get(ctx.cwd);
    if (!policy) {
      let instructions: CommandResult | undefined;
      try {
        instructions = await pi.exec("thts", ["agent-instructions"], options);
      } catch {
        policies.delete(ctx.cwd);
        return;
      }
      if (
        !instructions ||
        instructions.code !== 0 ||
        typeof instructions.stdout !== "string"
      ) {
        policies.delete(ctx.cwd);
        return;
      }
      policy = instructions.stdout.trim();
      if (!policy) {
        policies.delete(ctx.cwd);
        return;
      }
      policies.set(ctx.cwd, policy);
    }

    return {
      systemPrompt: `${event.systemPrompt}\n\n${THTS_MARKER}\n${policy}`,
    };
  });
}
