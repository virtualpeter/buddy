package buddy

import (
	"strings"
	"testing"
)

const samplePolicyXML = `<?xml version="1.0" encoding="UTF-8"?>
<policy>
  <general>
    <id>99</id>
    <name>Old Policy</name>
    <enabled>true</enabled>
    <trigger>USER_INITIATED</trigger>
    <trigger_other>install-app-1.0.0</trigger_other>
    <frequency>Ongoing</frequency>
    <date_time_limitations>
      <activation_date/>
      <expiration_date>2020-01-01</expiration_date>
    </date_time_limitations>
  </general>
  <scope>
    <all_computers>true</all_computers>
    <all_jss_users>false</all_jss_users>
    <computers/>
    <computer_groups/>
  </scope>
</policy>`

func TestPreparePolicyXMLForCreate_stripsIDSetsNameTriggerScope(t *testing.T) {
	trig := "install-app-2.0.0"
	out, err := PreparePolicyXMLForCreate([]byte(samplePolicyXML), PolicyPrepareSpec{
		Name:    "New Policy",
		Trigger: &trig,
	})
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	if strings.Contains(s, "<id>99</id>") {
		t.Fatal("source id should be stripped")
	}
	if !strings.Contains(s, "<name>New Policy</name>") {
		t.Fatalf("name not applied: %s", s)
	}
	if !strings.Contains(s, "<trigger_other>install-app-2.0.0</trigger_other>") {
		t.Fatalf("trigger not applied: %s", s)
	}
	if !strings.Contains(s, "<all_computers>false</all_computers>") {
		t.Fatal("default scope should be unassigned")
	}
	if strings.Contains(s, "<all_computers>true</all_computers>") {
		t.Fatal("source all_computers should not remain")
	}
	if strings.Contains(s, "2020-01-01") {
		t.Fatal("date_time_limitations should be cleared")
	}
}

func TestPreparePolicyXMLForCreate_scopeAllAndGroups(t *testing.T) {
	all := true
	out, err := PreparePolicyXMLForCreate([]byte(samplePolicyXML), PolicyPrepareSpec{
		Name:     "Scoped",
		ScopeAll: &all,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "<all_computers>true</all_computers>") {
		t.Fatal("scope_all=true not applied")
	}

	out2, err := PreparePolicyXMLForCreate([]byte(samplePolicyXML), PolicyPrepareSpec{
		Name:          "Groups",
		ScopeGroupIDs: []int{7, 8},
	})
	if err != nil {
		t.Fatal(err)
	}
	s := string(out2)
	if !strings.Contains(s, "<id>7</id>") || !strings.Contains(s, "<id>8</id>") {
		t.Fatalf("group ids missing: %s", s)
	}
	if strings.Contains(s, "<all_computers>true</all_computers>") {
		t.Fatal("group scope should not be all computers")
	}
}

func TestPreparePolicyXMLForCreate_requiresName(t *testing.T) {
	_, err := PreparePolicyXMLForCreate([]byte(samplePolicyXML), PolicyPrepareSpec{})
	if err == nil {
		t.Fatal("expected error for empty name")
	}
}

func TestPreparePolicyXMLForCreate_resourcePolicyRoot(t *testing.T) {
	xml := strings.Replace(samplePolicyXML, "<policy>", "<ResourcePolicy>", 1)
	xml = strings.Replace(xml, "</policy>", "</ResourcePolicy>", 1)
	out, err := PreparePolicyXMLForCreate([]byte(xml), PolicyPrepareSpec{Name: "X"})
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	if !strings.Contains(s, "<policy>") || strings.Contains(s, "ResourcePolicy") {
		t.Fatalf("root not normalized: %s", s)
	}
}

func TestSetPolicyGeneralID(t *testing.T) {
	prepared, err := PreparePolicyXMLForCreate([]byte(samplePolicyXML), PolicyPrepareSpec{Name: "Mig"})
	if err != nil {
		t.Fatal(err)
	}
	withID, err := setPolicyGeneralID(prepared, 42)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(withID), "<id>42</id>") {
		t.Fatalf("id not injected: %s", withID)
	}
}

func TestPolicyDetailsMap(t *testing.T) {
	d := &PolicyDetails{
		ID: 1, Name: "n", Enabled: true, Trigger: "USER_INITIATED",
		TriggerOther: "t", Frequency: "Ongoing",
	}
	d.Scope.AllComputers = true
	d.Packages = append(d.Packages, struct {
		ID     int
		Name   string
		Action string
	}{ID: 2, Name: "p.pkg", Action: "Install"})
	m := PolicyDetailsMap(d)
	if m["id"] != 1 || m["name"] != "n" || m["trigger_other"] != "t" {
		t.Fatalf("unexpected map: %#v", m)
	}
	scope, _ := m["scope"].(map[string]any)
	if scope["all_computers"] != true {
		t.Fatal("scope missing")
	}
}

func TestPreparePolicyMigrateXML_defaultsName(t *testing.T) {
	prepared, name, err := PreparePolicyMigrateXML([]byte(samplePolicyXML), PolicyMigrateSpec{})
	if err != nil {
		t.Fatal(err)
	}
	if name != "Old Policy" {
		t.Fatalf("name=%q", name)
	}
	if strings.Contains(string(prepared), "<id>99</id>") {
		t.Fatal("id should be stripped")
	}
}
