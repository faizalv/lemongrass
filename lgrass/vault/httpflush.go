package vault

// FlushHTTPTokens evicts channel id's cached tokens, for one user or, when user is empty, for
// every user, and returns how many were evicted. The next request for an evicted user logs in
// again. It runs under the same expiry and active checks as RequestHTTP, and a bring-your-own-token
// user's pasted token is put back in the cache by that next request.
func (s *Service) FlushHTTPTokens(id ChannelID, user string) (int, error) {
	_, domain, err := s.liveHTTPChannel(id)
	if err != nil {
		return 0, err
	}
	if user != "" {
		if user, err = resolveUser(domain, user); err != nil {
			return 0, err
		}
	}
	return s.flushTokens(id, user), nil
}

func (s *Service) flushTokens(id ChannelID, user string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	flushed := 0
	for key, t := range s.tokens {
		if key.channel != id || (user != "" && key.user != user) {
			continue
		}
		lockedFree(t.locked)
		delete(s.tokens, key)
		flushed++
	}
	return flushed
}
