package gatekeeper

import (
	"encoding/json"

	"github.com/faizalv/lemongrass/internal/guard"
)

const (
	opPutPolicy      = "put_policy"
	opGetPolicy      = "get_policy"
	opActivatePolicy = "activate_policy"
	opCurrentPolicy  = "current_policy"
	opPolicyCatalog  = "policy_catalog"
)

type putPolicyPayload struct {
	RootSecret string          `json:"root_secret"`
	Policy     json.RawMessage `json:"policy"`
}

type policyPayload struct {
	Policy json.RawMessage `json:"policy,omitempty"`
	Active bool            `json:"active"`
}

type policyCatalogPayload struct {
	Rules []guard.RuleInfo `json:"rules"`
}

// PutPolicy validates policyJSON against the guard catalog before the vault seals it, so a locked rule or a blocked lgrass binary is refused here, not only in the UI.
func (b *Backend) PutPolicy(rootSecret string, policyJSON []byte) error {
	policy, err := guard.ParsePolicy(policyJSON)
	if err != nil {
		return err
	}
	normalized, err := policy.Marshal()
	if err != nil {
		return err
	}
	return b.Service.PutPolicy(rootSecret, normalized)
}

func (c *Client) PutPolicy(rootSecret string, policy guard.Policy) error {
	raw, err := policy.Marshal()
	if err != nil {
		return err
	}
	return c.call(opPutPolicy, putPolicyPayload{RootSecret: rootSecret, Policy: raw}, nil)
}

// GetPolicy returns the stored policy for the editor, and an empty policy when none is stored.
func (c *Client) GetPolicy(rootSecret string) (guard.Policy, error) {
	var out policyPayload
	if err := c.call(opGetPolicy, rootSecretPayload{RootSecret: rootSecret}, &out); err != nil {
		return guard.Policy{}, err
	}
	return guard.ParsePolicy(out.Policy)
}

func (c *Client) ActivatePolicy(rootSecret string) error {
	return c.call(opActivatePolicy, rootSecretPayload{RootSecret: rootSecret}, nil)
}

// CurrentPolicy returns the active policy and whether one is loaded.
func (c *Client) CurrentPolicy() (guard.Policy, bool, error) {
	var out policyPayload
	if err := c.call(opCurrentPolicy, nil, &out); err != nil {
		return guard.Policy{}, false, err
	}
	policy, err := guard.ParsePolicy(out.Policy)
	return policy, out.Active, err
}

func (c *Client) PolicyCatalog() ([]guard.RuleInfo, error) {
	var out policyCatalogPayload
	err := c.call(opPolicyCatalog, nil, &out)
	return out.Rules, err
}
