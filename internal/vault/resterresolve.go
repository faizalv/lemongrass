// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (C) 2026 The Lemongrass Authors

package vault

// ResolveRester decrypts name's stored rester and flattens it for environment's api side, the same
// view an api channel gets, without creating a channel. It serves the human's own requests and is
// never reachable from the agent or the CLI.
func (s *Service) ResolveRester(rootSecret, name, environment string) (ResolvedRester, error) {
	r, err := s.GetRester(rootSecret, name)
	if err != nil {
		return ResolvedRester{}, err
	}
	return r.resolve(environment, TypeAPI)
}

// OnResterChange registers fn to run with a rester's name after it is updated or deleted, so state
// cached from its old config, such as the human's tokens, can be dropped.
func (s *Service) OnResterChange(fn func(name string)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onRester = append(s.onRester, fn)
}

func (s *Service) resterChanged(name string) {
	s.mu.Lock()
	hooks := append([]func(string){}, s.onRester...)
	s.mu.Unlock()
	for _, fn := range hooks {
		fn(name)
	}
}
