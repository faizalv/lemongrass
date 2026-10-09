package vault

import "errors"

const policyEntryName = "policy"

var ErrNoPassphrase = errors.New("vault: no passphrase is set")

// PutPolicy seals policy under the root key, replacing any earlier one, and makes it the active policy. The vault never inspects the bytes.
func (s *Service) PutPolicy(rootSecret string, policy []byte) error {
	has, err := s.HasPassphrase()
	if err != nil {
		return err
	}
	if !has {
		return ErrNoPassphrase
	}
	if err := s.VerifyPassphrase(rootSecret); err != nil {
		return err
	}
	rootKey, err := DeriveKey(rootSecret, s.rootSalt)
	if err != nil {
		return err
	}
	defer Zero(rootKey)
	if err := s.policies.Put(policyEntryName, rootKey, policy); err != nil {
		return err
	}
	return s.setActivePolicy(policy)
}

// GetPolicy decrypts the stored policy for the human-facing editor. It returns nil when none has been stored.
func (s *Service) GetPolicy(rootSecret string) ([]byte, error) {
	if err := s.VerifyPassphrase(rootSecret); err != nil {
		return nil, err
	}
	rootKey, err := DeriveKey(rootSecret, s.rootSalt)
	if err != nil {
		return nil, err
	}
	defer Zero(rootKey)
	policy, err := s.policies.Get(policyEntryName, rootKey)
	if errors.Is(err, ErrNotFound) {
		return nil, nil
	}
	return policy, err
}

// ActivatePolicy loads the stored policy into locked memory so the hook can read it without the root secret. Nothing stored activates an empty policy.
func (s *Service) ActivatePolicy(rootSecret string) error {
	policy, err := s.GetPolicy(rootSecret)
	if err != nil {
		return err
	}
	defer Zero(policy)
	return s.setActivePolicy(policy)
}

// ActivePolicy returns a copy of the active policy, and false while none is loaded, for example after a vault restart before the first unlock.
func (s *Service) ActivePolicy() ([]byte, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.policyLoaded {
		return nil, false
	}
	out := make([]byte, len(s.activePolicy))
	copy(out, s.activePolicy)
	return out, true
}

func (s *Service) setActivePolicy(policy []byte) error {
	copied := make([]byte, len(policy))
	copy(copied, policy)
	locked, err := LockKey(copied)
	if err != nil {
		Zero(copied)
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.dropActivePolicyLocked()
	s.activePolicy = locked
	s.policyLoaded = true
	return nil
}

func (s *Service) dropActivePolicyLocked() {
	if s.activePolicy != nil {
		LockedFree(s.activePolicy)
	}
	s.activePolicy = nil
	s.policyLoaded = false
}
