<!-- SPDX-License-Identifier: Apache-2.0 -->
<!-- Copyright (C) 2026 The Sankasten Authors -->

<script setup lang="ts">
import { ref } from 'vue'
import {
  AppCheckbox,
  AppInput,
  AppMultiPicker,
  AppSegmented,
  AppSwitcher,
  AppTextarea,
  type PickerGroup
} from '../../src'
import DemoRow from '../parts/DemoRow.vue'
import DocBlock from '../parts/DocBlock.vue'

const sizes = ['md', 'sm'] as const

const text = ref('A value')
const port = ref(5432)
const note = ref('Several lines of text.\nThe textarea is not on the height scale.')

const branches = [
  { id: 'main', label: 'main', group: 'Local' },
  { id: 'next', label: 'next', group: 'Local', hint: '2 ahead' },
  { id: 'old', label: 'old-release', group: 'Local', disabled: 'Checked out elsewhere' },
  { id: 'origin/main', label: 'origin/main', group: 'Remote' }
]
const branch = ref<string | null>('main')

const groups: PickerGroup[] = [
  {
    label: 'Tables',
    items: ['orders', 'users', 'invoices'].map((t) => ({ value: t, label: t }))
  },
  { label: 'Views', items: [{ value: 'daily', label: 'daily_totals' }] }
]
const picked = ref<string[]>(['orders'])

const mode = ref('diff')
const checks = ref({ a: true, b: false })
</script>

<template>
  <section id="fields" class="section">
    <h2>Fields</h2>

    <DocBlock name="AppInput">
      <DemoRow v-for="s in sizes" :key="s" :label="s">
        <AppInput v-model="text" :size="s" style="width: 180px" />
        <AppInput
          model-value=""
          :size="s"
          icon="search"
          placeholder="Filter"
          style="width: 160px"
        />
        <AppInput v-model.number="port" :size="s" type="number" mono style="width: 120px" />
        <AppInput model-value="Disabled" :size="s" disabled style="width: 120px" />
      </DemoRow>
    </DocBlock>

    <DocBlock name="AppTextarea">
      <AppTextarea v-model="note" rows="3" />
      <AppTextarea v-model="note" size="sm" mono rows="2" resize="none" />
    </DocBlock>

    <DocBlock name="AppSwitcher">
      <DemoRow v-for="s in sizes" :key="s" :label="`plain and framed, ${s}`">
        <AppSwitcher
          :items="branches"
          :active-id="branch"
          placeholder="Branch"
          empty-text="No branches"
          :size="s"
          @select="(id) => (branch = id)"
        />
        <AppSwitcher
          :items="branches"
          :active-id="branch"
          placeholder="Branch"
          empty-text="No branches"
          framed
          searchable
          :size="s"
          style="width: 200px"
          @select="(id) => (branch = id)"
        />
      </DemoRow>
    </DocBlock>

    <DocBlock name="AppMultiPicker">
      <DemoRow v-for="s in sizes" :key="s" :label="s">
        <AppMultiPicker
          v-model="picked"
          :groups="groups"
          :summary="`${picked.length} selected`"
          :size="s"
          style="width: 260px"
        />
      </DemoRow>
    </DocBlock>

    <DocBlock name="AppSegmented">
      <DemoRow v-for="s in sizes" :key="s" :label="s">
        <AppSegmented
          v-model="mode"
          :size="s"
          aria-label="View"
          :options="[
            { value: 'diff', label: 'Diff' },
            { value: 'blame', label: 'Blame' },
            { value: 'history', label: 'History' }
          ]"
        />
      </DemoRow>
    </DocBlock>

    <DocBlock name="AppCheckbox">
      <DemoRow v-for="s in sizes" :key="s" :label="s">
        <AppCheckbox :checked="checks.a" :size="s" @click="checks.a = !checks.a">
          Include untracked files
        </AppCheckbox>
        <AppCheckbox :checked="checks.b" :size="s" @click="checks.b = !checks.b">Amend</AppCheckbox>
        <AppCheckbox :checked="false" indeterminate :size="s">Some selected</AppCheckbox>
      </DemoRow>
    </DocBlock>
  </section>
</template>
