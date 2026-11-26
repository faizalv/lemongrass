// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

package berries

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
)

// Response is what one replay got back, in the shape an api channel returns it: a JSON body already decoded, any
// other body as a string.
type Response struct {
	Status  int
	Headers map[string][]string
	Body    any
}

// Requester sends one request through the channel as user. Every request berries makes goes through it, so the
// channel's scope, login and audit apply to replays exactly as to the calls they were recorded from.
type Requester interface {
	Do(user, method, target string, body []byte, contentType string, headers []Header) (Response, error)
}

// User is a channel user as the channel lists it: its handle and its tags, which are the map's roles.
type User struct {
	Name string
	Tags []string
}

// Closure states of an authorization pair, after Strix's: confirmed and open_proof_gap are stains, ruled_out names
// the control that held.
const (
	StateConfirmed    = "confirmed"
	StateOpenProofGap = "open_proof_gap"
	StateRuledOut     = "ruled_out"
)

// Coverage outcomes of a row, clean ones included, so a run shows what was tested and not only what failed.
const (
	OutcomeAllowed        = "allowed"
	OutcomeOverRestricted = "over_restricted"
	OutcomeError          = "error"
	OutcomeControlFailed  = "control_failed"
)

// Check ids of the authorization tier.
const (
	CheckVertical   = "authz.vertical"
	CheckHorizontal = "authz.horizontal"
)

var safeMethods = map[string]bool{"GET": true, "HEAD": true, "OPTIONS": true}

// Evidence is one response kept for a finding: enough to see what happened without keeping the whole body.
type Evidence struct {
	User    string `json:"user"`
	Status  int    `json:"status"`
	Digest  string `json:"digest,omitempty"`
	Preview string `json:"preview,omitempty"`
}

// Finding is one stain candidate from a run.
type Finding struct {
	Check       string    `json:"check"`
	State       string    `json:"state"`
	Rester      string    `json:"rester"`
	Environment string    `json:"environment"`
	Route       string    `json:"route"`
	Path        string    `json:"path"`
	SeedID      string    `json:"seed"`
	Owner       string    `json:"owner,omitempty"`
	User        string    `json:"user,omitempty"`
	Detail      string    `json:"detail"`
	Rule        string    `json:"rule,omitempty"`
	Control     *Evidence `json:"control,omitempty"`
	Replay      *Evidence `json:"replay,omitempty"`
	// discriminator separates findings of one check on one route, such as two cookies.
	discriminator string
}

// Key identifies a finding across runs. Authorization findings are per pair of users; passive ones per route.
func (f Finding) Key() string {
	return strings.Join([]string{f.Check, f.Rester, f.Environment, f.Route, f.Owner, f.User, f.discriminator}, "\x00")
}

// Scope is what a run must have tested for the absence of a finding to mean it is fixed.
func (f Finding) Scope() string {
	if strings.HasPrefix(f.Check, "authz.") {
		return f.Key()
	}
	return strings.Join([]string{f.Check, f.Rester, f.Environment, f.Route}, "\x00")
}

// Row is one coverage line: a seed replayed as one user and what came of it.
type Row struct {
	Route    string `json:"route"`
	SeedID   string `json:"seed"`
	Owner    string `json:"owner"`
	User     string `json:"user"`
	Expected string `json:"expected"`
	Outcome  string `json:"outcome"`
	Status   int    `json:"status,omitempty"`
	Control  string `json:"control,omitempty"`
}

// Run is one scan of one rester environment.
type Run struct {
	ID          int       `json:"id"`
	Rester      string    `json:"rester"`
	Environment string    `json:"environment"`
	At          time.Time `json:"at"`
	Seeds       int       `json:"seeds"`
	Replayed    int       `json:"replayed"`
	SkipWrites  int       `json:"skipped_writes"`
	SkipUsers   int       `json:"skipped_user_not_on_channel"`
	Unmapped    []string  `json:"unmapped"`
	Rows        []Row     `json:"rows"`
	Findings    []Finding `json:"findings"`
	Tested      []string  `json:"-"`
}

// RunInput is everything a run needs. Seeds must already be those of Rester and Environment.
type RunInput struct {
	Rester      string
	Environment string
	HTTPS       bool
	Users       []User
	Map         *Map
	Seeds       []Seed
	Requester   Requester
	// OnlySeed and OnlyUser narrow a run to one pair, for confirming a single stain.
	OnlySeed string
	OnlyUser string
}

