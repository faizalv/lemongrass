// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

import DefaultTheme from 'vitepress/theme'
import { defineComponent, h, watchEffect } from 'vue'
import { inBrowser, useData } from 'vitepress'
import '@fontsource-variable/comfortaa/wght.css'
import '@fontsource-variable/dm-sans/opsz.css'
import '@fontsource-variable/dm-sans/opsz-italic.css'
import '@fontsource-variable/jetbrains-mono/wght.css'
import '@fontsource-variable/jetbrains-mono/wght-italic.css'
import './custom.css'

// Keeps Sankasten's data-theme in step with VitePress's toggle after load; the head script in config.ts covers
// the first paint.
const Layout = defineComponent({
  setup() {
    const { isDark } = useData()
    if (inBrowser) {
      watchEffect(() => {
        document.documentElement.dataset.theme = isDark.value ? 'dark' : 'light'
      })
    }
    return () => h(DefaultTheme.Layout)
  }
})

export default { extends: DefaultTheme, Layout }
