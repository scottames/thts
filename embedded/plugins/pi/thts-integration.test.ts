import { describe, expect, test } from "bun:test";

import thtsIntegration from "./thts-integration";

type Result = { code: number; stdout?: string };
type Response =
  | Result
  | Error
  | undefined
  | null
  | { code?: number; stdout?: unknown };

const policy = "# thts Integration Instructions\n\nPolicy";

function createPlugin(responses: Response[]) {
  const calls: Array<{
    command: string;
    args: string[];
    cwd: string;
    signal: AbortSignal | undefined;
    timeout: number;
  }> = [];
  const registrations: string[] = [];
  let handler:
    | ((
        event: { systemPrompt: string },
        ctx: { cwd: string; signal?: AbortSignal },
      ) => Promise<unknown>)
    | undefined;

  const pi = {
    on(event: string, callback: typeof handler) {
      registrations.push(event);
      handler = callback;
    },
    async exec(
      command: string,
      args: string[],
      options: Omit<(typeof calls)[number], "command" | "args">,
    ) {
      calls.push({ command, args, ...options });
      if (responses.length === 0) {
        throw new Error(`unexpected command: ${command} ${args.join(" ")}`);
      }
      const response = responses.shift();
      if (response instanceof Error) {
        throw response;
      }
      return response as never;
    },
  };

  thtsIntegration(pi as never);

  if (!handler) {
    throw new Error("missing before_agent_start handler");
  }

  return {
    calls,
    registrations,
    run: (systemPrompt = "Base policy", cwd = "/repo", signal?: AbortSignal) =>
      handler({ systemPrompt }, { cwd, signal }),
  };
}

