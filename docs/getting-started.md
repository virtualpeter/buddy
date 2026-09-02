# Getting started with buddy

Install buddy, connect to Jamf, and run a first command. New here? Start with the [About](index.md) page for the full docs map.

## 1. Install

```sh
make buddy
make install    # optional: ~/go/bin/buddy
```

## 2. Connect to Jamf

If you have **no tenants configured**, the easiest path is to run any Jamf command on a terminal — buddy prompts for the first tenant and auth (same flow as `buddy cred add`):

```sh
buddy policy list                     # interactive wizard when config is empty
# or explicitly:
buddy cred add
```

You will be asked for a **tenant name**, optional Jamf Pro instance URL, optional Platform region + tenant id, then auth type and credentials. Secrets go into the encrypted vault.

Alternatively:

```sh
buddy setup basic-setup               # instance URL + basic auth only
buddy auth test
```

That writes `~/.config/buddy/config.yaml`, stores the password in the encrypted vault (`secrets.enc`), and sets a default tenant.

In the local docs UI (`buddy serve`), open **config** to view and edit tenant/auth **metadata**. Vault secrets are still set from the terminal (`buddy cred set secret …`).

If this Mac already has an org MDM profile for preference domain `virtualpete.buddy.cli` (tenants and optional auth stubs), skip the wizard and run:

```sh
buddy setup secrets                   # create missing auths; store client secrets / passwords
buddy cred status                     # see tenant × surface coverage / gaps
buddy auth test
```

**Optional — API client (oauth) for day-to-day use** (instance only; not Platform gateway):

```sh
buddy setup write-script              # writes setup_buddy_role.buddy
buddy setup_buddy_role.buddy -y       # creates role "Buddy" + client "buddy-cli" (idempotent)
buddy setup store-oauth               # rotates client secret into a buddy oauth auth
buddy --user <auth-name> auth test
```

See `buddy setup help` for flags. After setup, review the Jamf API role and **drop privileges** you do not need (especially destructive MDM actions).

Already have tenants/auths? Use `buddy cred …` and `buddy auth test` instead — details in the [reference manual](reference-manual.md) and [Auth, secrets, and tokens](auth.md).

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
| [Auth, secrets, and tokens](auth.md) | Tenants, vault, channel policy |
| [Reference manual](reference-manual.md) | Config, CLI, buddyscript, environment |
| [Script IDE](/ide) | Single-buffer runner |
| [BuddyScript overview](buddyscript.md) | Language, runners, conventions |
| [Function index](/docs/builtins) | Every builtin |
| [Command index](/docs/cli) | Every CLI command |
