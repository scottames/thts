// Actual host verification, with isolated homes and a local mock model (no credentials).
// Usage: bun scripts/verify-opencode-hosts.ts /path/to/v1.18.29 /path/to/v2.0.6
import assert from "node:assert/strict";
import { mkdtemp, mkdir, rm, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join, resolve } from "node:path";

const binaries = process.argv.slice(2).map((path) => resolve(path));
assert.equal(binaries.length, 2, "Provide v1.18.29 and v2.0.6 binary paths");
const root = await mkdtemp(join(tmpdir(), "thts-opencode-hosts-"));
const thts = join(root, "thts");
type ModelRequest = { messages: { role: string; content: unknown }[] };
const requests: ModelRequest[] = [];
const model = Bun.serve({
  hostname: "127.0.0.1",
  port: 0,
  async fetch(request) {
    requests.push(await request.json());
    const chunk = {
      id: "fixture",
      object: "chat.completion.chunk",
      created: 1,
      model: "test",
      choices: [
        {
          index: 0,
          delta: { role: "assistant", content: "Verified." },
          finish_reason: null,
        },
      ],
    };
    const finish = {
      ...chunk,
      choices: [{ index: 0, delta: {}, finish_reason: "stop" }],
      usage: { prompt_tokens: 100, completion_tokens: 2, total_tokens: 102 },
    };
    return new Response(
      `data: ${JSON.stringify(chunk)}\n\ndata: ${JSON.stringify(finish)}\n\ndata: [DONE]\n\n`,
      {
        headers: { "content-type": "text/event-stream" },
      },
    );
  },
});

function run(command: string[], cwd: string, env = process.env) {
  const result = Bun.spawnSync(command, {
    cwd,
    env,
    stdout: "pipe",
    stderr: "pipe",
  });
  assert.equal(
    result.exitCode,
    0,
    `${command.join(" ")}: ${result.stderr.toString()}`,
  );
  return result.stdout.toString();
}

async function until(check: () => Promise<boolean>, label: string) {
  for (let attempt = 0; attempt < 150; attempt++) {
    if (await check()) return;
    await Bun.sleep(200);
  }
  throw new Error(`Timed out: ${label}`);
}