describe("Pi thts integration", () => {
  test("default factory registers only before_agent_start", () => {
    const registrations: string[] = [];

    thtsIntegration({
      on: (event: string) => registrations.push(event),
      exec: async () => ({ code: 0, stdout: "" }),
    } as never);

    expect(registrations).toEqual(["before_agent_start"]);
  });

  test("registers only before_agent_start and chains one marked policy block", async () => {
    const plugin = createPlugin([{ code: 0 }, { code: 0, stdout: policy }]);
    const signal = new AbortController().signal;

    const result = await plugin.run("Existing policy", "/repo", signal);

    expect(plugin.registrations).toEqual(["before_agent_start"]);
    expect(result).toEqual({
      systemPrompt: `Existing policy\n\n<!-- thts-integration -->\n${policy}`,
    });
    expect(plugin.calls).toEqual([
      {
        command: "thts",
        args: ["init", "--check"],
        cwd: "/repo",
        signal,
        timeout: 10_000,
      },
      {
        command: "thts",
        args: ["agent-instructions"],
        cwd: "/repo",
        signal,
        timeout: 10_000,
      },
    ]);
  });

  test("rechecks eligibility while caching instructions", async () => {
    const plugin = createPlugin([
      { code: 0 },
      { code: 0, stdout: policy },
      { code: 0 },
    ]);

    await plugin.run();
    await plugin.run();

    expect(plugin.calls.map(({ args }) => args)).toEqual([
      ["init", "--check"],
      ["agent-instructions"],
      ["init", "--check"],
    ]);
  });

  test("avoids duplicate extensions and pre-existing canonical headings", async () => {
    const first = createPlugin([{ code: 0 }, { code: 0, stdout: policy }]);
    const second = createPlugin([]);

    const injected = await first.run();
    const chained = await second.run(
      (injected as { systemPrompt: string }).systemPrompt,
    );
    const heading = await second.run(
      "# thts Integration Instructions\n\nExisting policy",
    );

    expect(chained).toBeUndefined();
    expect(heading).toBeUndefined();
    expect(second.calls).toEqual([]);
  });

  test("quietly recovers from command failures and blank instructions", async () => {
    for (const failure of [new Error("missing thts"), { code: 1 }]) {
      const plugin = createPlugin([
        failure,
        { code: 0 },
        { code: 0, stdout: policy },
      ]);
      expect(await plugin.run()).toBeUndefined();
      expect(await plugin.run()).toEqual({
        systemPrompt: `Base policy\n\n<!-- thts-integration -->\n${policy}`,
      });
    }

    const instructionsFailure = createPlugin([
      { code: 0 },
      { code: 1 },
      { code: 0 },
      { code: 0, stdout: policy },
    ]);
    expect(await instructionsFailure.run()).toBeUndefined();
    expect(await instructionsFailure.run()).toEqual({
      systemPrompt: `Base policy\n\n<!-- thts-integration -->\n${policy}`,
    });

    const blank = createPlugin([
      { code: 0 },
      { code: 0, stdout: "   " },
      { code: 0 },
      { code: 0, stdout: policy },
    ]);
    expect(await blank.run()).toBeUndefined();
    expect(await blank.run()).toEqual({
      systemPrompt: `Base policy\n\n<!-- thts-integration -->\n${policy}`,
    });
  });

  test("quietly retries after undefined or malformed init results", async () => {
    for (const malformed of [undefined, null, { code: "0" }]) {
      const plugin = createPlugin([
        malformed,
        { code: 0 },
        { code: 0, stdout: policy },
      ]);

      expect(await plugin.run()).toBeUndefined();
      expect(await plugin.run()).toEqual({
        systemPrompt: `Base policy\n\n<!-- thts-integration -->\n${policy}`,
      });
    }
  });

  test("quietly retries after undefined or malformed instruction results", async () => {
    for (const malformed of [undefined, { code: 0, stdout: null }]) {
      const plugin = createPlugin([
        { code: 0 },
        malformed,
        { code: 0 },
        { code: 0, stdout: policy },
      ]);

      expect(await plugin.run()).toBeUndefined();
      expect(await plugin.run()).toEqual({
        systemPrompt: `Base policy\n\n<!-- thts-integration -->\n${policy}`,
      });
    }
  });

  test("quietly retries after cancellation and timeout", async () => {
    const plugin = createPlugin([
      new Error("cancelled"),
      new Error("timeout"),
      { code: 0 },
      { code: 0, stdout: policy },
    ]);
    const cancelled = new AbortController();
    cancelled.abort();

    expect(
      await plugin.run("Base policy", "/repo", cancelled.signal),
    ).toBeUndefined();
    expect(await plugin.run()).toBeUndefined();
    expect(await plugin.run()).toEqual({
      systemPrompt: `Base policy\n\n<!-- thts-integration -->\n${policy}`,
    });
    expect(plugin.calls.every(({ timeout }) => timeout === 10_000)).toBe(true);
  });

  test("clears cached policy when initialization is removed and restored", async () => {
    const changedPolicy = "# thts Integration Instructions\n\nChanged policy";
    const plugin = createPlugin([
      { code: 0 },
      { code: 0, stdout: policy },
      { code: 1 },
      { code: 0 },
      { code: 0, stdout: changedPolicy },
    ]);

    await plugin.run();
    expect(await plugin.run()).toBeUndefined();
    expect(await plugin.run()).toEqual({
      systemPrompt: `Base policy\n\n<!-- thts-integration -->\n${changedPolicy}`,
    });
  });

  test("reuses validated policies when returning to a working directory", async () => {
    const plugin = createPlugin([
      { code: 0 },
      { code: 0, stdout: "# thts Integration Instructions\n\nA" },
      { code: 0 },
      { code: 0, stdout: "# thts Integration Instructions\n\nB" },
      { code: 0 },
    ]);

    expect(await plugin.run("Base policy", "/a")).toEqual({
      systemPrompt:
        "Base policy\n\n<!-- thts-integration -->\n# thts Integration Instructions\n\nA",
    });
    expect(await plugin.run("Base policy", "/b")).toEqual({
      systemPrompt:
        "Base policy\n\n<!-- thts-integration -->\n# thts Integration Instructions\n\nB",
    });
    expect(await plugin.run("Base policy", "/a")).toEqual({
      systemPrompt:
        "Base policy\n\n<!-- thts-integration -->\n# thts Integration Instructions\n\nA",
    });
    expect(plugin.calls.map(({ cwd }) => cwd)).toEqual([
      "/a",
      "/a",
      "/b",
      "/b",
      "/a",
    ]);
    expect(plugin.calls.map(({ args }) => args)).toEqual([
      ["init", "--check"],
      ["agent-instructions"],
      ["init", "--check"],
      ["agent-instructions"],
      ["init", "--check"],
    ]);
  });
});
