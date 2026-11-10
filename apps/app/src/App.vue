<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import Icon from '@/components/Icon.vue'

const route = useRoute()
const auth = useAuthStore()

const showChrome = computed(() => route.name !== 'sign-in')
const title = computed(() => (route.meta.title as string | undefined) ?? 'WCS Workshops')

const tabs = computed(() => {
  const list = [
    { name: 'library', icon: 'library', label: 'Library', match: ['library', 'collection', 'video'] },
    { name: 'scan', icon: 'qr', label: 'Scan QR code', match: ['scan'] },
  ]
  if (auth.isCreator) list.push({ name: 'upload', icon: 'upload', label: 'Upload', match: ['upload'] })
  list.push({ name: 'settings', icon: 'settings', label: 'Settings', match: ['settings', 'audit'] })
  return list
})

function isActive(match: string[]) {
  return typeof route.name === 'string' && match.includes(route.name)
}
</script>

<template>
  <header v-if="showChrome" class="topbar">
    <h1>{{ title }}</h1>
    <nav aria-label="Main">
      <RouterLink
        v-for="t in tabs"
        :key="t.name"
        :to="{ name: t.name }"
        class="iconbtn"
        :class="{ active: isActive(t.match) }"
        :aria-label="t.label"
        :title="t.label"
      >
        <Icon :name="t.icon" />
      </RouterLink>
    </nav>
  </header>
  <main class="page">
    <RouterView />
  </main>
</template>
