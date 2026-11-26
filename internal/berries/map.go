// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

// Package berries is lemongrass's deterministic security tester. It replays requests already made through an api
// channel as the channel's other users and judges each answer against the project's access map, so no model ever
// writes a probe and the human-approved map, never a model, decides what should be denied.
package berries

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// MapFile is where a project's berry lives, relative to the project root. It is committed, so an access rule change
// shows in review.
const MapFile = ".berries/map.yaml"

// Pseudo-roles a rule may allow besides the map's own roles. Owner is the user a seed was recorded as; any is every
// user on the channel.
const (
	RoleOwner = "owner"
	RoleAny   = "any"
)

// Rule statuses. Approval is recorded in the file and reviewed in its diff; berries reports it but never treats it as
// a boundary, since a model can edit the file too.
const (
	StatusProposed = "proposed"
	StatusApproved = "approved"
)

var mapMethods = map[string]bool{"GET": true, "HEAD": true, "OPTIONS": true, "POST": true, "PUT": true, "PATCH": true, "DELETE": true}

// Map is a project's expected access: its roles, which are rester user tags, and who may call each route.
type Map struct {
	Roles  []string `yaml:"roles"`
	Routes []Route  `yaml:"routes"`
}

// Route is one rule. Pattern is "METHOD /path" where a {name} segment matches any one path segment.
type Route struct {
	Pattern string   `yaml:"route"`
	Allow   []string `yaml:"allow"`
	Source  string   `yaml:"source"`
	Status  string   `yaml:"status"`

	method   string
	segments []string
}

// Method is the route's HTTP method.
func (r Route) Method() string { return r.method }

// LoadMap reads the project's map. A project without one returns an error satisfying errors.Is(err, os.ErrNotExist).
func LoadMap(root string) (*Map, error) {
	data, err := os.ReadFile(filepath.Join(root, MapFile))
	if err != nil {
		return nil, err
	}
	return ParseMap(data)
}

// HasMap reports whether the project has a berry, which is what turns seed capture on.
func HasMap(root string) bool {
	info, err := os.Stat(filepath.Join(root, MapFile))
	return err == nil && !info.IsDir()
}

// ParseMap decodes and validates a map. Every refusal names the route at fault.
func ParseMap(data []byte) (*Map, error) {
	var m Map
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&m); err != nil && !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("berries: %s: %w", MapFile, err)
	}
	if err := m.validate(); err != nil {
		return nil, err
	}
	return &m, nil
}

func (m *Map) validate() error {
	roles := map[string]bool{}
	for _, r := range m.Roles {
		name := normRole(r)
		if name == "" {
			return fmt.Errorf("berries: %s: a role has no name", MapFile)
		}
		if name == RoleOwner || name == RoleAny {
			return fmt.Errorf("berries: %s: role %q is reserved; owner and any are built in", MapFile, r)
		}
		if roles[name] {
			return fmt.Errorf("berries: %s: role %q is listed twice", MapFile, r)
		}
		roles[name] = true
	}
	shapes := map[string]string{}
	for i := range m.Routes {
		r := &m.Routes[i]
		if err := r.parse(); err != nil {
			return err
		}
		if len(r.Allow) == 0 {
			return fmt.Errorf("berries: %s: route %q allows nobody; list at least one role, owner or any", MapFile, r.Pattern)
		}
		for _, a := range r.Allow {
			name := normRole(a)
			if name != RoleOwner && name != RoleAny && !roles[name] {
				return fmt.Errorf("berries: %s: route %q allows %q, which is not in roles", MapFile, r.Pattern, a)
			}
		}
		if strings.TrimSpace(r.Source) == "" {
			return fmt.Errorf("berries: %s: route %q has no source; name the PRD, book or person the rule comes from", MapFile, r.Pattern)
		}
		if r.Status != StatusProposed && r.Status != StatusApproved {
			return fmt.Errorf("berries: %s: route %q has status %q; use proposed or approved", MapFile, r.Pattern, r.Status)
		}
		shape := r.shape()
		if other, ok := shapes[shape]; ok {
			return fmt.Errorf("berries: %s: routes %q and %q match the same requests", MapFile, other, r.Pattern)
		}
		shapes[shape] = r.Pattern
	}
	return nil
}

func (r *Route) parse() error {
	method, p, ok := strings.Cut(strings.TrimSpace(r.Pattern), " ")
	method = strings.ToUpper(method)
	p = strings.TrimSpace(p)
	if !ok || !mapMethods[method] {
		return fmt.Errorf("berries: %s: route %q needs a method and a path, such as GET /orders/{id}", MapFile, r.Pattern)
	}
	if !strings.HasPrefix(p, "/") || strings.ContainsAny(p, "?# ") {
		return fmt.Errorf("berries: %s: route %q needs a path starting with / and no query", MapFile, r.Pattern)
	}
	r.method = method
	r.segments = splitPath(p)
	for _, s := range r.segments {
		if strings.HasPrefix(s, "{") != strings.HasSuffix(s, "}") || s == "{}" {
			return fmt.Errorf("berries: %s: route %q has a malformed {param} segment %q", MapFile, r.Pattern, s)
		}
	}
	return nil
}

// shape is the route with every parameter name dropped, so two routes that match the same requests collide.
func (r Route) shape() string {
	parts := make([]string, len(r.segments))
	for i, s := range r.segments {
		if isParam(s) {
			parts[i] = "{}"
		} else {
			parts[i] = s
		}
	}
	return r.method + " /" + strings.Join(parts, "/")
}

// Match finds the route for a request path relative to the base URL. Among several matches the one with the most
// literal segments wins, so /orders/export beats /orders/{id}.
func (m *Map) Match(method, requestPath string) (*Route, bool) {
	method = strings.ToUpper(method)
	segs := splitPath(requestPath)
	var best *Route
	bestLiterals := -1
	for i := range m.Routes {
		r := &m.Routes[i]
		if r.method != method || len(r.segments) != len(segs) {
			continue
		}
		literals, ok := 0, true
		for j, s := range r.segments {
			if isParam(s) {
				continue
			}
			if s != segs[j] {
				ok = false
				break
			}
			literals++
		}
		if ok && literals > bestLiterals {
			best, bestLiterals = r, literals
		}
	}
	return best, best != nil
}

// Allows reports whether the rule lets a user with these tags call the route on an object recorded by owner.
func (r Route) Allows(user string, tags []string, owner string) bool {
	for _, a := range r.Allow {
		switch name := normRole(a); name {
		case RoleAny:
			return true
		case RoleOwner:
			if user == owner {
				return true
			}
		default:
			for _, t := range tags {
				if normRole(t) == name {
					return true
				}
			}
		}
	}
	return false
}

func splitPath(p string) []string {
	p = strings.Trim(p, "/")
	if p == "" {
		return nil
	}
	return strings.Split(p, "/")
}

func isParam(s string) bool { return strings.HasPrefix(s, "{") && strings.HasSuffix(s, "}") }

// normRole compares roles the way rester tags are typed: case and surrounding space do not matter.
func normRole(s string) string { return strings.ToLower(strings.TrimSpace(s)) }
