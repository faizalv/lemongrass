<!-- SPDX-License-Identifier: Apache-2.0 -->
<!-- Copyright (C) 2026 The Sankasten Authors -->

<script setup lang="ts">
// The dropdown renders in body, fixed to the window at the trigger, so a modal or a scrolling pane
// that clips with overflow never crops it.
import AppIcon from './AppIcon.vue'
import IconFrame from './IconFrame.vue'
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import AppInput from './AppInput.vue'
import type { AppInputHandle } from './appInput'
import type { IconShape } from '../icons/appIcons'
import { placeFloat, placementStyle } from '../lib/floatPlace'

const props = withDefaults(
  defineProps<{
    // search is the text the filter box matches, the label when absent. disabled is the reason
    // the item cannot be picked, shown beside its label; hint is muted text in the same place for
    // an item that can be picked. group puts the item under a heading; items of one group must be
    // next to each other. icon is stroke shapes in a 14 by 14 box, drawn before the label and on the
    // trigger while the item is picked. A pinned item sits above the rest, past a divider, and
    // stays shown while the search filters the others; it is for a fixed entry that is not one of
    // the configured items.
    items: {
      id: string
      label: string
      title?: string
      search?: string
      disabled?: string
      hint?: string
      group?: string
      icon?: IconShape[]
      pinned?: boolean
    }[]
    activeId: string | null
    placeholder: string
    // Overrides what the closed trigger shows, such as an unsaved name; the list is unaffected.
    label?: string
    // Without it the dropdown has no add row.
    addLabel?: string
    emptyText: string
    // A framed trigger sits among form fields and reads as an input; the plain one sits in a header.
    framed?: boolean
    // sm is for dense rows, such as a rule list.
    size?: 'sm' | 'md'
    disabled?: boolean
    // Adds a filter box at the top of the dropdown.
    searchable?: boolean
    searchPlaceholder?: string
    // Prefers opening upward, for a trigger near the bottom of the window; either way the dropdown
    // takes the side with room.
    above?: boolean
  }>(),
  {
    label: undefined,
    addLabel: undefined,
    framed: false,
    size: 'md',
    disabled: false,
    searchable: false,
    searchPlaceholder: 'Search',
    above: false
  }
)

const emit = defineEmits<{
  select: [id: string]
  add: []
  // The row the pointer or keyboard focus is on, or null when it leaves the rows or the dropdown
  // closes, for a caller that shows what a row means while it is pointed at.
  point: [id: string | null]
}>()

const open = ref(false)
const rootRef = ref<HTMLElement | null>(null)
const triggerRef = ref<HTMLElement | null>(null)
const dropdownRef = ref<HTMLElement | null>(null)
const query = ref('')
const searchRef = ref<AppInputHandle | null>(null)
const dropdownStyle = ref<Record<string, string>>({})
const placed = ref(false)
// The unfiltered height, measured once per opening, decides the side; filtering only shrinks it.
let naturalHeight = 0

const GAP = 8

function place(): void {
  const trigger = triggerRef.value
  const dropdown = dropdownRef.value
  if (!trigger || !dropdown) return
  const rect = trigger.getBoundingClientRect()
  const p = placeFloat(rect, dropdown.offsetWidth, naturalHeight, GAP, props.above)
  dropdownStyle.value = {
    ...placementStyle(p),
    ...(props.framed ? { minWidth: `${rect.width}px` } : {})
  }
  placed.value = true
}

async function openDropdown(): Promise<void> {
  placed.value = false
  const rect = triggerRef.value?.getBoundingClientRect()
  dropdownStyle.value = props.framed && rect ? { minWidth: `${rect.width}px` } : {}
  open.value = true
  await nextTick()
  naturalHeight = dropdownRef.value?.offsetHeight ?? 0
  place()
}

function listen(on: boolean): void {
  const method = on ? 'addEventListener' : 'removeEventListener'
  window[method]('resize', place)
  document[method]('scroll', place, true)
}

watch(open, (isOpen) => {
  listen(isOpen)
  if (!isOpen) emit('point', null)
})

const pinnedItems = computed(() => props.items.filter((item) => item.pinned))

