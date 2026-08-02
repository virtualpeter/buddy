#!/usr/bin/env node
/**
 * Bundle the extension (+ vscode-languageclient) into out/extension.js.
 *
 * vscode-languageclient's Unix process killer loads terminateProcess.sh via
 * __dirname (the bundle directory for CJS output); copy that script next to
 * out/extension.js so process teardown works after packaging.
 */
const esbuild = require("esbuild");
const fs = require("fs");
const path = require("path");

const root = path.join(__dirname, "..");
const outfile = path.join(root, "out", "extension.js");
const watch = process.argv.includes("--watch");
const minify = process.argv.includes("--minify");

const terminateSrc = path.join(
  root,
  "node_modules",
  "vscode-languageclient",
  "lib",
  "node",
  "terminateProcess.sh"
);

function copyTerminateScript() {
  const dest = path.join(root, "out", "terminateProcess.sh");
  fs.mkdirSync(path.dirname(dest), { recursive: true });
  fs.copyFileSync(terminateSrc, dest);
}

async function main() {
  // Bundled CJS keeps runtime __dirname as out/; terminateProcess.sh is copied there.
  const options = {
    entryPoints: [path.join(root, "src", "extension.ts")],
    bundle: true,
    outfile,
    external: ["vscode"],
    format: "cjs",
    platform: "node",
    target: "node18",
    sourcemap: !minify,
    minify,
    logLevel: "info",
  };

  if (watch) {
    const ctx = await esbuild.context(options);
    await ctx.watch();
    copyTerminateScript();
    console.log("watching…");
    return;
  }

  await esbuild.build(options);
  copyTerminateScript();
}

main().catch((err) => {
  console.error(err);
  process.exit(1);
});
