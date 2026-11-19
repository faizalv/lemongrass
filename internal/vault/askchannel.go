// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

package vault

import "time"

// MarkFromAsk records that db channel id was created to answer an ask, so it is deleted when it expires.
func (s *Service) MarkFromAsk(id ChannelID) error {
	c, err := s.loadMeta(id)
	if err != nil {
		return err
	}
	c.FromAsk = true
	return s.saveMeta(c)
}

// MarkHTTPFromAsk records that HTTP channel id was created to answer an ask, so it is deleted when it expires.
func (s *Service) MarkHTTPFromAsk(id ChannelID) error {
	c, err := s.loadHTTPMeta(id)
	if err != nil {
		return err
	}
	c.FromAsk = true
	return s.saveHTTPMeta(c)
}

// SweepAskChannels deletes every expired db and HTTP channel that came from an ask. Each channel is read again before it goes, because the human may have activated it since the listing.
func (s *Service) SweepAskChannels(now time.Time) error {
	channels, err := s.ListChannels()
	if err != nil {
		return err
	}
	for _, c := range channels {
		if !c.FromAsk || !c.Expired(now) {
			continue
		}
		if cur, err := s.loadMeta(c.ID); err != nil || !cur.Expired(now) {
			continue
		}
		if err := s.Revoke(c.ID); err != nil {
			return err
		}
	}

	httpChannels, err := s.ListHTTPChannels()
	if err != nil {
		return err
	}
	for _, c := range httpChannels {
		if !c.FromAsk || !c.Expired(now) {
			continue
		}
		if cur, err := s.loadHTTPMeta(c.ID); err != nil || !cur.Expired(now) {
			continue
		}
		if err := s.RevokeHTTP(c.ID); err != nil {
			return err
		}
	}
	return nil
}
