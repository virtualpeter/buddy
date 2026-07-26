# Change batches (howto)

Enterprise Jamf changes are usually **approved as a document** before anyone presses “go”. Buddy’s change-batch APIs let you author that document in a `.buddy` script: human notes plus steps that resolve against live Jamf, then apply the same batch after approval.

This guide is the intended use of:

- `change_batch` / `plan_change` / `run_change`
- `note=` on steps, `step_note`, `step_with_note`
- Related CLI confirms for `policy set package` / `policy clone`

Canonical signatures: `buddy help builtins` (sections **DECLARATIVE WORKFLOW** and **CHANGE BATCH**).

---

## When to use what

| Tool | Use it for |
|------|------------|
| **CLI** (`buddy policy set package`, `clone`, …) | One-off ops; interactive confirm shows source ID/trigger and disable decision |
| **`run_workflow([step_…])`** | Quick multi-step script with one Proceed? — no change-record markdown |
| **`plan_change` + `run_change`** | Lodge a CAB/change ticket, then apply the same batch later |

Prefer **triggers** (`from_trigger=` / `--trigger=`) over app-name heuristics when naming a source policy.

---

## The approval loop

```text
1. Author a .buddy script (title, summary, steps + notes)
2. buddy script.buddy                 → plan_change (no writes)
3. Paste markdown into the change record / get approval
4. buddy script.buddy -- apply        → run_change (re-resolve, apply, outcomes)
```

**Plan time** describes *intent*: source policies (with existing IDs), package IDs, create vs update, whether the source will be disabled, and your notes.

**Apply time** records *outcomes*: new policy IDs from Jamf creates, updates, and disabled sources.

Created policy IDs are **not** predicted in the plan — Jamf assigns them only after POST.

---

## Minimal script shape

Put control flow inside `main()` (Starlark disallows top-level `if` / `for`).

```python
#!/usr/bin/env buddy

def main():
    mode = "plan"
    if len(args) > 0 and args[0] in ("apply", "run"):
        mode = "apply"

    change = change_batch(
        title = "App 2.0 cache + install rollout",
        summary = """Create versioned cache for 2.0; replace install-app-next.
Self Service already calls install-app-next.""",
        risk = "low",
        window = "2026-07-28 22:00–23:00",
    )

    steps = [
        step_set_package(
            app = "app",
            version = "2.0",
            from_trigger = "cache-app-1.0",
            package_pattern = "*app*2.0*",
            note = "Versioned cache; leave 1.0 cache enabled for rollback.",
        ),
        step_set_package(
            app = "app",
            version = "2.0",
            from_trigger = "install-app-next",
            package_pattern = "*app*2.0*",
            note = "Unversioned install-next replacement; disable prior install policy.",
        ),
        step_note(
            note = "Self Service already invokes install-app-next; no SS edit.",
        ),
    ]

    planned = plan_change(change = change, steps = steps)
    if not planned["ok"]:
        fail(planned["error"])

    if mode != "apply":
        print("Plan only. Re-run with: buddy this-script.buddy -- apply")
        return

    r = run_change(change = change, steps = steps, planned = planned["value"])
    if not r["ok"]:
        fail(r["error"])
```

Runnable sample: [`examples/change-batch-santa.buddy`](../examples/change-batch-santa.buddy).

```sh
buddy examples/change-batch-santa.buddy              # plan only
buddy examples/change-batch-santa.buddy -- apply     # apply (prompts)
buddy -y examples/change-batch-santa.buddy -- apply  # apply, skip confirms
```

---

## Writing good notes

| Helper | Role |
|--------|------|
| `note=` on `step_set_package` / `step_clone_policy` | Why this step exists (rollback, disable intent, scope) |
| `step_note(note=…)` | Prose with **no** Jamf write (context for reviewers) |
| `step_with_note(step, note)` | Attach a note to any other `step_*` |

Notes show up under each planned change and again in Outcomes. They are the human layer reviewers actually read — keep them specific (“leave 1.0 cache enabled”) rather than restating the API call.

---

## What the plan document contains

Typical markdown from `plan_change`:

1. **Title / risk / window / summary** from `change_batch`
2. Disclaimer that create IDs appear only after apply
3. Numbered steps:
   - **Update** — existing policy ID + new package
   - **Create** — new name + trigger; **source** name/ID/trigger; package name/ID; disable yes/no (reason)
   - **Note** — prose-only lines
4. Your `note=` text on each mutating step

**Included (known before write):** source policy IDs, package IDs, triggers, disable decision.  
**Excluded until Outcomes:** IDs of policies that will be created.

Copy the printed markdown (or `planned["value"]["markdown"]`) into ServiceNow / Jira / your CAB form.

---

## Disable-source rules (defaults)

