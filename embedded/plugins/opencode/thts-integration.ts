// thts integration for OpenCode v1.18.29+ and v2.
// Keep this file self-contained: thts installs it without an npm package.

import { execFile } from "node:child_process";
import { promisify } from "node:util";
import type { Plugin as V1Plugin } from "@opencode-ai/plugin";
import type { Plugin } from "@opencode/plugin";
import type { SessionContext } from "@opencode/plugin/promise/session";

const exec = promisify(execFile);
const THTS_MARKER = "<!-- thts-integration -->";
const THTS_HEADING = "# thts Integration Instructions";

function hasPolicy(content: string) {
  return content.includes(THTS_MARKER) || content.includes(THTS_HEADING);
}

function createPolicyLoader() {
  const policies = new Map<string, Promise<string | null>>();

  const run = async (directory: string, args: string[]) => {
    try {
      const { stdout } = await exec("thts", args, {
        cwd: directory,
        timeout: 10_000,
        encoding: "utf8",
      });
      return stdout.trim();
    } catch {
      return null;
    }
  };

  return async (directory: string) => {
    if ((await run(directory, ["init", "--check"])) === null) {
      policies.delete(directory);
      return null;
    }

    let pending = policies.get(directory);
    if (!pending) {
      pending = run(directory, ["agent-instructions"]);
      policies.set(directory, pending);
    }
    const content = await pending;
    if (!content) {
      if (policies.get(directory) === pending) policies.delete(directory);
      return null;
    }
    return `${THTS_MARKER}\n${content}`;
  };
}

const server: V1Plugin = async ({ directory }) => {
  const loadPolicy = createPolicyLoader();
  return {
    "experimental.chat.system.transform": async (_input, output) => {
      if (output.system.some(hasPolicy)) return;
      const content = await loadPolicy(directory);
      if (content && !output.system.some(hasPolicy))
        output.system.push(content);
    },
  };
};

async function setup(ctx: Plugin.Context) {
  const loadPolicy = createPolicyLoader();
  const inject = async (
    event: Pick<SessionContext, "sessionID" | "system">,
  ) => {
    if (event.system.some((part) => hasPolicy(part.text))) return;
    let directory: string;
    try {
      const session = await ctx.session.get({ sessionID: event.sessionID });
      directory = session.location.directory;
    } catch {
      return;
    }
    const content = await loadPolicy(directory);
    if (content && !event.system.some((part) => hasPolicy(part.text))) {
      event.system.push({ type: "text", text: content });
    }
  };
  await ctx.session.hook("context", inject);
  await ctx.session.hook("compaction", inject);
}

// V1 calls server(); V2 calls setup(). Plugin.define is an identity helper,
// so the structural definition avoids a runtime dependency on either SDK.
export default { id: "thts.integration", server, setup };
