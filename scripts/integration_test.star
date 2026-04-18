TEST_PREFIX = "buddy-test-"

# Optional: set to a static computer group Jamf ID to run static_group_list_computers (read-only).
# Leave "" to skip. Membership add/remove is not exercised by this script.
TEST_STATIC_GROUP_ID = ""

created = {
    "policies": [],
    "groups": [],
    "profiles": [],
}


def require_ok(res, ctx):
    if not res["ok"]:
        fail("%s: %s" % (ctx, res["error"]))


def test_policy(cred, created):
    name = TEST_PREFIX + "policy"
    # Cleanup any leftover test policies (previous runs may have exited early).
    res = policy_find_ids(keywords=[TEST_PREFIX], force_refresh=True, cred=cred)
    require_ok(res, "policy_find_ids")
    leftovers = res["value"]
    if leftovers:
        dres = policy_delete(policy_ids=leftovers, cred=cred)
        require_ok(dres, "policy_delete (leftovers)")

    # Create a simple self-service policy; adjust args to match your environment.
    # Use all-computers scope for portability across Jamf instances.
    res = policy_create_ss(trigger="buddy-test", display_name=name, scope_group_id=0, cred=cred)
    require_ok(res, "policy_create_ss")
    policy_id = res["value"]
    created["policies"].append(policy_id)

    # policy_get_enable: should be "true"/"false"
    res = policy_get_enable(policy_id=policy_id)
    require_ok(res, "policy_get_enable")
    enabled = res["value"]
    if enabled not in ["true", "false"]:
        fail("policy_get_enable unexpected value: %r" % enabled)

    # Read back details.
    res = policy_get_details(policy_id=policy_id, cred=cred)
    require_ok(res, "policy_get_details")
    details = res["value"]
    expected_name = name + " [SS]"
    if details["name"] != expected_name:
        fail("policy name mismatch: %r" % details["name"])

    # policy_fetch_xml + policy_put_xml: set an initial custom trigger so policy_set_trigger can be exercised.
    res = policy_fetch_xml(policy_id=policy_id, cred=cred)
    require_ok(res, "policy_fetch_xml")
    xml = res["value"]
    # IMPORTANT: policy_get_id uses heuristics that only consider candidate policies
    # whose *name* contains the "app" part derived from the trigger. Use an
    # install-* trigger whose app substring matches the policy name.
    initial_trigger = "install-" + TEST_PREFIX + "policy-%d" % policy_id
    # Jamf policies always have a <trigger_other> element; populate it if empty.
    if "<trigger_other></trigger_other>" in xml:
        xml2 = xml.replace("<trigger_other></trigger_other>", "<trigger_other>%s</trigger_other>" % initial_trigger, 1)
    elif "<trigger_other/>" in xml:
        xml2 = xml.replace("<trigger_other/>", "<trigger_other>%s</trigger_other>" % initial_trigger, 1)
    else:
        # Best-effort: don't fail if Jamf changes the schema; skip trigger-based tests.
        xml2 = ""
    if xml2:
        res = policy_put_xml(policy_id=policy_id, xml_str=xml2, cred=cred)
        require_ok(res, "policy_put_xml (set trigger_other)")
        res = policy_fetch_xml(policy_id=policy_id, cred=cred)
        require_ok(res, "policy_fetch_xml (post put)")
        if initial_trigger not in res["value"]:
            fail("policy_put_xml did not persist trigger_other")

        # policy_set_trigger + policy_get_id
        new_trigger = "install-" + TEST_PREFIX + "policy-%d" % policy_id
        require_ok(policy_set_trigger(policy_id=policy_id, trigger=new_trigger, cred=cred), "policy_set_trigger")
        # policy_get_id is best-effort: it relies on the classic policy list endpoint.
        # Some Jamf roles can create/read by ID but cannot list all policies.
        res = policy_get_id(trigger=new_trigger, force_refresh=True)
        if not res["ok"]:
            error("policy_get_id skipped:", res["error"])
        else:
            if res["value"] != policy_id:
                fail("policy_get_id mismatch: got %r want %r" % (res["value"], policy_id))

        # Verify trigger_other via details regardless of policy_get_id availability.
        res = policy_get_details(policy_id=policy_id, cred=cred)
        require_ok(res, "policy_get_details (post set_trigger)")
        if res["value"]["trigger_other"] != new_trigger:
            fail("policy_set_trigger did not persist trigger_other: %r" % res["value"]["trigger_other"])
    else:
        error("policy_put_xml trigger_other setup skipped (no trigger_other element found)")

    # policy_set_frequency
    require_ok(policy_set_frequency(policy_id=policy_id, frequency="Ongoing", cred=cred), "policy_set_frequency")
    res = policy_get_details(policy_id=policy_id, cred=cred)
    require_ok(res, "policy_get_details (post frequency)")
    if res["value"]["frequency"] != "Ongoing":
        fail("policy_set_frequency did not persist")

    # policy_set_scope_all
    require_ok(policy_set_scope_all(policy_id=policy_id, cred=cred), "policy_set_scope_all")
    res = policy_get_details(policy_id=policy_id, cred=cred)
    require_ok(res, "policy_get_details (post scope_all)")
    if not res["value"]["scope"]["all_computers"]:
        fail("policy_set_scope_all did not persist")

    # policy_set_name
    new_name = TEST_PREFIX + "policy-renamed-%d" % policy_id
    require_ok(policy_set_name(policy_id=policy_id, new_name=new_name, cred=cred), "policy_set_name")
    res = policy_get_details(policy_id=policy_id, cred=cred)
    require_ok(res, "policy_get_details (post set_name)")
    if res["value"]["name"] != new_name:
        fail("policy_set_name did not persist: %r" % res["value"]["name"])

    # policy_set_ss (best-effort verify via XML)
    ss_desc = "Buddy integration test (policy_id=%d)" % policy_id
    require_ok(
        policy_set_ss(
            policy_id=policy_id,
            description=ss_desc,
            install_text="Install",
            reinstall_text="Reinstall",
            cred=cred,
        ),
        "policy_set_ss",
    )
    res = policy_fetch_xml(policy_id=policy_id, cred=cred)
    require_ok(res, "policy_fetch_xml (post set_ss)")
    if ss_desc not in res["value"]:
        error("policy_set_ss verify skipped (description not found in XML)")

    # Toggle enable/disable.
    require_ok(policy_set_enable(policy_id=policy_id, cred=cred), "policy_set_enable")
    res = policy_get_enable(policy_id=policy_id)
    require_ok(res, "policy_get_enable (post enable)")
    if res["value"] != "true":
        error("policy_get_enable expected true after enable, got:", res["value"])

    require_ok(policy_set_disable(policy_id=policy_id, cred=cred), "policy_set_disable")
    res = policy_get_enable(policy_id=policy_id)
    require_ok(res, "policy_get_enable (post disable)")
    if res["value"] != "false":
        error("policy_get_enable expected false after disable, got:", res["value"])

    # policy_clone (best-effort): diff before/after TEST_PREFIX policy IDs to discover clone.
    before = leftovers
    res = policy_find_ids(keywords=[TEST_PREFIX], force_refresh=True, cred=cred)
    require_ok(res, "policy_find_ids (pre clone)")
    before = res["value"]
    clone_version = "9.9.9"
    cres = policy_clone(policy_id=policy_id, version=clone_version, cred=cred)
    if not cres["ok"]:
        error("policy_clone skipped:", cres["error"])
    else:
        res = policy_find_ids(keywords=[TEST_PREFIX], force_refresh=True, cred=cred)
        require_ok(res, "policy_find_ids (post clone)")
        after = res["value"]
        # Find new IDs not in before.
        for pid in after:
            if pid not in before and pid not in created["policies"]:
                created["policies"].append(pid)

    # policy_set_install (best-effort): may depend on script params existing in policy.
    ires = policy_set_install(policy_id=policy_id, version="1.2.3", cred=cred)
    if not ires["ok"]:
        error("policy_set_install skipped:", ires["error"])

    # policy_set_package (best-effort): depends on environment-specific policies/packages.
    pres = policy_set_package(app="chrome", version="1.2.3", policy_id=policy_id, cred=cred)
    if not pres["ok"]:
        error("policy_set_package skipped:", pres["error"])

    # policy_template (coverage only): ensure it returns a dict tagged as a template.
    tmpl = policy_template(
        name=TEST_PREFIX + "tmpl",
        trigger=TEST_PREFIX + "tmpl-trigger",
        script_id=51,
        scope_all=True,
    )
    if type(tmpl) != "dict" or "__policy_template__" not in tmpl:
        fail("policy_template did not return a template dict")


