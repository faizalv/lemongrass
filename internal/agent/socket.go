// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

package agent

import (
	"github.com/faizalv/lemongrass/internal/ipc"
)

func SocketPath() string {
	return ipc.Address("agent")
}
