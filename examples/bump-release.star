# bump-release.star — illustrative release bump (edit constants for your Jamf tenant)
#
# Usage:
#   buddy run examples/bump-release.star -- 2.4.27
#
# Every buddy builtin returns {"ok": True, "value": ...} or {"ok": False, "error": "..."}.
# Resource IDs first; optional cred= last.
# Use error(...) for messages (stderr); use print(...) only for stdout you intend to pipe.

# --- edit for your environment ---
TRIGGER_PREFIX = "install-exampleapp-"
APP_NAME = "exampleapp"

# Policies found by trigger above often cover package + install + self-service together.
# If yours differ, set these to the other policies' numeric IDs and the guards below will run them.
INSTALL_POLICY_ID = 0   # 0 = skip policy_set_install
SELF_SERVICE_POLICY_ID = 0  # 0 = skip policy_set_ss
VERSION_GROUP_ID = ""   # smart group ID string, or "" = skip group_set_version


def main():
    if len(args) < 1:
        error("Usage: buddy run examples/bump-release.star -- <version>")
        exit(1)

    version = args[0]

    r = policy_get_id(trigger=TRIGGER_PREFIX + version, force_refresh=True)
    if not r["ok"]:
        error("policy_get_id:", r["error"])
        exit(1)

    policy_id = r["value"]

    policy_set_package(app=APP_NAME, version=version, policy_id=policy_id)

    install_id = INSTALL_POLICY_ID if INSTALL_POLICY_ID else policy_id
    policy_set_install(policy_id=install_id, version=version)

    ss_id = SELF_SERVICE_POLICY_ID if SELF_SERVICE_POLICY_ID else policy_id
    policy_set_ss(policy_id=ss_id, version=version)

    if VERSION_GROUP_ID:
        group_set_version(group_id=VERSION_GROUP_ID, version=version)

    error("Done. Bumped to", version)


main()
