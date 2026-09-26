package main

import (
	"fmt"
	"os"
	"strconv"

	"github.com/faizalv/lemongrass/session"
	"github.com/faizalv/lemongrass/workgroup"
)

func cmdWorkgroup(args []string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: lgrass workgroup <create|disband|thread> ...")
		os.Exit(1)
	}
	switch args[0] {
	case "create":
		cmdWorkgroupCreate(args[1:])
	case "disband":
		cmdWorkgroupDisband(args[1:])
	case "thread":
		cmdWorkgroupThread(args[1:])
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
	pilotVendor, _ := store.VendorForTab(tabID)
	proj := currentProject()

	members := make([]workgroup.SpawnMember, len(cfg.Copilots))
	for i, c := range cfg.Copilots {
		members[i] = workgroup.SpawnMember{
			TabID:  workgroup.NewTabID(),
			Label:  c.Label,
			Vendor: c.Vendor,
			Model:  c.Model,
			Prompt: workgroup.ComposePrompt(c.Vendor, c.Label, cfg.Name, cfg.PilotLabel, c.RequiredSkills(), c.Prompt),
			Skills: c.RequiredSkills(),
		}
	}
	req := workgroup.Request{ProjectPath: proj.Path, PilotTabID: tabID, PilotLabel: cfg.PilotLabel, GroupName: cfg.Name, Members: members}

	fmt.Println("lgrass: waiting for the human to approve this workgroup in the lemongrass app.")
	token, approved, err := workgroup.Propose(workgroup.AppSocketPath(), req)
	if err != nil {
		fail(err)
	}
	if !approved {
		fmt.Fprintln(os.Stderr, "lgrass workgroup: the human declined, nothing was created")
		os.Exit(1)
	}

	group, err := store.CreateGroup(cfg.Name, session.Member{TabID: tabID, Label: cfg.PilotLabel, Vendor: pilotVendor}, toMembers(members, cfg.Copilots))
	if err != nil {
		fail(err)
	}
	if err := workgroup.Spawn(workgroup.AppSocketPath(), token); err != nil {
		store.DeleteGroup(group.ID)
		fail(err)
	}

	fmt.Printf("workgroup %d created, group thread %d.\n", group.ID, group.ThreadID)
	for _, m := range members {
		fmt.Printf("  %s (%s) tab %s\n", m.Label, m.Vendor, m.TabID)
	}
}

func toMembers(spawn []workgroup.SpawnMember, copilots []workgroup.Copilot) []session.Member {
	out := make([]session.Member, len(spawn))
	for i, m := range spawn {
		out[i] = session.Member{TabID: m.TabID, Label: m.Label, Vendor: m.Vendor, Prompt: copilots[i].Prompt, Skills: copilots[i].Skills}
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
	if tabID != "" && tabID != group.PilotTabID {
		fmt.Fprintf(os.Stderr, "lgrass workgroup: only the pilot can disband workgroup %d\n", id)
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
	header := session.FormatGroupHeader(group, members)
	for _, m := range members {
		if m.TabID == tabID {
			header += "\n" + session.FormatMemberHeader(m)
		}
	}
	printThread(store, group.ThreadID, parsed, header)
}
