// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

import type { WorkgroupThreadMessage } from '../../preload/types'

// Adds newly read messages to the ones already held, by id, oldest first, so a poll that reads
// only the newest page never drops what was paged in before.
export function mergeMessages(
  held: WorkgroupThreadMessage[],
  incoming: WorkgroupThreadMessage[]
): WorkgroupThreadMessage[] {
  const byId = new Map(held.map((message) => [message.id, message]))
  for (const message of incoming) byId.set(message.id, message)
  return [...byId.values()].sort((a, b) => a.id - b.id)
}
