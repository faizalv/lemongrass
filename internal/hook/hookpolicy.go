package hook

import (
	"time"

	"github.com/faizalv/lemongrass/internal/gatekeeper"
	"github.com/faizalv/lemongrass/internal/guard"
)

const policyFetchTimeout = 500 * time.Millisecond

// A vault that is down, still locked, or serving a policy that fails validation leaves only the built-in catalog in force.
func hookPolicy() guard.Policy {
	return hookPolicyFrom(gatekeeper.SocketPath())
}

func hookPolicyFrom(socketPath string) guard.Policy {
	client := &gatekeeper.Client{SocketPath: socketPath, Timeout: policyFetchTimeout}
	policy, active, err := client.CurrentPolicy()
	if err != nil || !active {
		return guard.Policy{}
	}
	return policy
}
