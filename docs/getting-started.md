# Getting started with buddy

Install buddy, add a Jamf workplace, run a first command. Docs map: [About](index.md).

## macOS (recommended)

Install the **notarized** package from [GitHub Releases](https://github.com/virtualpeter/buddy/releases). That places **Buddy.app** in `/Applications` and `buddy` at `/usr/local/bin/buddy`. Building from source is for [contributors](../README.md#contributing).

1. Open **Buddy.app**. It starts the nested **Buddy Agent** (menu bar) and hosts Docs, Config, and IDE.
2. **Agent Settings…** (⇧⌘, or the toolbar gear): add a Jamf Pro or Account workplace and its secret. The Agent owns identity and secrets; the CLI does not write `config.yaml`.
3. Run `buddy policy list` (or use the IDE). Touch ID / the Agent unlock prompt appears when a token is minted.

Import instead of typing:

- Identity only: `buddy setup export --json` into the Agent’s import picker.
- Identity and secrets: a `buddy cred backup` file (the Agent decrypts it).

When the Agent owns config, Buddy.app hides the Config tab — edit workplaces in Agent Settings. Appearance is **View → Appearance**.

A running Agent with **no** workplaces is an error until you add one there. Linux and Windows have no Agent; use [CLI setup](#other-platforms) instead.

## Other platforms

Use the Linux or Windows binary from the same [Releases](https://github.com/virtualpeter/buddy/releases) page, or [build from source](../README.md#contributing).

With an empty config, any interactive Jamf command (or `buddy setup` / `buddy cred add`) opens a chooser: paste Jamf Pro API-client JSON or basic URL + user, paste an Account/Platform curl, import a secrets-free YAML/plist, or fill secrets for an existing MDM profile (`virtualpete.buddy.cli`).

```sh
buddy policy list
buddy cred add --paste --context=sandbox
buddy setup import ./quickstart.yaml
buddy setup secrets          # MDM / quickstart identity already present
buddy auth test
```

Secrets go in the encrypted vault. Recovery passphrase is **user-held** (typed for `cred backup` / `restore`). Details: [Auth](auth.md).

Share identity from a working install with `buddy setup export` (YAML) or `buddy setup managed-plist`. Recipients import, then `buddy setup secrets`.

## Try it

```sh
buddy policy list
buddy serve --open            # same Docs / IDE as Buddy.app, in the browser
buddy help
buddy help builtins
```

In the IDE, **Libraries → Getting started** lists builtins to try.

## Next

| Guide | What it’s for |
|-------|----------------|
| [About](index.md) | Docs map |
| [Release notes](releases.md) | Changes per tagged version |
| [Auth, secrets, and tokens](auth.md) | Contexts, vault, Buddy Agent, channel policy |
| [Reference manual](reference-manual.md) | Config, CLI, buddyscript, env vars |
| [Script IDE](/ide) | Single-buffer runner |
| [BuddyScript overview](buddyscript.md) | Language, runners, conventions |
| [Function index](/docs/builtins) | Every builtin |
| [Command index](/docs/cli) | Every CLI command |
