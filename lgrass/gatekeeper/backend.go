// Package gatekeeper serves the vault and its gates over the daemon's unix socket and dials that socket as a client.
package gatekeeper

import (
	"github.com/faizalv/lemongrass/dbgate"
	"github.com/faizalv/lemongrass/restergate"
	"github.com/faizalv/lemongrass/vault"
)

// Backend is everything the daemon serves: the vault for credential keeping and the gates that execute against it.
type Backend struct {
	*vault.Service
	DB   *dbgate.Gate
	HTTP *restergate.Gate
}

// NewBackend opens the vault under dir and attaches both gates to it.
func NewBackend(dir string) (*Backend, error) {
	svc, err := vault.NewService(dir)
	if err != nil {
		return nil, err
	}
	return NewBackendFor(svc), nil
}

// NewBackendFor attaches both gates to an already-open vault.
func NewBackendFor(svc *vault.Service) *Backend {
	return &Backend{Service: svc, DB: dbgate.New(svc), HTTP: restergate.New(svc)}
}
