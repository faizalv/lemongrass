// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

package main

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/faizalv/lemongrass/internal/agent"
	"github.com/faizalv/lemongrass/internal/berries"
	"github.com/faizalv/lemongrass/internal/config"
	"github.com/faizalv/lemongrass/internal/project"
	"github.com/faizalv/lemongrass/internal/restergate"
	"github.com/faizalv/lemongrass/internal/vault"
)

const berriesUsage = `usage:
  lgrass berries map [<short-id>]
  lgrass berries seeds
  lgrass berries run <short-id>
  lgrass berries stains [--all]
  lgrass berries stain <stain-id>
  lgrass berries replay <short-id> <stain-id>

Berries tests a project's api for access-control holes without writing any probe. The project's
access map is .berries/map.yaml: its roles, which are the channel users' tags, and for each route
who may call it. Every call made with lgrass rester in a project with a map is recorded as a seed
once it succeeds. A run replays each GET, HEAD and OPTIONS seed as its own user, then as every
other user on the channel, and judges each answer against the map.

map prints the routes with their seed counts and the requests no route covers; with a channel it
also names the roles that have too few users to test. seeds lists the recorded requests. run
replays the seeds of the channel's rester and environment and prints the stains it found or
fixed. stains lists the open stains, --all includes fixed and accepted ones. stain prints one
stain with the rule and the pair of responses that prove it. replay runs one stain's pair again
to confirm a fix.

A stain is confirmed when a user the map denies got the same content as the seed's own user,
and open_proof_gap when that user got other content, which needs a look at whose content it is.
Accepting a stain is the human's call, made in the Berries pane.
`

func cmdBerries(args []string) {
	if len(args) == 0 || args[0] == "-h" || args[0] == "--help" {
		fmt.Print(berriesUsage)
		return
	}
	proj := currentProject()
	store := berries.Store{Dir: berries.StoreDir(config.Dir(), proj.ID)}
	out, err := runBerries(&agent.Client{SocketPath: agent.SocketPath()}, proj, store, args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	fmt.Print(out)
}

func runBerries(client *agent.Client, proj project.Project, store berries.Store, args []string) (string, error) {
	m, err := berries.LoadMap(proj.Path)
	if errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("this project has no %s; lgrass how berries map explains how to write one", berries.MapFile)
	}
	if err != nil {
		return "", err
	}
	cmd, rest := args[0], args[1:]
	switch cmd {
	case "map":
		if len(rest) > 1 {
			return "", fmt.Errorf("map takes at most a channel id\n\n%s", berriesUsage)
		}
		seeds, err := store.Seeds()
		if err != nil {
			return "", err
		}
		var users []berries.User
		if len(rest) == 1 {
			info, err := apiChannel(client, rest[0])
			if err != nil {
				return "", err
			}
			users = channelUsers(info)
			seeds = seedsOf(seeds, info)
		}
		return berries.FormatMap(m, seeds, users), nil
	case "seeds":
		if len(rest) != 0 {
			return "", fmt.Errorf("seeds takes no arguments\n\n%s", berriesUsage)
		}
		seeds, err := store.Seeds()
		if err != nil {
			return "", err
		}
		return berries.FormatSeeds(m, seeds), nil
	case "run":
		if len(rest) != 1 {
			return "", fmt.Errorf("run needs a channel id\n\n%s", berriesUsage)
		}
		return berriesRun(client, store, m, rest[0], "", "")
	case "stains":
		all := len(rest) == 1 && rest[0] == "--all"
		if len(rest) > 1 || (len(rest) == 1 && !all) {
			return "", fmt.Errorf("stains takes only --all\n\n%s", berriesUsage)
		}
		stains, err := store.Stains()
		if err != nil {
			return "", err
		}
		return berries.FormatStains(stains, all), nil
	case "stain":
		if len(rest) != 1 {
			return "", fmt.Errorf("stain needs a stain id\n\n%s", berriesUsage)
		}
		s, err := findStain(store, rest[0])
		if err != nil {
			return "", err
		}
		return berries.FormatStain(s), nil
	case "replay":
		if len(rest) != 2 {
			return "", fmt.Errorf("replay needs a channel id and a stain id\n\n%s", berriesUsage)
		}
		s, err := findStain(store, rest[1])
		if err != nil {
			return "", err
		}
		user := s.Finding.User
		if user == "" && s.Finding.Replay != nil {
			user = s.Finding.Replay.User
		}
		return berriesRun(client, store, m, rest[0], s.Finding.SeedID, user)
	}
	return "", fmt.Errorf("unknown berries command %q\n\n%s", cmd, berriesUsage)
}

