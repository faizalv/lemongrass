package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/faizalv/lemongrass/lgrassconf/agentconf"
)

var errAccountsUsage = errors.New("usage: lgrassd accounts <list|add|remove> ...")

func cmdAccounts(args []string) {
	home, err := os.UserHomeDir()
	if err != nil {
		fail(err)
	}
	self, err := os.Executable()
	if err != nil {
		fail(err)
	}
	if err := runAccounts(agentconf.Accounts{Home: home, HookBinary: self}, args, os.Stdout); err != nil {
		if errors.Is(err, errAccountsUsage) {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		fail(err)
	}
}

func runAccounts(accounts agentconf.Accounts, args []string, out io.Writer) error {
	switch {
	case len(args) == 1 && args[0] == "list":
		list, err := accounts.List()
		if err != nil {
			return err
		}
		return writeJSON(out, list)
	case len(args) == 2 && args[0] == "add":
		account, err := accounts.Add(args[1])
		if err != nil {
			return err
		}
		return writeJSON(out, account)
	case len(args) == 2 && args[0] == "remove":
		return accounts.Remove(args[1])
	}
	return errAccountsUsage
}

func writeJSON(out io.Writer, v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(out, string(data))
	return err
}