// Execute replays every safe, mapped seed as its own user (the control) and as every other channel user, and judges
// each answer against the map. It never sends a request that is not a recorded seed.
func Execute(in RunInput) *Run {
	run := &Run{Rester: in.Rester, Environment: in.Environment, At: time.Now().UTC()}
	tags := map[string][]string{}
	for _, u := range in.Users {
		tags[u.Name] = u.Tags
	}
	tested := map[string]bool{}
	unmapped := map[string]bool{}
	passiveSeen := map[string]bool{}

	addFinding := func(f Finding) {
		tested[f.Scope()] = true
		run.Findings = append(run.Findings, f)
	}
	inspect := func(route *Route, seed Seed, user string, resp Response) {
		for _, f := range passiveChecks(resp, seed.Method, in.HTTPS) {
			f.Rester, f.Environment, f.Route, f.Path, f.SeedID = in.Rester, in.Environment, route.Pattern, seed.Path, seed.ID
			f.Replay = evidence(user, resp)
			tested[f.Scope()] = true
			if passiveSeen[f.Key()] {
				continue
			}
			passiveSeen[f.Key()] = true
			run.Findings = append(run.Findings, f)
		}
		for _, check := range passiveCheckIDs {
			tested[strings.Join([]string{check, in.Rester, in.Environment, route.Pattern}, "\x00")] = true
		}
	}

	for _, seed := range in.Seeds {
		if in.OnlySeed != "" && seed.ID != in.OnlySeed {
			continue
		}
		run.Seeds++
		route, ok := in.Map.Match(seed.Method, seed.Path)
		if !ok {
			unmapped[seed.Method+" "+seed.Path] = true
			continue
		}
		if !safeMethods[seed.Method] {
			run.SkipWrites++
			continue
		}
		if _, ok := tags[seed.User]; !ok {
			run.SkipUsers++
			continue
		}
		run.Replayed++
		rule := describeRule(route)

		control, err := in.Requester.Do(seed.User, seed.Method, seed.Target, seed.Body, seed.ContentType, seed.Headers)
		if err != nil || !is2xx(control.Status) {
			row := Row{Route: route.Pattern, SeedID: seed.ID, Owner: seed.User, User: seed.User, Expected: "allow", Outcome: OutcomeControlFailed, Status: control.Status}
			if err != nil {
				row.Control = err.Error()
			}
			run.Rows = append(run.Rows, row)
			continue
		}
		inspect(route, seed, seed.User, control)
		controlDigest := Digest(control.Body)

		// The owner's own success is a finding when the map does not let its role in: the seed alone proves it.
		if (in.OnlyUser == "" || in.OnlyUser == seed.User) && !route.Allows(seed.User, tags[seed.User], seed.User) {
			f := Finding{Check: CheckVertical, State: StateConfirmed, Rester: in.Rester, Environment: in.Environment, Route: route.Pattern, Path: seed.Path,
				SeedID: seed.ID, User: seed.User, Rule: rule, Replay: evidence(seed.User, control),
				Detail: fmt.Sprintf("%s got %d with content, but the map allows %s", seed.User, control.Status, strings.Join(route.Allow, ", "))}
			addFinding(f)
			run.Rows = append(run.Rows, Row{Route: route.Pattern, SeedID: seed.ID, Owner: seed.User, User: seed.User, Expected: "deny", Outcome: StateConfirmed, Status: control.Status})
		} else {
			run.Rows = append(run.Rows, Row{Route: route.Pattern, SeedID: seed.ID, Owner: seed.User, User: seed.User, Expected: "allow", Outcome: OutcomeAllowed, Status: control.Status})
		}

		for _, u := range in.Users {
			if u.Name == seed.User || (in.OnlyUser != "" && u.Name != in.OnlyUser) {
				continue
			}
			allowed := route.Allows(u.Name, u.Tags, seed.User)
			check := CheckVertical
			if sharesRole(u.Tags, tags[seed.User]) {
				check = CheckHorizontal
			}
			base := Finding{Check: check, Rester: in.Rester, Environment: in.Environment, Route: route.Pattern, Path: seed.Path,
				SeedID: seed.ID, Owner: seed.User, User: u.Name, Rule: rule, Control: evidence(seed.User, control)}
			if !allowed {
				tested[base.Scope()] = true
			}
			row := Row{Route: route.Pattern, SeedID: seed.ID, Owner: seed.User, User: u.Name, Expected: expectation(allowed)}

			resp, err := in.Requester.Do(u.Name, seed.Method, seed.Target, seed.Body, seed.ContentType, seed.Headers)
			if err != nil {
				row.Outcome, row.Control = OutcomeError, err.Error()
				run.Rows = append(run.Rows, row)
				continue
			}
			row.Status = resp.Status
			inspect(route, seed, u.Name, resp)
			denied, control := denyShaped(resp, control)

			switch {
			case allowed && denied:
				row.Outcome, row.Control = OutcomeOverRestricted, control
			case allowed:
				row.Outcome = OutcomeAllowed
			case denied:
				row.Outcome, row.Control = StateRuledOut, control
			case Digest(resp.Body) == controlDigest:
				row.Outcome = StateConfirmed
				base.State = StateConfirmed
				base.Replay = evidence(u.Name, resp)
				base.Detail = fmt.Sprintf("%s got %d with the same content as %s; the map allows %s", u.Name, resp.Status, seed.User, strings.Join(route.Allow, ", "))
				addFinding(base)
			default:
				row.Outcome = StateOpenProofGap
				base.State = StateOpenProofGap
				base.Replay = evidence(u.Name, resp)
				base.Detail = fmt.Sprintf("%s got %d with other content than %s; the map allows %s, so check whether this content belongs to %s", u.Name, resp.Status, seed.User, strings.Join(route.Allow, ", "), u.Name)
				addFinding(base)
			}
			run.Rows = append(run.Rows, row)
		}
	}
	for p := range unmapped {
		run.Unmapped = append(run.Unmapped, p)
	}
	sort.Strings(run.Unmapped)
	for s := range tested {
		run.Tested = append(run.Tested, s)
	}
	return run
}

