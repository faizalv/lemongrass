package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"

	"github.com/faizalv/lemongrass/session"
	"github.com/faizalv/lemongrass/workgroup"
)

func cmdWorkgroup(args []string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: lgrass workgroup <create|disband|thread|list> ...")
		os.Exit(1)
	}
	switch args[0] {
	case "create":
		cmdWorkgroupCreate(args[1:])
	case "disband":
		cmdWorkgroupDisband(args[1:])
	case "thread":
		cmdWorkgroupThread(args[1:])
	case "list":
		cmdWorkgroupList(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "unknown workgroup command: %s\n", args[0])
		os.Exit(1)
	}
}

func cmdWorkgroupCreate(args []string) {
	if len(args) != 1 {
		fmt.Fprintln(os.Stderr, "usage: lgrass workgroup create <path-to-config>")
		os.Exit(1)
	}
	if tabID == "" {
		fmt.Fprintf(os.Stderr, "lgrass workgroup: creating a group needs a lemongrass tab, %s is not set\n", tabIDEnv)
		os.Exit(1)
	}
	cfg, err := workgroup.Load(args[0])
	if err != nil {
		fail(err)
	}

	if home, err := os.UserHomeDir(); err == nil {
		if err := cfg.CheckSkillsInstalled(home); err != nil {
			fail(err)
		}
	}

	store := openStore()
	defer store.Close()
	if _, err := store.LiveGroupForTab(tabID); err == nil {
		fail(session.ErrAlreadyInside)
	}
	if err := store.CheckGroupName(cfg.Name); err != nil {
		fail(err)
	}
	leaderVendor, _ := store.VendorForTab(tabID)
	proj := currentProject()

	members := make([]workgroup.SpawnMember, len(cfg.Thinkers))
	for i, c := range cfg.Thinkers {
		members[i] = workgroup.SpawnMember{
			TabID:  workgroup.NewTabID(),
			Label:  c.Label,
			Vendor: c.Vendor,
			Model:  c.Model,
			Prompt: workgroup.ComposePrompt(c.Label, cfg.Name, cfg.LeaderLabel, c.RequiredSkills(), c.Prompt),
			Skills: c.RequiredSkills(),
		}
	}
	req := workgroup.Request{ProjectPath: proj.Path, LeaderTabID: tabID, LeaderLabel: cfg.LeaderLabel, GroupName: cfg.Name, Members: members}

	fmt.Println("lgrass: waiting for the human to approve this workgroup in the lemongrass app.")
	answer, err := workgroup.Propose(workgroup.AppSocketPath(), req)
	if err != nil {
		fail(err)
	}
	if !answer.Approved {
		fmt.Fprintln(os.Stderr, declinedMessage(answer))
		os.Exit(1)
	}
	token := answer.Token

	group, err := store.CreateGroup(cfg.Name, session.Member{TabID: tabID, Label: cfg.LeaderLabel, Vendor: leaderVendor}, toMembers(members, cfg.Thinkers))
	if err != nil {
		fail(err)
	}
	if err := workgroup.Spawn(workgroup.AppSocketPath(), token); err != nil {
		store.DeleteGroup(group.ID)
		fail(err)
	}

	fmt.Printf("workgroup %d created, group thread %s.\n", group.ID, group.ThreadID)
	for _, m := range members {
		fmt.Printf("  %s (%s) tab %s\n", m.Label, m.Vendor, m.TabID)
	}
}

func toMembers(spawn []workgroup.SpawnMember, thinkers []workgroup.Thinker) []session.Member {
	out := make([]session.Member, len(spawn))
	for i, m := range spawn {
		out[i] = session.Member{TabID: m.TabID, Label: m.Label, Vendor: m.Vendor, Prompt: thinkers[i].Prompt, Skills: thinkers[i].Skills}
	}
	return out
}

