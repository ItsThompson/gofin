import { describe, it, expect } from "vitest";
import { readFileSync } from "node:fs";
import { join } from "node:path";

// The image build and the CD workflow carry constraints that only fail in
// production: a stripped package the server SDK imports at boot, a source map
// shipped to users, or a release name that splits one deploy across two. Nothing
// in this suite executes either file, so both are read as text, the way
// server-entry.test.ts pins server.js's import order.
const repoRoot = join(process.cwd(), "../../..");
const dockerfile = readFileSync(join(repoRoot, "frontend/Dockerfile"), "utf8");
const cdWorkflow = readFileSync(
  join(repoRoot, ".github/workflows/cd.yml"),
  "utf8",
);
const ciWorkflow = readFileSync(
  join(repoRoot, ".github/workflows/ci.yml"),
  "utf8",
);

/** Source lines with comment lines removed. */
const withoutComments = (source: string): string =>
  source
    .split("\n")
    .filter((line) => !line.trim().startsWith("#"))
    .join("\n");

/** One Dockerfile stage with its comment lines removed. */
const stageCommands = (name: string): string => {
  const stage = dockerfile
    .split(/^FROM /m)
    .find((section) => new RegExp(`^\\S+ AS ${name}\\b`).test(section));
  if (!stage) throw new Error(`frontend/Dockerfile has no stage "${name}"`);
  return withoutComments(stage);
};

/** One workflow step, up to the next step at the same indentation. */
const workflowStep = (name: string): string => {
  const start = cdWorkflow.indexOf(`- name: ${name}\n`);
  if (start === -1) throw new Error(`cd.yml has no step "${name}"`);
  const rest = cdWorkflow.slice(start);
  const end = rest.indexOf("\n      - name: ");
  return end === -1 ? rest : rest.slice(0, end);
};

const builder = stageCommands("builder");
const runner = stageCommands("runner");

/** The runner stage's `rm -rf` invocation, including its continuation lines. */
const strip = (): string => {
  const start = runner.indexOf("rm -rf");
  if (start === -1) throw new Error("the runner stage strips nothing");
  const lines = runner.slice(start).split("\n");
  const last = lines.findIndex((line) => !line.trimEnd().endsWith("\\"));
  return lines.slice(0, last + 1).join("\n");
};

