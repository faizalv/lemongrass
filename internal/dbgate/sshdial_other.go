// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

//go:build !unix && !windows

package dbgate

import (
	"errors"
	"net"
	"os/exec"
)

func wireSSH(cmd *exec.Cmd) (net.Conn, func(), error) {
	return nil, nil, errors.New("dbgate: ssh connections are not supported on this platform")
}
