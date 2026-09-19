import { afterEach, beforeEach, describe, expect, test } from "bun:test";
import { mkdtemp, mkdir, readFile, rm, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";

import plugin from "./thts-integration";

type Response = { exitCode: number; stdout?: string };
type Event = { sessionID: string; system: { type: "text"; text: string }[] };
const success = { exitCode: 0 };
const instructions = {
  exitCode: 0,
  stdout: "# thts Integration Instructions\n\nPolicy",
};

let root: string;
let oldPath: string | undefined;
beforeEach(async () => {
  root = await mkdtemp(join(tmpdir(), "thts-opencode-"));
  oldPath = process.env.PATH;
  await writeFile(
    join(root, "thts"),
    `#!${process.execPath}
const { readFileSync, writeFileSync, appendFileSync } = require("node:fs");
const responses = JSON.parse(readFileSync("responses.json", "utf8"));
const response = responses.shift();
appendFileSync("calls.txt", process.argv.slice(2).join(" ") + "\\n");
writeFileSync("responses.json", JSON.stringify(responses));
if (!response) process.exit(99);
process.stdout.write(response.stdout ?? "");
process.exit(response.exitCode);
`,
    { mode: 0o755 },
  );
  process.env.PATH = `${root}:${oldPath}`;
});
afterEach(async () => {
  process.env.PATH = oldPath;
  await rm(root, { recursive: true, force: true });
});

async function repo(name: string, responses: Response[]) {
  const directory = join(root, name);
  await mkdir(directory);
  await writeFile(join(directory, "responses.json"), JSON.stringify(responses));
  await writeFile(join(directory, "calls.txt"), "");
  return directory;
}
async function calls(directory: string) {
  return (await readFile(join(directory, "calls.txt"), "utf8"))
    .split("\n")
    .filter(Boolean);
}

async function v1(directory: string) {
  const hooks = await plugin.server({ directory } as never);
  const transform = hooks["experimental.chat.system.transform"]!;
  return {
    hooks,
    async inject(system: string[]) {
      await transform({} as never, { system });
    },
  };
}
async function v2(directory: () => string | Promise<string>) {
  const hooks = new Map<string, (event: Event) => Promise<void>>();
  const sessions: string[] = [];
  await plugin.setup({
    // Deliberately different from the session's directory.
    location: { directory: "/wrong-plugin-location" },
    session: {
      async get({ sessionID }: { sessionID: string }) {
        sessions.push(sessionID);
        return {
          location: { directory: await directory() },
          subpath: "ignored",
        };
      },
      async hook(name: string, callback: (event: Event) => Promise<void>) {
        hooks.set(name, callback);
      },
    },
  } as never);
  return { hooks, sessions };
}

for (const version of [1, 2]) {
  describe(`OpenCode v${version}`, () => {
    async function adapter(directory: string) {
      if (version === 1) return v1(directory);
      const instance = await v2(() => directory);
      return {
        async inject(system: string[]) {
          const event: Event = {
            sessionID: "session-a",
            system: system.map((text) => ({ type: "text", text })),
          };
          await instance.hooks.get("context")!(event);
          system.splice(
            0,
            system.length,
            ...event.system.map((part) => part.text),
          );
        },
      };
    }

    test("injects once across plugin instances and respects existing policy", async () => {
      const directory = await repo("repo", [success, instructions]);
      const first = await adapter(directory);
      const second = await adapter(directory);
      const system: string[] = ["Other instructions"];
      await first.inject(system);
      await second.inject(system);
      expect(system).toHaveLength(2);
      expect(system[1]).toContain("<!-- thts-integration -->");
      const legacy = ["# thts Integration Instructions\nLegacy policy"];
      await second.inject(legacy);
      expect(legacy).toHaveLength(1);
      expect(await calls(directory)).toEqual([
        "init --check",
        "agent-instructions",
      ]);
    });

    test("rechecks eligibility, caches policy, and invalidates on uninit", async () => {
      const directory = await repo("repo", [
        success,
        instructions,
        success,
        { exitCode: 1 },
        success,
        { exitCode: 0, stdout: "Changed policy" },
      ]);
      const instance = await adapter(directory);
      for (const expected of ["Policy", "Policy", null, "Changed policy"]) {
        const system: string[] = [];
        await instance.inject(system);
        if (expected) expect(system[0]).toContain(expected);
        else expect(system).toEqual([]);
      }
      expect(await calls(directory)).toEqual([
        "init --check",
        "agent-instructions",
        "init --check",
        "init --check",
        "init --check",
        "agent-instructions",
      ]);
    });

    test("recovers from check, render, and empty-output failures", async () => {
      const directory = await repo("repo", [
        { exitCode: 1 },
        success,
        { exitCode: 1 },
        success,
        { exitCode: 0, stdout: "   " },
        success,
        instructions,
      ]);
      const instance = await adapter(directory);
      for (let i = 0; i < 4; i++) {
        const system: string[] = [];
        await instance.inject(system);
        expect(system).toHaveLength(i === 3 ? 1 : 0);
      }
    });

    test("missing executable and missing cwd are quiet no-ops", async () => {
      const directory = await repo("repo", [success, instructions]);
      const instance = await adapter(directory);
      process.env.PATH = "";
      const system: string[] = [];
      await instance.inject(system);
      expect(system).toEqual([]);
      process.env.PATH = `${root}:${oldPath}`;
      await instance.inject(system);
      expect(system).toHaveLength(1);
      await (await adapter(join(root, "missing"))).inject([]);
    });
  });
}

test("v2 registers work and compaction only; compaction shares policy cache", async () => {
  const directory = await repo("repo", [success, instructions, success]);
  const instance = await v2(() => directory);
  expect([...instance.hooks.keys()]).toEqual(["context", "compaction"]);
  for (const hook of instance.hooks.values()) {
    const event: Event = { sessionID: "session-a", system: [] };
    await hook(event);
    expect(event.system).toEqual([
      {
        type: "text",
        text: `<!-- thts-integration -->\n${instructions.stdout}`,
      },
    ]);
  }
  expect(await calls(directory)).toEqual([
    "init --check",
    "agent-instructions",
    "init --check",
  ]);
});

test("v2 follows session moves and isolates directory caches", async () => {
  const first = await repo("first", [success, instructions, success]);
  const second = await repo("second", [
    success,
    { exitCode: 0, stdout: "Other policy" },
  ]);
  let directory = first;
  const instance = await v2(() => directory);
  for (const [cwd, expected] of [
    [first, "Policy"],
    [second, "Other policy"],
    [first, "Policy"],
  ]) {
    directory = cwd;
    const event: Event = { sessionID: "moving-session", system: [] };
    await instance.hooks.get("context")!(event);
    expect(event.system[0].text).toContain(expected);
  }
  expect(instance.sessions).toEqual(Array(3).fill("moving-session"));
  expect(await calls(first)).toEqual([
    "init --check",
    "agent-instructions",
    "init --check",
  ]);
  expect(await calls(second)).toEqual(["init --check", "agent-instructions"]);
});

test("v2 lookup failures are retryable and existing policy avoids lookup", async () => {
  const directory = await repo("repo", [success, instructions]);
  let fail = true;
  const instance = await v2(() => {
    if (fail) throw new Error("session unavailable");
    return directory;
  });
  const event: Event = { sessionID: "session-a", system: [] };
  await instance.hooks.get("context")!(event);
  expect(event.system).toEqual([]);
  fail = false;
  await instance.hooks.get("context")!(event);
  await instance.hooks.get("context")!(event);
  expect(event.system).toHaveLength(1);
  expect(instance.sessions).toHaveLength(2);
});