describe("the source-map upload in the builder stage", () => {
  it("injects debug IDs into the bundle it just built, then uploads", () => {
    // Injection mutates the .js files, so the maps must be uploaded from the
    // build that ships. Uploading before injecting keys the maps to nothing.
    const build = builder.indexOf("turbo build");
    const inject = builder.indexOf("sourcemaps inject");
    const upload = builder.indexOf("sourcemaps upload");

    expect(build).toBeGreaterThan(-1);
    expect(inject).toBeGreaterThan(build);
    expect(upload).toBeGreaterThan(inject);
  });

  it("targets the browser bundle and not the whole build tree", () => {
    // build/server holds the SSR bundle. Uploading it would double the upload
    // and mix two artifact sets under one release name.
    expect(builder).toContain("sourcemaps inject apps/shell/build/client");
    expect(builder).toContain("sourcemaps upload apps/shell/build/client");
    expect(builder).not.toMatch(
      /sourcemaps (?:inject|upload) apps\/shell\/build(?!\/client)/,
    );
  });

  it("proves sentry-cli resolves before using it", () => {
    // @sentry/cli arrives hoisted to the root node_modules as a transitive
    // dependency, so a hoisting change must fail at the resolution boundary
    // with a clear message rather than opaquely inside the upload.
    const version = builder.indexOf("sentry-cli --version");
    expect(version).toBeGreaterThan(-1);
    expect(version).toBeLessThan(builder.indexOf("sourcemaps inject"));
    expect(builder).not.toContain("node_modules/.bin/sentry-cli");
  });

  it("takes the auth token from a secret mount, never an ARG", () => {
    // ARG values are recorded in the image history and readable by anyone who
    // can pull the image.
    expect(builder).toContain("--mount=type=secret,id=sentry_auth_token");
    expect(dockerfile).not.toMatch(/^\s*ARG\s+SENTRY_AUTH_TOKEN/m);
  });

  it("is a no-op when no token is mounted", () => {
    // A local `docker compose build` and the CI and E2E builds pass no secret,
    // and all three must still succeed.
    expect(builder).not.toContain("required=true");
    expect(builder).toContain("if [ -s /run/secrets/sentry_auth_token ]");
    expect(builder).toMatch(/else \\\n\s*echo "No sentry_auth_token secret/);
  });

  it("deletes the maps after the upload, not before it", () => {
    // The runner stage copies this tree wholesale, so deleting the maps here is
    // what keeps the original TypeScript out of the shipped image's layers. It
    // has to come after the upload, or Sentry receives nothing.
    const upload = builder.indexOf("sourcemaps upload");
    const deleted = builder.indexOf('find apps/shell/build -name "*.map" -delete');

    expect(deleted).toBeGreaterThan(upload);
  });
});

describe("the runtime image", () => {
  const stripped = [...strip().matchAll(/node_modules\/\S+/g)].map(
    (match) => match[0],
  );

  it("strips both sentry-cli binary locations", () => {
    // getBinaryPath() checks node_modules/@sentry/cli/sentry-cli first and
    // returns it when present, so removing only the platform package leaves a
    // second 21 MB copy behind.
    expect(stripped).toContain("node_modules/@sentry/cli-linux-x64");
    expect(stripped).toContain("node_modules/@sentry/cli/sentry-cli");
  });

  it("keeps the two packages the server SDK imports at boot", () => {
    // @sentry/react-router's node entry statically re-exports sentryOnBuildEnd
    // and sentryReactRouter, which import @sentry/cli and @sentry/vite-plugin.
    // Removing either tree makes the first import of server.js throw
    // ERR_MODULE_NOT_FOUND.
    expect(stripped).not.toContain("node_modules/@sentry/cli");
    expect(stripped).not.toContain("node_modules/@sentry/vite-plugin");
  });

  it("skips the sentry-cli binary download on this stage only", () => {
    // The postinstall downloads from downloads.sentry-cdn.com and exits
    // non-zero on failure, which is a build-failure mode an image that never
    // runs the CLI does not need. The installer stage must keep the download
    // path, because the source-map step needs a working binary.
    expect(runner).toContain("SENTRYCLI_SKIP_DOWNLOAD=1 npm ci --omit=dev");
    expect(stageCommands("installer")).not.toContain("SENTRYCLI_SKIP_DOWNLOAD");
    expect(builder).not.toContain("SENTRYCLI_SKIP_DOWNLOAD");
  });

  it("deletes every source map, and does so after the build arrives", () => {
    // Maps contain the original TypeScript. Rooting the delete at /app also
    // covers the maps some production dependencies ship. Above the COPY it
    // would run against a tree that does not exist yet, and the maps would ship
    // with every assertion in this file still green.
    const copyBuild = runner.indexOf(
      "COPY --from=builder /app/apps/shell/build",
    );
    const deleteMaps = runner.indexOf('find /app -name "*.map" -delete');

    expect(copyBuild).toBeGreaterThan(-1);
    expect(deleteMaps).toBeGreaterThan(copyBuild);
  });
});

describe("the CD workflow", () => {
  it("builds the bundle and uploads the maps against one release name", () => {
    // sentry-cli keys the maps to the exact release string the bundle was built
    // with, so both have to read the same declaration. Re-inlining the prefix
    // at either site is how one deploy ends up split across two releases.
    expect(cdWorkflow).toContain("SENTRY_RELEASE_WEB: gofin-web@${{ github.sha }}");
    expect(cdWorkflow).toContain("SENTRY_RELEASE_API: gofin-api@${{ github.sha }}");
    expect(workflowStep("Build and push ${{ matrix.service }}")).toContain(
      "VITE_SENTRY_RELEASE=${{ env.SENTRY_RELEASE_WEB }}",
    );
  });

  it("mounts the auth token into the image build", () => {
    expect(workflowStep("Build and push ${{ matrix.service }}")).toContain(
      "sentry_auth_token=${{ secrets.SENTRY_AUTH_TOKEN }}",
    );
  });

  it("names the org that owns both projects", () => {
    // Inferring the org from the project names yields `gofin`, which 404s at
    // release create and at the upload.
    expect(cdWorkflow).toContain("SENTRY_ORG: t-industries");
    expect(cdWorkflow).toContain("SENTRY_PROJECT_FRONTEND: gofin-frontend");
    expect(cdWorkflow).toContain("SENTRY_PROJECT_BACKEND: gofin-backend");
  });

  it("sets no SENTRY_URL, because the org is in the US region", () => {
    // US is sentry-cli's default host. An EU-shaped override breaks every call.
    expect(withoutComments(cdWorkflow)).not.toContain("SENTRY_URL");
  });

  it("creates both releases before the deploy and finalizes after it", () => {
    const create = cdWorkflow.indexOf("- name: Create Sentry releases");
    const deploy = cdWorkflow.indexOf("- name: Deploy to VPS");
    const finalize = cdWorkflow.indexOf("- name: Finalize Sentry releases");

    expect(create).toBeGreaterThan(-1);
    expect(deploy).toBeGreaterThan(create);
    expect(finalize).toBeGreaterThan(deploy);

    const created = workflowStep("Create Sentry releases");
    const finalized = workflowStep("Finalize Sentry releases");
    for (const step of [created, finalized]) {
      expect(step).toContain('--project "${SENTRY_PROJECT_BACKEND}" "${SENTRY_RELEASE_API}"');
      expect(step).toContain('--project "${SENTRY_PROJECT_FRONTEND}" "${SENTRY_RELEASE_WEB}"');
    }
  });

  it("leaves both releases unfinalized when the deploy fails", () => {
    // No `if:` on the finalize step, so it inherits the default success()
    // condition and the recorded window matches the deploy that happened.
    expect(workflowStep("Finalize Sentry releases")).not.toMatch(/^\s+if:/m);
  });
});

describe("the CI workflow", () => {
  it("mentions nothing Sentry-related", () => {
    // CI copies .env.example to .env and builds the whole stack on every branch
    // push, and its E2E suite drives a deliberate 500. One DSN or one upload
    // here would spend the org-wide monthly allowance on branch noise.
    expect(withoutComments(ciWorkflow).toLowerCase()).not.toContain("sentry");
  });
});
