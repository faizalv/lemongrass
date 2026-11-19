<!-- SPDX-License-Identifier: MPL-2.0 -->
<!-- Copyright (C) 2026 The Lemongrass Authors -->

<script setup lang="ts">
import { VPButton } from 'vitepress/theme'
import HomeHero from './HomeHero.vue'
import InstallCards from './InstallCards.vue'
import FeatureTiles, { type Tile } from './FeatureTiles.vue'
import screenshot from '../../../images/overall-view-4-web.webp'

// Rows of the six column grid add up to six, so wide screens never leave a gap.
const features: Tile[] = [
  {
    title: 'Bring your own account.',
    body: 'Sign in with the subscription you already have. Lemongrass runs Claude Code, Codex and Antigravity in its own terminal panes and keeps track of which account each tab uses.',
    link: '/features/accounts',
    cols: 2
  },
  {
    title: 'Quality per cost, 1 to 1.',
    body: 'Every agent works from the same idea of how things should be done, whether it is Claude, Codex or Antigravity. The shared system prompt is built to stay cached across many agents, and it makes each agent own the work it did. One run, one finished result. That is the whole point.',
    cols: 4,
    glow: 'amber'
  },
  {
    title: 'Agents that know your project.',
    body: 'Every agent follows operating procedures that guide it step by step. <code>biblio</code>, a file based knowledge system, lets any session pick up any task cold with the full context, without spending tokens reading and reinterpreting your code.',
    link: '/features/orientation',
    cols: 4,
    glow: 'violet'
  },
  {
    title: 'Limit access, not capability.',
    body: 'Agents in Lemongrass get tools to work directly with your databases, REST APIs, mobile apps and web apps. You decide what is allowed, <code>lgrass</code> guards it, and the agent uses it responsibly.',
    link: '/features/connectors',
    cols: 2
  },
  {
    title: 'Safe, without being annoying.',
    body: 'Agents run with permission prompts off by default. Can you trust them? No. That is why Lemongrass checks every command they run, from plain shell commands to PowerShell read by a real parser, down to the code inside scripts they hand to an interpreter. Even every SQL statement is checked against what you allowed.',
    link: '/features/guard',
    cols: 3,
    glow: 'rose'
  },
  {
    title: 'Workgroup, anyone?',
    body: 'Put Claude, Codex and Antigravity agents in one team. Plain worktrees leave each agent working in a black box until you face conflict hell. In a workgroup the agents post their plans, agree on who does what, and talk to each other directly before anything changes.',
    link: '/features/workgroups',
    cols: 3
  },
  {
    title: 'You are the manager.',
    body: 'When they need your decision, they ask instead of stalling. Can’t delete a file? Ask. About to run something that looks dangerous? Ask.',
    link: '/features/ask',
    cols: 2
  },
  {
    title: 'A vault!',
    body: 'Your database credentials and real accounts stay in a vault. When you tell an agent to query a database or call an API as a certain user, it holds zero credentials. Lemongrass hands it a random 6 character code, and it picks which user to act as.',
    link: '/features/connectors',
    cols: 4,
    glow: 'indigo'
  },
  {
    title: 'The classic IDE is still here.',
    items: [
      'A proper Git GUI, designed with agents in mind.',
      'SSH with a file explorer and a key manager.',
      'Rester, an HTTP client for testing APIs without the fuss.'
    ],
    cols: 6
  }
]

const planned: Tile[] = [
  {
    title: 'Lemongrass workspace',
    body: 'A virtual file system that lets agents work in isolation while staying aware of everything around them.',
    cols: 3,
    rows: 2,
    glow: 'violet'
  },
  {
    title: 'Sibling projects',
    body: 'Agents switch between project folders seamlessly, without losing their prompt cache.',
    cols: 3,
    glow: 'rose'
  },
  {
    title: 'Lightweight language server plugins',
    body: 'Agents drive live, deterministic debugging together with you.',
    cols: 3,
    glow: 'amber'
  }
]
</script>

<template>
  <div class="home">
    <HomeHero />

    <img
      class="shot"
      :src="screenshot"
      alt="The Lemongrass app with an SSH terminal and its file browser, biblio editors, the Git panel, an agent tab and the Rester client"
    />

    <InstallCards />

    <hr class="divider" />

    <section class="section">
      <FeatureTiles :tiles="features" />
    </section>

    <section class="closer">
      <p>More features are coming, free. Get it now and send us feedback and weird ideas!</p>
      <VPButton size="big" theme="brand" text="Download" href="#install" />
    </section>

    <section class="section">
      <h2 class="heading">Coming soon <a class="heading-link" href="/roadmap">See the roadmap</a></h2>
      <FeatureTiles :tiles="planned" />
    </section>

    <footer class="license">Lemongrass is licensed under the Mozilla Public License 2.0.</footer>
  </div>
</template>

<style scoped>
.home {
  max-width: 1152px;
  margin: 0 auto;
  padding: 0 var(--space-6) var(--space-12);
}

.shot {
  display: block;
  max-width: 100%;
  margin: 0 auto;
  border: 1px solid var(--color-border-default);
  border-radius: var(--radius-xl);
  box-shadow: var(--shadow-lg);
}

.divider {
  margin: var(--space-8) 0 var(--space-16);
  border: 0;
  border-top: 1px solid var(--color-border-subtle);
}

.section {
  padding-bottom: var(--space-16);
}

.heading {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: var(--space-4);
  margin: 0 0 var(--space-6);
  font-size: var(--text-2xl);
  font-weight: var(--weight-bold);
  color: var(--color-fg-primary);
}

.heading-link {
  font-size: var(--text-sm);
  font-weight: var(--weight-medium);
  color: var(--color-amber);
}

.closer {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: var(--space-6);
  padding: var(--space-8) 0 var(--space-20);
  text-align: center;
}

.closer p {
  margin: 0;
  max-width: 32rem;
  font-size: var(--text-xl);
  line-height: var(--leading-snug);
  font-weight: var(--weight-semibold);
  color: var(--color-fg-primary);
}

.license {
  padding-top: var(--space-8);
  border-top: 1px solid var(--color-border-subtle);
  font-size: var(--text-sm);
  text-align: center;
  color: var(--color-fg-muted);
}
</style>
