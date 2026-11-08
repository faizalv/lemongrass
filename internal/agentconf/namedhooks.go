// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

package agentconf

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
)

// ReconcileNamedHooks owns one complete named entry; all other names belong to the user.
func ReconcileNamedHooks(data []byte, name string, desired any) ([]byte, bool, error) {
	top, err := namedHookEntries(data)
	if err != nil {
		return nil, false, err
	}
	want, err := json.Marshal(desired)
	if err != nil {
		return nil, false, err
	}
	if sameJSON(top[name], want) {
		return data, false, nil
	}
	top[name] = want
	out, err := json.MarshalIndent(top, "", "  ")
	return append(out, '\n'), true, err
}

func namedHookEntries(data []byte) (map[string]json.RawMessage, error) {
	top := map[string]json.RawMessage{}
	if len(bytes.TrimSpace(data)) > 0 {
		if err := json.Unmarshal(data, &top); err != nil {
			return nil, fmt.Errorf("agentconf: parsing named hooks: %w", err)
		}
	}
	if top == nil {
		return nil, fmt.Errorf("agentconf: named hooks must be an object")
	}
	return top, nil
}

func RemoveNamedHookConfig(path, name string) (bool, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	top, err := namedHookEntries(data)
	if err != nil {
		return false, err
	}
	if _, ok := top[name]; !ok {
		return false, nil
	}
	delete(top, name)
	if len(top) == 0 {
		return true, os.Remove(path)
	}
	out, err := json.MarshalIndent(top, "", "  ")
	if err != nil {
		return false, err
	}
	info, err := os.Stat(path)
	if err != nil {
		return false, err
	}
	return true, WriteAtomic(path, append(out, '\n'), info.Mode().Perm())
}