// denyShaped reports whether resp shows the request was refused, and names the control that refused it. An empty
// body where the owner's was not counts, since an app often enforces access by filtering rather than refusing.
func denyShaped(resp, owner Response) (bool, string) {
	if !is2xx(resp.Status) {
		return true, fmt.Sprintf("status %d", resp.Status)
	}
	if Empty(resp.Body) && !Empty(owner.Body) {
		return true, "empty body where the owner's was not"
	}
	return false, ""
}

func expectation(allowed bool) string {
	if allowed {
		return "allow"
	}
	return "deny"
}

func sharesRole(a, b []string) bool {
	for _, x := range a {
		for _, y := range b {
			if normRole(x) == normRole(y) {
				return true
			}
		}
	}
	return false
}

func describeRule(r *Route) string {
	return fmt.Sprintf("%s allows %s (%s, source: %s)", r.Pattern, strings.Join(r.Allow, ", "), r.Status, r.Source)
}

func is2xx(status int) bool { return status >= 200 && status < 300 }

// Digest is a stable fingerprint of a body: a decoded JSON body is re-encoded first, which sorts object keys, so the
// same content compares equal however the server ordered it.
func Digest(body any) string {
	data, err := json.Marshal(body)
	if err != nil {
		data = []byte(fmt.Sprint(body))
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])[:16]
}

// Empty reports a body that carries no content: nothing, blank text, null, an empty array or an empty object.
func Empty(body any) bool {
	switch b := body.(type) {
	case nil:
		return true
	case string:
		return strings.TrimSpace(b) == ""
	case []any:
		return len(b) == 0
	case map[string]any:
		return len(b) == 0
	}
	return false
}

const previewLimit = 600

func evidence(user string, resp Response) *Evidence {
	return &Evidence{User: user, Status: resp.Status, Digest: Digest(resp.Body), Preview: preview(resp.Body)}
}

func preview(body any) string {
	var text string
	if s, ok := body.(string); ok {
		text = s
	} else if data, err := json.Marshal(body); err == nil {
		text = string(data)
	}
	if len(text) > previewLimit {
		text = text[:previewLimit] + "..."
	}
	return text
}