const shownItems = computed(() => {
  const q = query.value.trim().toLowerCase()
  const rest = props.items.filter((item) => !item.pinned)
  if (!q) return rest
  return rest.filter((item) => (item.search ?? item.label).toLowerCase().includes(q))
})

const activeItem = computed(() => props.items.find((i) => i.id === props.activeId))

const activeLabel = computed(() => props.label ?? activeItem.value?.label ?? props.placeholder)

function headingAt(index: number): string | undefined {
  const group = shownItems.value[index].group
  if (!group) return undefined
  return index === 0 || shownItems.value[index - 1].group !== group ? group : undefined
}

async function toggleOpen(): Promise<void> {
  if (props.disabled) return
  if (open.value) {
    open.value = false
    return
  }
  query.value = ''
  await openDropdown()
  if (props.searchable) searchRef.value?.focus()
}

function pickFirstShown(): void {
  const first = (
    query.value.trim() ? shownItems.value : [...pinnedItems.value, ...shownItems.value]
  ).find((item) => !item.disabled)
  if (first) pick(first.id)
}

function pick(id: string): void {
  emit('select', id)
  open.value = false
}

function pickAdd(): void {
  emit('add')
  open.value = false
}

function onDocumentClickOutside(event: MouseEvent): void {
  if (!open.value) return
  const target = event.target as Node
  if (rootRef.value?.contains(target) || dropdownRef.value?.contains(target)) return
  open.value = false
}

onMounted(() => document.addEventListener('click', onDocumentClickOutside, true))
onBeforeUnmount(() => {
  document.removeEventListener('click', onDocumentClickOutside, true)
  listen(false)
})
</script>

<template>
  <div ref="rootRef" class="switcher">
    <button
      ref="triggerRef"
      type="button"
      :class="
        framed
          ? ['sk-field-trigger', { 'sk-field-trigger--sm': size === 'sm' }]
          : ['sk-plain-trigger', { 'sk-plain-trigger--sm': size === 'sm' }]
      "
      :aria-expanded="open"
      :disabled="disabled"
      @click="toggleOpen"
    >
      <IconFrame v-if="activeItem?.icon && !label" class="item-icon" :size="13">
        <component
          :is="shape.tag"
          v-for="(shape, i) in activeItem.icon"
          :key="i"
          v-bind="shape.attrs"
        />
      </IconFrame>
      <span class="trigger-label">{{ activeLabel }}</span>
      <AppIcon class="sk-trigger-caret" name="chevronDown" :size="9" />
    </button>

    <Teleport to="body">
      <div
        v-if="open"
        ref="dropdownRef"
        class="dropdown sk-float"
        :class="{ placed, fit: framed, 'sk-menu--sm': size === 'sm' }"
        :style="dropdownStyle"
      >
        <AppInput
          v-if="searchable"
          ref="searchRef"
          v-model="query"
          class="search"
          :size="size"
          icon="search"
          spellcheck="false"
          :placeholder="searchPlaceholder"
          @keydown.enter.prevent="pickFirstShown"
          @keydown.escape="open = false"
        />
        <div v-if="pinnedItems.length" class="pinned-list">
          <button
            v-for="item in pinnedItems"
            :key="item.id"
            type="button"
            class="sk-menu-row pinned"
            :class="{ 'is-active': item.id === activeId }"
            :title="item.disabled ?? item.title"
            :disabled="!!item.disabled"
            @click="pick(item.id)"
            @mouseenter="emit('point', item.id)"
            @mouseleave="emit('point', null)"
            @focus="emit('point', item.id)"
            @blur="emit('point', null)"
          >
            <IconFrame v-if="item.icon" class="item-icon" :size="13">
              <component
                :is="shape.tag"
                v-for="(shape, si) in item.icon"
                :key="si"
                v-bind="shape.attrs"
              />
            </IconFrame>
            <span class="sk-menu-label">{{ item.label }}</span>
            <span v-if="item.disabled" class="sk-menu-hint">{{ item.disabled }}</span>
            <span v-else-if="item.hint" class="sk-menu-hint">{{ item.hint }}</span>
          </button>
        </div>
        <div class="item-list">
          <template v-for="(item, i) in shownItems" :key="item.id">
            <p v-if="headingAt(i)" class="sk-menu-heading">{{ headingAt(i) }}</p>
            <button
              type="button"
              class="sk-menu-row"
              :class="{ 'is-active': item.id === activeId }"
              :title="item.disabled ?? item.title"
              :disabled="!!item.disabled"
              @click="pick(item.id)"
              @mouseenter="emit('point', item.id)"
              @mouseleave="emit('point', null)"
              @focus="emit('point', item.id)"
              @blur="emit('point', null)"
            >
              <IconFrame v-if="item.icon" class="item-icon" :size="13">
                <component
                  :is="shape.tag"
                  v-for="(shape, si) in item.icon"
                  :key="si"
                  v-bind="shape.attrs"
                />
              </IconFrame>
              <span class="sk-menu-label">{{ item.label }}</span>
              <span v-if="item.disabled" class="sk-menu-hint">{{ item.disabled }}</span>
              <span v-else-if="item.hint" class="sk-menu-hint">{{ item.hint }}</span>
            </button>
          </template>
          <p v-if="items.length === pinnedItems.length" class="sk-menu-empty">{{ emptyText }}</p>
          <p v-else-if="shownItems.length === 0" class="sk-menu-empty">No match.</p>
        </div>
        <div v-if="$slots.footer" class="footer">
          <slot name="footer" />
        </div>
        <button v-if="addLabel" type="button" class="add-item" @click="pickAdd">
          <span class="icon">+</span>
          <span>{{ addLabel }}</span>
        </button>
      </div>
    </Teleport>
  </div>
