# Getting started with buddy

Install buddy, connect to Jamf, and run a first command. New here? Start with the [About](index.md) page for the full docs map.

## 1. Install

```sh
make buddy
make install          # optional: copy unsigned CLI to ~/go/bin/buddy
make install-signed   # macOS: ~/Applications/Buddy.app + symlink ~/go/bin/buddy
```

## 2. Connect to Jamf

`--context` is a **workplace name** (buddy config), not a Jamf UUID. Jamf **scope** (`organization` / `environment` / `tenant`) lives on the **auth**. Optional nested `orgs:` YAML expands into the same workplaces; instance-only installs omit it. See [Auth](auth.md#contexts-and-auths).

If you have **no workplaces configured**, run any Jamf command on a terminal — buddy asks how to start (same as `buddy setup` / `buddy cred add`):

1. **Jamf Pro instance** — paste the API-client **JSON** Jamf Pro shows (oauth), or type the instance URL plus username/password (**basic**).
2. **Jamf Account / Platform** — paste the integration **curl**. Organization-admin is `https://us.api.jamfcloud.com` with a blank environment id (no env/tenant header). That workplace is the same object as `orgs.<name>`. Environment curls use a regional host **plus** an environment id. (Samples that show `api.jamfcloud.com` do not resolve; paste still treats them as org-against-US.)
3. **Quickstart file** — a secrets-free YAML (or ACS plist) with `orgs:` / `contexts:` / ids. Then fill secrets.
4. **Org MDM profile** already on this Mac (`virtualpete.buddy.cli`) — fill secrets only.

```sh
buddy policy list                     # chooser when config is empty
# or:
buddy setup
buddy cred add --paste --context=sandbox
buddy cred add --context=sandbox --url=https://example.jamfcloud.com --type=basic --username=admin
buddy setup import ./quickstart.yaml
pbpaste | buddy cred add --paste --context=sandbox -y
buddy auth test
```

Secrets go into the encrypted vault. In the local docs UI (`buddy serve` → `/config`), edit workplace/auth **metadata** after you have identity — `/config` is not first-run. Empty config: `buddy setup` in a terminal. Vault secrets stay in the terminal (`buddy cred set secret` / `buddy setup secrets`).

If this Mac already has an org MDM profile (or you imported a quickstart), skip identity and run:

```sh
buddy setup secrets                   # create missing auths; store client secrets / passwords
buddy cred status                     # org → environment → tenant coverage / gaps
buddy auth test
```

Admins share the same identity document two ways. From a working install:

```sh
buddy setup export ./quickstart.yaml           # YAML to mail around (no secrets)
buddy setup import ./quickstart.yaml           # recipients merge identity, then fill secrets
buddy setup managed-plist /tmp/buddy.plist     # same identity as Jamf ACS plist
```

Already have workplaces/auths? Use `buddy cred …` and `buddy auth test` — details in the [reference manual](reference-manual.md) and [Auth, secrets, and tokens](auth.md).

## 3. Try it

```sh
buddy policy list                     # or: group list, package list, …
buddy serve --open                    # docs + /config + /ide in the browser
buddy help                            # CLI overview
buddy help builtins                   # buddyscript API reference
```

In the IDE, open **Libraries** → **Getting started** for a short list of builtins to try.

## Next

| Guide | What it’s for |
|-------|----------------|
| [About](index.md) | Docs map: resources, Platform, guides |
| [Release notes](releases.md) | Notable changes per tagged version |
| [Auth, secrets, and tokens](auth.md) | Contexts, vault, channel policy |
| [Reference manual](reference-manual.md) | Config, CLI, buddyscript, env vars |
| [Script IDE](/ide) | Single-buffer runner |
| [BuddyScript overview](buddyscript.md) | Language, runners, conventions |
| [Function index](/docs/builtins) | Every builtin |
| [Command index](/docs/cli) | Every CLI command |
