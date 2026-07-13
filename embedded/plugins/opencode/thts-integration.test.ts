import { describe, expect, test } from "bun:test";

import { ThtsIntegration } from "./thts-integration";

type Response = {
  exitCode: number;
  stdout?: string;
};

function createShell(responses: Response[]) {
  const calls: string[] = [];
  const directories: string[] = [];
  const shell = (strings: TemplateStringsArray) => {
    const command = strings.join("");
    calls.push(command);
    const response = responses.shift();
    if (!response) {
      throw new Error(`unexpected command: ${command}`);
    }

    const output = {
      exitCode: response.exitCode,
      text: () => response.stdout ?? "",
    };
    const result = Object.assign(Promise.resolve(output), {
      cwd: (directory: string) => {
        directories.push(directory);
        return result;
      },
      quiet: () => result,
      nothrow: () => result,
    });
    return result;
  };

  return { calls, directories, shell };
}

async function createPlugin(responses: Response[]) {
  const fake = createShell(responses);
  const hooks = await ThtsIntegration({
    directory: "/repo",
    $: fake.shell,
  } as never);
  const transform = hooks["experimental.chat.system.transform"];
  if (!transform) {
    throw new Error("missing system transform hook");
  }
  return { ...fake, hooks, transform };
}

const success = { exitCode: 0 };
const instructions = {
  exitCode: 0,
  stdout: "# thts Integration Instructions\n\nPolicy",
};

describe("OpenCode thts integration", () => {
  test("injects once across duplicate plugin instances", async () => {
    const first = await createPlugin([success, instructions]);
    const second = await createPlugin([]);
    const output = { system: [] as string[] };

    await first.transform({} as never, output);
    await second.transform({} as never, output);

    expect(output.system).toHaveLength(1);
    expect(output.system[0]).toContain("<!-- thts-integration -->");
    expect(first.calls).toEqual([
      "thts init --check",
      "thts agent-instructions",
    ]);
    expect(first.directories).toEqual(["/repo", "/repo"]);
    expect(second.calls).toEqual([]);

    const legacyOutput = {
      system: ["# thts Integration Instructions\n\nLegacy policy"],
    };
    await second.transform({} as never, legacyOutput);
    expect(legacyOutput.system).toHaveLength(1);
  });

  test("rechecks eligibility but reuses instructions for later requests", async () => {
    const plugin = await createPlugin([success, instructions, success]);

    const first = { system: [] as string[] };
    await plugin.transform({} as never, first);
    const afterCompaction = { system: [] as string[] };
    await plugin.transform({} as never, afterCompaction);

    expect(first.system).toHaveLength(1);
    expect(afterCompaction.system).toHaveLength(1);
    expect(plugin.calls).toEqual([
      "thts init --check",
      "thts agent-instructions",
      "thts init --check",
    ]);
    expect(plugin.hooks["experimental.session.compacting"]).toBeUndefined();
  });

  test("recovers after initialization or instruction failures", async () => {
    const afterInit = await createPlugin([
      { exitCode: 1 },
      success,
      instructions,
    ]);
    const disabled = { system: [] as string[] };
    await afterInit.transform({} as never, disabled);
    const enabled = { system: [] as string[] };
    await afterInit.transform({} as never, enabled);

    expect(disabled.system).toEqual([]);
    expect(enabled.system).toHaveLength(1);

    const afterCommand = await createPlugin([
      success,
      { exitCode: 1 },
      success,
      instructions,
    ]);
    const failed = { system: [] as string[] };
    await afterCommand.transform({} as never, failed);
    const recovered = { system: [] as string[] };
    await afterCommand.transform({} as never, recovered);

    expect(failed.system).toEqual([]);
    expect(recovered.system).toHaveLength(1);

    const afterBlank = await createPlugin([
      success,
      { exitCode: 0, stdout: "   " },
      success,
      instructions,
    ]);
    const blank = { system: [] as string[] };
    await afterBlank.transform({} as never, blank);
    const afterBlankRetry = { system: [] as string[] };
    await afterBlank.transform({} as never, afterBlankRetry);

    expect(blank.system).toEqual([]);
    expect(afterBlankRetry.system).toHaveLength(1);
  });

  test("stops injecting after the project is uninitialized", async () => {
    const changedInstructions = {
      exitCode: 0,
      stdout: "# thts Integration Instructions\n\nChanged policy",
    };
    const plugin = await createPlugin([
      success,
      instructions,
      { exitCode: 1 },
      success,
      changedInstructions,
    ]);

    const initialized = { system: [] as string[] };
    await plugin.transform({} as never, initialized);
    const uninitialized = { system: [] as string[] };
    await plugin.transform({} as never, uninitialized);
    const reinitialized = { system: [] as string[] };
    await plugin.transform({} as never, reinitialized);

    expect(initialized.system).toHaveLength(1);
    expect(uninitialized.system).toEqual([]);
    expect(reinitialized.system[0]).toContain("Changed policy");
  });
});
