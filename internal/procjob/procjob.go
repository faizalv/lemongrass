// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

// Package procjob ties a Windows process tree to a kill-on-close job object: closing the job, or the death of the
// process holding it, ends every process in it, the Windows counterpart of a process group plus a parent-death signal.
package procjob