func cmdWorkgroupDisband(args []string) {
	if len(args) != 1 {
		fmt.Fprintln(os.Stderr, "usage: lgrass workgroup disband <workgroup-id>")
		os.Exit(1)
	}
	id, err := strconv.ParseInt(args[0], 10, 64)
	if err != nil {
		fmt.Fprintf(os.Stderr, "lgrass workgroup: %q is not a workgroup id\n", args[0])
		os.Exit(1)
	}

	store := openStore()
	defer store.Close()
	group, err := store.GroupByID(id)
	if err != nil {
		fail(err)
	}
	if tabID != "" && tabID != group.LeaderTabID {
		fmt.Fprintf(os.Stderr, "lgrass workgroup: only the leader can disband workgroup %d\n", id)
		os.Exit(1)
	}
	if err := store.DisbandGroup(id); err != nil {
		fail(err)
	}
	fmt.Printf("workgroup %d disbanded. Its tabs stay open and its thread stays readable.\n", id)
}

func cmdWorkgroupThread(args []string) {
	parsed := parseThreadArgs(args, defaultThreadReadLimit)
	if tabID == "" {
		fmt.Fprintf(os.Stderr, "lgrass workgroup: this needs a lemongrass tab, %s is not set\n", tabIDEnv)
		os.Exit(1)
	}

	store := openStore()
	defer store.Close()
	group, err := store.LiveGroupForTab(tabID)
	if err != nil {
		fmt.Fprintln(os.Stderr, "lgrass workgroup: this tab is not in a live workgroup")
		os.Exit(1)
	}
	members, err := store.GroupMembers(group.ID)
	if err != nil {
		fail(err)
	}
	var self session.Member
	for _, m := range members {
		if m.TabID == tabID {
			self = m
		}
	}
	header := func(firstRead bool) string {
		if !firstRead {
			return session.FormatGroupShort(group, self)
		}
		return session.FormatGroupHeader(group, members) + "\n" + session.FormatMemberHeader(self)
	}
	printThread(store, group.ThreadID, parsed, header)
}

type listedMember struct {
	TabID  string `json:"tabId"`
	Role   string `json:"role"`
	Label  string `json:"label"`
	Vendor string `json:"vendor"`
}

type listedGroup struct {
	ID          int64          `json:"id"`
	Name        string         `json:"name"`
	ThreadID    string         `json:"threadId"`
	LeaderTabID string         `json:"leaderTabId"`
	Members     []listedMember `json:"members"`
}

func cmdWorkgroupList(args []string) {
	asJSON := len(args) == 1 && args[0] == "--json"
	if len(args) > 1 || (len(args) == 1 && !asJSON) {
		fmt.Fprintln(os.Stderr, "usage: lgrass workgroup list [--json]")
		os.Exit(1)
	}

	store := openStore()
	defer store.Close()
	groups, err := store.LiveGroups()
	if err != nil {
		fail(err)
	}
	listed := make([]listedGroup, 0, len(groups))
	for _, g := range groups {
		members, err := store.GroupMembers(g.ID)
		if err != nil {
			fail(err)
		}
		entry := listedGroup{ID: g.ID, Name: g.Name, ThreadID: g.ThreadID, LeaderTabID: g.LeaderTabID, Members: make([]listedMember, len(members))}
		for i, m := range members {
			entry.Members[i] = listedMember{TabID: m.TabID, Role: m.Role, Label: m.Label, Vendor: m.Vendor}
		}
		listed = append(listed, entry)
	}

	if asJSON {
		json.NewEncoder(os.Stdout).Encode(listed)
		return
	}
	if len(listed) == 0 {
		fmt.Println("no live workgroups in this project")
		return
	}
	for _, g := range listed {
		fmt.Printf("workgroup %d [%s], thread %s\n", g.ID, g.Name, g.ThreadID)
		for _, m := range g.Members {
			fmt.Printf("  %s (%s, %s) tab %s\n", m.Label, m.Role, m.Vendor, m.TabID)
		}
	}
}

func declinedMessage(answer workgroup.Answer) string {
	switch {
	case answer.Withdrawn:
		return "lgrass workgroup: the proposal was withdrawn before the human answered, nothing was created"
	case answer.Reason != "":
		return fmt.Sprintf("lgrass workgroup: the human declined, nothing was created. Their reason: %s", answer.Reason)
	default:
		return "lgrass workgroup: the human declined without a reason, nothing was created"
	}
}