</template>

<style scoped>
.switcher {
  position: relative;
}

.trigger-label {
  max-width: 220px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.sk-field-trigger .trigger-label {
  flex: 1;
  max-width: none;
}

/* Above the app modal, below the app's closing overlay. Hidden until placed, so it never flashes at
   the window corner. */
.dropdown {
  position: fixed;
  top: 0;
  left: 0;
  z-index: 950;
  width: 260px;
  display: flex;
  flex-direction: column;
  overflow: hidden;
  visibility: hidden;
}

.dropdown.placed {
  visibility: visible;
}

/* Under a field the dropdown is at least as wide as its trigger (set inline) and otherwise fits
   its items. */
.dropdown.fit {
  width: max-content;
  max-width: 320px;
}

.search {
  margin: var(--space-2) var(--space-2) 0;
}

.item-list {
  flex: 1 1 auto;
  min-height: 0;
  max-height: 320px;
  overflow-y: auto;
  padding: var(--space-2);
  display: flex;
  flex-direction: column;
  gap: 2px;
}

/* The pinned rows stand apart from the configured items: a divider below them and an accent icon. */
.pinned-list {
  display: flex;
  flex-direction: column;
  gap: 2px;
  padding: var(--space-2) var(--space-2) 0;
}

.pinned-list + .item-list {
  margin-top: var(--space-2);
  border-top: 1px solid var(--color-border-default);
}

.item-icon {
  flex-shrink: 0;
  color: var(--color-fg-secondary);
}

.sk-menu-row.pinned .item-icon,
.sk-field-trigger .item-icon,
.sk-plain-trigger .item-icon {
  color: var(--color-fg-accent);
}

/* The footer slot holds content that is not an item, such as a field that unlocks more items; a
   click inside it keeps the dropdown open. */
.footer {
  display: flex;
  flex-direction: column;
  gap: var(--space-2);
  padding: var(--space-2);
  border-top: 1px solid var(--color-border-default);
}

.add-item {
  display: flex;
  align-items: center;
  gap: var(--space-2);
  padding: var(--space-2) var(--space-3);
  background: transparent;
  border: none;
  border-top: 1px solid var(--color-border-default);
  color: var(--color-fg-secondary);
  font-family: var(--font-body);
  font-size: var(--text-sm);
  text-align: left;
  cursor: pointer;
  transition:
    background var(--duration-fast) var(--ease-out),
    color var(--duration-fast) var(--ease-out);
}

.add-item:hover {
  background: var(--color-amber);
  color: var(--color-on-fill);
}

.icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 16px;
  font-size: var(--text-sm);
  line-height: 1;
}
</style>
