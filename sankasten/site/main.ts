// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2026 The Sankasten Authors

// The site bundles the three typefaces the type tokens name, the way an app using the package does.
import '@fontsource-variable/comfortaa/wght.css'
import '@fontsource-variable/dm-sans/opsz.css'
import '@fontsource-variable/dm-sans/opsz-italic.css'
import '@fontsource-variable/jetbrains-mono/wght.css'
import '../src/styles/index.css'
import './site.css'

import { createApp } from 'vue'
import App from './App.vue'

createApp(App).mount('#app')
