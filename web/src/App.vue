<script setup lang="ts">
import { onMounted } from 'vue'
import { RouterView, RouterLink } from 'vue-router'
import { Monitor, Users, Share2, LayoutGrid, Clock, Sun, Moon } from 'lucide-vue-next'
import { useThemeStore } from './stores/theme'
import { useLiveStore } from './stores/live'
import AppShell from './layouts/AppShell.vue'

const theme = useThemeStore()
const live = useLiveStore()

onMounted(() => {
  live.connect()
  theme.fetchConfig()
})

const handleNavClick = () => {
  if (window.innerWidth < 1024) {
    theme.sidebarOpen = false
  }
}
</script>

<template>
  <AppShell>
    <!-- Header Actions -->
    <template #header-actions>
      <button @click="theme.toggleMode()" 
              class="p-2 rounded-lg hover:bg-slate-100 dark:hover:bg-slate-800 text-slate-600 dark:text-slate-400 transition-colors"
              title="Toggle Theme">
        <Sun :size="20" v-if="theme.mode === 'light'" />
        <Moon :size="20" v-else-if="theme.mode === 'dark'" />
        <Monitor :size="20" v-else />
      </button>
    </template>

    <!-- Sidebar Navigation -->
    <template #sidebar>
      <nav class="space-y-1">
        <RouterLink to="/" @click="handleNavClick" class="flex items-center gap-3 px-3 py-2 rounded-lg hover:bg-slate-100 dark:hover:bg-slate-800 transition-colors" active-class="bg-blue-50 dark:bg-blue-900/30 text-blue-600 dark:text-blue-400 font-medium">
          <Clock :size="20" />
          <span>Last Heard</span>
        </RouterLink>
        <RouterLink to="/nodes" @click="handleNavClick" class="flex items-center gap-3 px-3 py-2 rounded-lg hover:bg-slate-100 dark:hover:bg-slate-800 transition-colors" active-class="bg-blue-50 dark:bg-blue-900/30 text-blue-600 dark:text-blue-400 font-medium">
          <Monitor :size="20" />
          <span>Nodes</span>
        </RouterLink>
        <RouterLink to="/users" @click="handleNavClick" class="flex items-center gap-3 px-3 py-2 rounded-lg hover:bg-slate-100 dark:hover:bg-slate-800 transition-colors" active-class="bg-blue-50 dark:bg-blue-900/30 text-blue-600 dark:text-blue-400 font-medium">
          <Users :size="20" />
          <span>Users</span>
        </RouterLink>
        <RouterLink to="/peers" @click="handleNavClick" class="flex items-center gap-3 px-3 py-2 rounded-lg hover:bg-slate-100 dark:hover:bg-slate-800 transition-colors" active-class="bg-blue-50 dark:bg-blue-900/30 text-blue-600 dark:text-blue-400 font-medium">
          <Share2 :size="20" />
          <span>Peers</span>
        </RouterLink>
        <RouterLink to="/modules" @click="handleNavClick" class="flex items-center gap-3 px-3 py-2 rounded-lg hover:bg-slate-100 dark:hover:bg-slate-800 transition-colors" active-class="bg-blue-50 dark:bg-blue-900/30 text-blue-600 dark:text-blue-400 font-medium">
          <LayoutGrid :size="20" />
          <span>Modules</span>
        </RouterLink>
      </nav>
    </template>

    <!-- Main Content -->
    <RouterView />
  </AppShell>
</template>

<style>
/* Basic transition for theme switching */
.transition-colors {
  transition-property: color, background-color, border-color, text-decoration-color, fill, stroke;
  transition-timing-function: cubic-bezier(0.4, 0, 0.2, 1);
  transition-duration: 200ms;
}
</style>