These match CLI / `policy_set_package` / `policy_clone` and appear in every plan line:

| Operation | Default | Override |
|-----------|---------|----------|
| Versioned create (new trigger, e.g. `cache-app-1.0` → `cache-app-2.0`) | Do **not** disable source | `--disable-source` / `disable_source=True` |
| Unversioned replacement (same trigger, e.g. `install-app-next`) | **Disable** source | `--keep-source` / `keep_source=True` |
| Same-version in-place package update | No disable | — |
| `clone` | **Disable** source | `keep_source=True` / `--keep-source` |

CLI confirms and change-batch plans both spell this out as `Disable source: yes/no (reason)`.

---

## Drift detection

Pass the `plan_change` result into apply:

```python
planned = plan_change(change, steps)
run_change(change, steps, planned = planned["value"])
```

`run_change` **re-resolves** against Jamf. Fingerprints cover source ID, modify vs create, names/triggers, package ID, and disable intent — not created policy IDs.

If anything differs since the lodged plan (source gone, different package, disable flipped):

1. Prints a **Drift warning**
2. Asks **Proceed despite drift?** (unless `-y`)
3. Then asks **Apply this change batch?**

`-y` skips both prompts (automation / already-approved pipelines). Prefer interactive apply for human CAB follow-through.

---

## Outcomes after apply

On success, markdown continues with **## Outcomes**, for example:

- `Created policy "Cache App 2.0" (ID 22) with trigger cache-app-2.0`
- `Disabled source "Install App 1.0" (ID 20)`
- Notes echoed per step

`run_change` returns `{ok, value}` where `value` is the full markdown (plan + outcomes) for logging or attaching to the closed change record.

---

## Tips

1. **Resolve sources by trigger** — `from_trigger=` / `--trigger=` beats app-name scan when cache and install both exist.
2. **Keep plan and apply in one script** — same `steps` list; only the mode changes (`plan` vs `apply`).
3. **Do not invent create IDs in notes** — write names and triggers; IDs land in Outcomes.
4. **Test in a non-production tenant** first — package uploads and policy creates are real Jamf writes on apply.
5. **`run_workflow` is still fine** for lab scripts that never need a change ticket.

---

## Version bump vs new app setup

| Kind | Typical steps | Example |
|------|---------------|---------|
| **Version bump** | `step_set_package` / `step_clone_policy` against existing cache/install triggers | [`examples/change-batch-santa.buddy`](../examples/change-batch-santa.buddy) |
| **New app (Classic)** | Separate prefs + PPPC `step_create_profile`, then `step_create_package_policy` (cache + install), optional `step_create_ss` | [`examples/setup-app-classic.buddy`](../examples/setup-app-classic.buddy) |

### New app setup (Classic)

For a brand-new app, compose a greenfield batch with **two profiles** (prefs and PPPC are separate) plus cache/install policies. Prefer the stdlib helper so callers always go through `plan_change` / `run_change` (it returns steps; it does not write):

```python
load("stdlib/app_setup.buddy", "setup_app_classic")

prefs = prefs_payload(domain = "com.example.app", settings = {"Foo": True})["value"]
fda = pppc_fda(
    bundle_id = "com.example.app",
    code_requirement = 'identifier "com.example.app" and anchor apple generic',
)["value"]

steps = setup_app_classic(
    app = "example",
    version = "1.0.0",
    prefs_content = [prefs],
    pppc_services = {"SystemPolicyAllFiles": [fda]},
    package_pattern = "*example*1.0.0*",
    create_ss = False,
    scope_all = True,
)
```

Or build steps by hand with `step_create_profile` / `step_create_package_policy` / `step_create_ss`. Scope defaults to **all computers** on creates (override with `scope_group_ids`).

PPPC helpers (offline):

- `pppc_fda(bundle_id, code_requirement, comment?)` → one `SystemPolicyAllFiles` entry
- `pppc_payload(services={...})` → full `com.apple.TCC.configuration-profile-policy` payload dict
- Compose Accessibility (etc.) the same way: extra keys under `services`

Prefs: `prefs_payload(domain, settings)` for a simple preference-domain payload, or assemble MCX via existing `payloads_*` helpers.

**Follow-up (not in this pass):** DDM / Platform blueprints can carry the same profile payloads via `com.jamf.ddm-configuration-profile`; cache/install policies remain Classic.

---

## Related

- Version-bump example: [`examples/change-batch-santa.buddy`](../examples/change-batch-santa.buddy)
- New-app example: [`examples/setup-app-classic.buddy`](../examples/setup-app-classic.buddy)
- Examples index: [`examples/README.md`](../examples/README.md)
- Builtins reference: `buddy help builtins`
- CLI: `buddy policy set help`, `buddy policy clone help`
- Reference manual: [`reference-manual.md`](reference-manual.md)
