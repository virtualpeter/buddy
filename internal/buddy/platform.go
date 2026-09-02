package buddy

import (
	"encoding/json"
	"fmt"
)

// requirePlatformCred ensures a platform-capable session can be resolved for credName
// (auth pin or active tenant). Auto-picks a platform-linked auth when possible.
func requirePlatformCred(credName string) error {
	sess, err := ResolveSessionForCred(credName, OpPlatform)
	if err != nil {
		return fmt.Errorf("Platform API requires a Platform API Gateway tenant+auth (set tenantId on the tenant and link an oauth auth); see 'buddy cred help': %w", err)
	}
	if sess.Credential.EffectiveTenantID() == "" {
		return fmt.Errorf("Platform API requires a Platform API Gateway tenant (tenantId); see 'buddy cred help'")
	}
	return nil
}

// requirePlatformMutate is like requirePlatformCred but rejects readonly auths / channel policy.
func requirePlatformMutate(credName string) error {
	if _, err := ResolveSessionForCred(credName, OpPlatformWrite); err != nil {
		return err
	}
	return nil
}

func printJSON(v interface{}) error {
	enc, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal json: %w", err)
	}
	fmt.Println(string(enc))
	return nil
}

func decodeJSONObject(body []byte) (map[string]interface{}, error) {
	var out map[string]interface{}
	if len(body) == 0 {
		return map[string]interface{}{}, nil
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, err
	}
	if out == nil {
		out = map[string]interface{}{}
	}
	return out, nil
}

func decodeJSONObjectList(body []byte) ([]map[string]interface{}, error) {
	var out []map[string]interface{}
	if len(body) == 0 {
		return nil, nil
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, err
	}
	return out, nil
}