def cleanup_policy(cred, created):
    if not created["policies"]:
        return
    # policy_delete takes a list of IDs.
    res = policy_delete(policy_ids=created["policies"], cred=cred)
    if not res["ok"]:
        error("cleanup policy_delete error:", res["error"])
    created["policies"].clear()


def test_smart_group(cred, created):
    # Remove any leftover test smart groups from previous runs.
    res = group_list_smart(filter=TEST_PREFIX, cred=cred)
    require_ok(res, "group_list_smart")
    for g in res["value"]:
        gid = g["id"]
        dres = group_delete(group_id=gid, cred=cred)
        if not dres["ok"]:
            error("cleanup old group_delete error:", dres["error"])

    # Create a minimal smart group via group_put.
    group_name = TEST_PREFIX + "group"
    body = {
        "id": -1,
        "name": group_name,
        "criteria": [
            {
                "name": "Application Title",
                "priority": 0,
                "andOr": "and",
                "searchType": "is",
                "value": "Example Application",
            },
        ],
    }
    # Jamf Pro API expects id to be a positive numeric string or -1 for create.
    res = group_put(group_id="-1", dict=body, cred=cred)
    if not res["ok"]:
        fail("group_put: %s" % res["error"])

    # Find the created group by name.
    res = group_find_id(name=group_name, cred=cred)
    require_ok(res, "group_find_id")
    ginfo = res["value"]
    group_id = ginfo["id"]
    created["groups"].append(group_id)

    # Inspect and then update criteria.
    res = smart_group_get_criteria(group_id=group_id, cred=cred)
    require_ok(res, "smart_group_get_criteria")
    criteria = res["value"]
    if len(criteria) < 1:
        fail("expected at least one criterion")

    # Attempt to update any criterion whose name contains 'Application'.
    res = smart_group_update_criterion_value(
        group_id=group_id,
        name_substring="Application",
        new_value="Example Application",
        cred=cred,
    )
    require_ok(res, "smart_group_update_criterion_value")


