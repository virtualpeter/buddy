# stdlib/release.star — multi-step release helpers composed from buddy builtins.
#
# load("stdlib/release.star", "bump_release")

def _ok(value):
    return {"ok": True, "value": value, "error": ""}

def _err(msg):
    return {"ok": False, "value": None, "error": msg}

def _cred_kwargs(cred):
    if cred == None or cred == "":
        return {}
    return {"cred": cred}

# bump_release — resolve policy by trigger, set package/install/SS, optional smart-group version.
#
# Returns {"ok", "value", "error"}. value is {"policy_id": int} on success.
def bump_release(
        trigger_prefix,
        app,
        version,
        install_policy_id = 0,
        self_service_policy_id = 0,
        version_group_id = "",
        cred = None):
    """Bump an app release: package + install + self-service (+ optional version group)."""
    ck = _cred_kwargs(cred)
    trigger = trigger_prefix + version

    r = policy(trigger = trigger, force_refresh = True, **ck)
    if not r["ok"]:
        return _err("policy: " + r["error"])
    policy_id = r["value"]["id"]

    r = policy_set_package(app = app, version = version, policy_id = policy_id, **ck)
    if not r["ok"]:
        return _err("policy_set_package: " + r["error"])

    install_id = install_policy_id if install_policy_id else policy_id
    r = policy_set_install(policy_id = install_id, version = version, **ck)
    if not r["ok"]:
        return _err("policy_set_install: " + r["error"])

    ss_id = self_service_policy_id if self_service_policy_id else policy_id
    r = policy_set_ss(policy_id = ss_id, version = version, **ck)
    if not r["ok"]:
        return _err("policy_set_ss: " + r["error"])

    if version_group_id:
        r = smart_group_set_version(group_id = version_group_id, version = version, **ck)
        if not r["ok"]:
            return _err("smart_group_set_version: " + r["error"])

    return _ok({"policy_id": policy_id})