// berriesRun runs the authz and passive tiers on one channel, narrowed to one seed and user when a stain is replayed,
// and folds the result into the stain list.
func berriesRun(client *agent.Client, store berries.Store, m *berries.Map, shortID, onlySeed, onlyUser string) (string, error) {
	info, err := apiChannel(client, shortID)
	if err != nil {
		return "", err
	}
	all, err := store.Seeds()
	if err != nil {
		return "", err
	}
	seeds := seedsOf(all, info)
	if onlySeed != "" {
		found := false
		for _, s := range seeds {
			found = found || s.ID == onlySeed
		}
		if !found {
			return "", fmt.Errorf("seed %s is not recorded for %s/%s; replay with a channel on the stain's rester and environment, or record the request again with lgrass rester", onlySeed, info.Rester, info.Environment)
		}
	}
	run := berries.Execute(berries.RunInput{
		Rester: info.Rester, Environment: info.Environment, HTTPS: strings.HasPrefix(info.BaseURL, "https://"),
		Users: channelUsers(info), Map: m, Seeds: seeds,
		Requester: channelRequester{client: client, shortID: shortID},
		OnlySeed:  onlySeed, OnlyUser: onlyUser,
	})
	run.ID = store.NextRunID()
	stains, err := store.Stains()
	if err != nil {
		return "", err
	}
	stains = berries.Merge(stains, run)
	if err := store.SaveRun(run); err != nil {
		return "", err
	}
	if err := store.SaveStains(stains); err != nil {
		return "", err
	}
	return berries.FormatRun(run, stains), nil
}

func apiChannel(client *agent.Client, shortID string) (restergate.HTTPChannelInfo, error) {
	info, err := client.HTTPChannelInfo(shortID)
	if err != nil {
		return info, err
	}
	if info.Type != "api" {
		return info, fmt.Errorf("berries tests api channels; this channel is %s", info.Type)
	}
	if info.Status != "active" {
		return info, fmt.Errorf("this channel is %s; ask the human for an active one", info.Status)
	}
	return info, nil
}

func channelUsers(info restergate.HTTPChannelInfo) []berries.User {
	users := make([]berries.User, 0, len(info.Users))
	for _, u := range info.Users {
		users = append(users, berries.User{Name: u.Name, Tags: u.Tags})
	}
	return users
}

func seedsOf(seeds []berries.Seed, info restergate.HTTPChannelInfo) []berries.Seed {
	var out []berries.Seed
	for _, s := range seeds {
		if s.Rester == info.Rester && s.Environment == info.Environment {
			out = append(out, s)
		}
	}
	return out
}

func findStain(store berries.Store, id string) (berries.Stain, error) {
	stains, err := store.Stains()
	if err != nil {
		return berries.Stain{}, err
	}
	for _, s := range stains {
		if s.ID == id {
			return s, nil
		}
	}
	return berries.Stain{}, fmt.Errorf("no stain %s; lgrass berries stains --all lists them", id)
}

// channelRequester sends berries' replays through the agent like any lgrass rester call, so they are scoped,
// logged in and audited the same way.
type channelRequester struct {
	client  *agent.Client
	shortID string
}

func (r channelRequester) Do(user, method, target string, body []byte, contentType string, headers []berries.Header) (berries.Response, error) {
	hs := make([]vault.Header, 0, len(headers))
	for _, h := range headers {
		hs = append(hs, vault.Header{Name: h.Name, Value: h.Value})
	}
	res, err := r.client.RequestHTTP(r.shortID, user, method, target, body, contentType, hs, actorFor(r.shortID))
	return berries.Response{Status: res.Status, Headers: res.Headers, Body: res.Body}, err
}

// maxSeedBody keeps an upload's bytes out of the seed store; such a call is a write, which this slice never replays.
const maxSeedBody = 1 << 20

// seedStore finds where a call made from the working directory records its seed, and reports false when the
// project has no map. A variable so tests can point it at a temporary project.
var seedStore = func() (berries.Store, bool) {
	proj, err := project.ResolveCwd()
	if err != nil || !berries.HasMap(proj.Path) {
		return berries.Store{}, false
	}
	return berries.Store{Dir: berries.StoreDir(config.Dir(), proj.ID)}, true
}

// recordSeed keeps a successful rester call as a berries seed when the project has a map. It never fails the call it
// follows: a seed that cannot be recorded is reported on stderr and the call's own output is unchanged.
func recordSeed(client *agent.Client, cmd resterCommand, body []byte, contentType string, result restergate.HTTPResult) {
	if result.Status < 200 || result.Status >= 300 || len(body) > maxSeedBody {
		return
	}
	store, ok := seedStore()
	if !ok {
		return
	}
	if err := saveSeed(client, store, cmd, body, contentType, result); err != nil {
		fmt.Fprintf(os.Stderr, "berries: seed not recorded: %v\n", err)
	}
}

func saveSeed(client *agent.Client, store berries.Store, cmd resterCommand, body []byte, contentType string, result restergate.HTTPResult) error {
	info, err := client.HTTPChannelInfo(cmd.shortID)
	if err != nil {
		return err
	}
	user := vault.UserHandle(cmd.user)
	if user == "" && len(info.Users) == 1 {
		user = info.Users[0].Name
	}
	if user == "" {
		return fmt.Errorf("the call's user is unknown")
	}
	path, query, err := restergate.ResolvePath(info.BaseURL, cmd.path)
	if err != nil {
		return err
	}
	headers := make([]berries.Header, 0, len(cmd.headers))
	for _, h := range cmd.headers {
		headers = append(headers, berries.Header{Name: h.Name, Value: h.Value})
	}
	_, err = store.SaveSeed(berries.Seed{
		Rester: info.Rester, Environment: info.Environment, User: user,
		Method: cmd.method, Path: path, Query: query, Target: cmd.path,
		ContentType: contentType, Body: body, Headers: headers,
		Status: result.Status, Digest: berries.Digest(result.Body), RecordedAt: time.Now().UTC(),
	})
	return err
}
