#!/usr/bin/env node
/**
 * Package the buddy VS Code/Cursor extension as a VSIX.
 * Publisher is configurable at build time:
 *
 *   PUBLISHER=virtualpete npm run package
 *
 * Default publisher: virtualpete (extension id virtualpete.buddy).
 */
const fs = require("fs");
const path = require("path");
const { spawnSync } = require("child_process");

const root = path.join(__dirname, "..");
const pkgPath = path.join(root, "package.json");

const publisher = (process.env.PUBLISHER || "virtualpete").trim();
if (!publisher) {
  console.error("PUBLISHER must be a non-empty string");
  process.exit(1);
}

const originalText = fs.readFileSync(pkgPath, "utf8");
const pkg = JSON.parse(originalText);
pkg.publisher = publisher;
fs.writeFileSync(pkgPath, JSON.stringify(pkg, null, 2) + "\n");

console.log(`Packaging extension as ${publisher}.${pkg.name}`);

let exitCode = 0;
try {
  const compile = spawnSync("npm", ["run", "compile"], { cwd: root, stdio: "inherit", shell: true });
  if (compile.status !== 0) {
    exitCode = compile.status || 1;
  } else {
    const vsce = spawnSync(
      "npx",
      ["vsce", "package", "--allow-missing-repository"],
      { cwd: root, stdio: "inherit", shell: true }
    );
    if (vsce.status !== 0) {
      exitCode = vsce.status || 1;
    } else {
      const vsixName = `${pkg.name}-${pkg.version}.vsix`;
      const vsixPath = path.join(root, vsixName);
      console.log(`Created ${vsixPath} (extension id ${publisher}.${pkg.name})`);
    }
  }
} finally {
  fs.writeFileSync(pkgPath, originalText);
}

process.exit(exitCode);