try {
  run(
    ["go", "build", "-o", thts, "./cmd/thts"],
    resolve(import.meta.dir, ".."),
  );
  for (const [index, binary] of binaries.entries()) {
    const v2 = index === 1;
    const version = v2 ? "2.0.6" : "1.18.29";
    assert.equal(
      run([binary, "--version"], root).trim(),
      v2 ? `opencode v${version}` : version,
    );
    const base = join(root, version);
    const bin = join(base, "bin");
    const home = join(base, "home");
    const projects = [join(base, "overlap"), join(base, "global-only")];
    for (const directory of [bin, home, ...projects])
      await mkdir(directory, { recursive: true });
    await writeFile(
      join(bin, "thts"),
      `#!${process.execPath}
if (process.argv.slice(2).join(" ") === "agent-instructions") {
  console.log("# thts Integration Instructions\\nHOST-POLICY:" + process.cwd());
}
`,
      { mode: 0o755 },
    );
    // Do not inherit credentials, user config overrides, or external skills.
    const env = {
      HOME: home,
      PATH: `${bin}:/usr/bin:/bin`,
      XDG_CONFIG_HOME: join(base, "config"),
      XDG_DATA_HOME: join(base, "data"),
      XDG_CACHE_HOME: join(base, "cache"),
      XDG_STATE_HOME: join(base, "state"),
      THTS_CONFIG_PATH: join(base, "thts.yaml"),
      OPENCODE_SERVER_PASSWORD: "fixture-only",
      OPENCODE_DISABLE_DEFAULT_PLUGINS: "1",
      OPENCODE_DISABLE_EXTERNAL_SKILLS: "1",
      OPENCODE_DISABLE_UPDATE_CHECK: "1",
      OPENCODE_DISABLE_MODELS_FETCH: "1",
      DO_NOT_TRACK: "1",
      CI: "true",
      GIT_CONFIG_GLOBAL: "/dev/null",
      GIT_CONFIG_NOSYSTEM: "1",
    };
    const modelInfo = {
      name: "Test",
      limit: { context: 128000, output: 1000 },
    };
    const settings = {
      baseURL: `http://127.0.0.1:${model.port}/v1`,
      apiKey: "fixture",
    };
    const config = v2
      ? {
          model: "test/test",
          providers: {
            test: {
              name: "Test",
              package: "@opencode/ai/providers/openai-compatible",
              settings,
              models: { test: modelInfo },
            },
          },
        }
      : {
          model: "test/test",
          small_model: "test/test",
          provider: {
            test: {
              name: "Test",
              npm: "@ai-sdk/openai-compatible",
              options: settings,
              models: { test: modelInfo },
            },
          },
        };
    for (const directory of projects) {
      run(["git", "init", "-q"], directory, env);
      await writeFile(join(directory, "opencode.json"), JSON.stringify(config));
    }
    run([thts, "init", "agents", "--agents", "opencode"], projects[0], env);
    run(
      [thts, "init", "agents", "--agents", "opencode", "--global=all"],
      projects[0],
      env,
    );

    // Let the OS choose a port; parse only the address, never server log credentials.
    const child = Bun.spawn(
      [binary, "serve", "--hostname", "127.0.0.1", "--port", "0"],
      {
        cwd: projects[0],
        env,
        stdout: "pipe",
        stderr: "pipe",
      },
    );
    const stderr = new Response(child.stderr).text();
    let address = "";
    const output = (async () => {
      for await (const chunk of child.stdout) {
        address ||=
          new TextDecoder()
            .decode(chunk)
            .match(/http:\/\/127\.0\.0\.1:\d+/)?.[0] ?? "";
      }
    })();
    try {
      await until(async () => Boolean(address), `${version} server startup`);
      async function api(path: string, directory: string, body?: unknown) {
        const response = await fetch(`${address}${path}`, {
          method: body === undefined ? "GET" : "POST",
          headers: {
            "content-type": "application/json",
            "x-opencode-directory": directory,
            authorization: `Basic ${btoa("opencode:fixture-only")}`,
          },
          body: body === undefined ? undefined : JSON.stringify(body),
          signal: AbortSignal.timeout(30_000),
        });
        const text = await response.text();
        assert.ok(
          response.ok,
          `${version} ${path}: ${response.status} ${text}`,
        );
        const parsed = text ? JSON.parse(text) : undefined;
        return v2 ? parsed?.data : parsed;
      }
      const prefix = v2 ? "/api" : "";
      for (const directory of projects) {
        if (v2) {
          await until(
            async () =>
              (await api("/api/plugin", directory)).some(
                (entry: { id: string; state: { status: string } }) =>
                  entry.id === "thts.integration" &&
                  entry.state.status === "active",
              ),
            "thts plugin activation",
          );
        }
        for (const [kind, names] of [
          ["agent", ["thoughts-locator", "thoughts-analyzer"]],
          ["skill", ["thts-integrate"]],
          ["command", ["thts-handoff", "thts-resume"]],
        ] as const) {
          const resources = await api(`${prefix}/${kind}`, directory);
          for (const name of names)
            assert.ok(
              resources.some(
                (entry: { id?: string; name: string }) =>
                  entry.id === name || entry.name === name,
              ),
              `${version}: missing ${name}`,
            );
        }
        const session = await api(`${prefix}/session`, directory, {
          title: "Verification",
          model: { providerID: "test", id: "test" },
          ...(v2 ? { location: { directory } } : {}),
        });
        for (const compact of [false, true]) {
          const before = requests.length;
          const operation = compact
            ? v2
              ? "compact"
              : "summarize"
            : v2
              ? "prompt"
              : "message";
          const body = compact
            ? v2
              ? {}
              : { providerID: "test", modelID: "test" }
            : v2
              ? { text: "Say verified." }
              : {
                  parts: [{ type: "text", text: "Say verified." }],
                  model: { providerID: "test", modelID: "test" },
                };
          await api(
            `${prefix}/session/${session.id}/${operation}`,
            directory,
            body,
          );
          if (v2)
            await api(
              `/api/experimental/session/${session.id}/wait`,
              directory,
              {},
            );
          assert.ok(
            requests.length > before,
            `${version}: ${operation} made no model request`,
          );
          for (const request of requests.slice(before)) {
            const system = JSON.stringify(
              request.messages.filter(
                (message) =>
                  message.role === "system" || message.role === "developer",
              ),
            );
            assert.equal(
              system.split("<!-- thts-integration -->").length - 1,
              1,
              `${version}: ${operation} policy count`,
            );
            assert.ok(
              system.includes(`HOST-POLICY:${directory}`),
              `${version}: wrong project policy`,
            );
          }
        }
        console.log(
          `PASS: ${version} ${directory === projects[0] ? "global/project overlap" : "global-only second project"}: resources, prompt, compaction`,
        );
      }
    } finally {
      child.kill();
      await child.exited;
      await output;
      const errors = await stderr;
      assert.ok(
        !errors.includes("failed to load plugin"),
        `${version}: plugin load failure`,
      );
    }
  }
} finally {
  model.stop(true);
  await rm(root, { recursive: true, force: true });
}