def cleanup_smart_group(cred, created):
    for gid in created["groups"]:
        res = group_delete(group_id=gid, cred=cred)
        if not res["ok"]:
            error("cleanup group_delete error:", res["error"])
    created["groups"].clear()


def test_static_group_list(cred):
    if TEST_STATIC_GROUP_ID == "":
        error("static_group_list_computers: skipped (set TEST_STATIC_GROUP_ID to a static group id)")
        return
    res = static_group_list_computers(group_id=TEST_STATIC_GROUP_ID, cred=cred)
    require_ok(res, "static_group_list_computers")
    members = res["value"]
    if type(members) != "list":
        fail("static_group_list_computers: expected list")


def test_profile(cred, created):
    # 1. Find a base profile to clone.
    base_name = TEST_PREFIX + "source-profile"
    res = profile_find_id(name=base_name)
    if not res["ok"]:
        error("profile_find_id skipped (no source profile named %r): %s" % (base_name, res["error"]))
        return
    base = res["value"]
    source_id = base["id"]

    # 2. Clone into a test profile.
    clone_name = TEST_PREFIX + "profile"
    res = profile_clone(profile_id=source_id, name=clone_name, cred=cred)
    require_ok(res, "profile_clone")
    new_profile = res["value"]
    profile_id = new_profile["id"]
    created["profiles"].append(profile_id)

    # 3. Extract payloads plist XML from the profile.
    res = profile_xml_extract_payloads(profile_id=profile_id, cred=cred)
    require_ok(res, "profile_xml_extract_payloads")
    payloads_plist = res["value"]

    # 4. List identifiers and pick a payload domain.
    res = payloads_list_identifiers(payloads_plist)
    require_ok(res, "payloads_list_identifiers")
    ids = res["value"]
    if not ids:
        error("profile has no payloads; skipping payload mutation")
        return
    domain = ids[0]

    # 5. Get the payload dict XML for that domain and decode to dict.
    res = payloads_get_payload(payloads_plist, domain)
    require_ok(res, "payloads_get_payload")
    payload_dict_xml = res["value"]

    res = plist_decode(payload_dict_xml)
    require_ok(res, "plist_decode")
    payload_dict = res["value"]

    # 6. Mutate the payload dict.
    payload_dict["BuddyTestKey"] = TEST_PREFIX

    # 7. Encode back to plist XML and replace the payload in the payloads plist.
    res = plist_encode(payload_dict)
    require_ok(res, "plist_encode")
    new_payload_dict_xml = res["value"]

    res = payloads_replace_payload(payloads_plist, domain, new_payload_dict_xml)
    require_ok(res, "payloads_replace_payload")
    new_payloads_plist = res["value"]

    # 8. Write updated payloads back into the profile.
    res = profile_xml_replace_payloads(profile_id=profile_id, payloads_plist_xml=new_payloads_plist, cred=cred)
    require_ok(res, "profile_xml_replace_payloads")


def cleanup_profile(cred, created):
    for pid in created["profiles"]:
        error("profile cleanup: consider deleting test profile ID", pid)
    created["profiles"].clear()


def test_package_and_script(cred):
    # Packages: list and fetch one
    res = package_list(filter="")
    require_ok(res, "package_list")
    pkgs = res["value"]
    if pkgs:
        first = pkgs[0]
        pkg_id = first["id"] if type(first) == "dict" else first
        res2 = package_fetch(package_id=pkg_id, cred=cred)
        require_ok(res2, "package_fetch")
        pkg = res2["value"]
        if "packageName" not in pkg:
            fail("package_fetch: missing packageName in package %r" % pkg)

    # Scripts: list and fetch one
    res = script_list(cred=cred)
    require_ok(res, "script_list")
    scripts = res["value"]
    if scripts:
        first = scripts[0]
        script_id = first["id"] if type(first) == "dict" else first
        res2 = script_fetch(script_id=script_id, cred=cred)
        require_ok(res2, "script_fetch")
        body = res2["value"]
        if not body:
            fail("script_fetch returned empty body for script_id=%r" % script_id)


def cleanup_all(cred, created):
    # Run per-domain cleanups; order chosen to minimize dependencies.
    cleanup_policy(cred, created)
    cleanup_smart_group(cred, created)
    cleanup_profile(cred, created)


def main(cred=None):
    if cred == None:
        cred = "default"
    # Mutable dict for this run (globals are frozen after ExecFile).
    created = {"policies": [], "groups": [], "profiles": []}
    test_policy(cred, created)
    test_smart_group(cred, created)
    test_static_group_list(cred)
    test_profile(cred, created)
    test_package_and_script(cred)
    cleanup_all(cred, created)

