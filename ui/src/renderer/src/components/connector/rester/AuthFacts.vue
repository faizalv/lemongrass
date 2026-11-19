<!-- SPDX-License-Identifier: MPL-2.0 -->
<!-- Copyright (C) 2026 The Lemongrass Authors -->

<script setup lang="ts">
import { AppCodeValue } from '@sankasten'
import type { AuthFact } from '../../../authFacts'

// An authentication card's values as label and value rows. A value typed or sent as written is a
// code value that copies on click; the rest is plain text.
defineProps<{ facts: AuthFact[] }>()
</script>

<template>
  <dl class="vault-facts">
    <template v-for="fact in facts" :key="fact.label">
      <dt>{{ fact.label }}</dt>
      <dd>
        <span v-if="fact.prefix" class="vault-fact-prefix">{{ fact.prefix }}</span>
        <AppCodeValue v-if="fact.code" :value="fact.value" />
        <template v-else>{{ fact.value }}</template>
      </dd>
    </template>
  </dl>
</template>
