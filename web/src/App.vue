<script setup lang="ts">
import { onMounted } from 'vue'
import { RouterView, RouterLink } from 'vue-router'
import { Monitor, Users, Share2, LayoutGrid, Clock, Sun, Moon, Wifi, WifiOff } from 'lucide-vue-next'
import { useThemeStore } from './stores/theme.ts'
import { useLiveStore } from './stores/live.ts'

const theme = useThemeStore()
const live = useLiveStore()

onMounted(() => {
  live.connect()
})
</script>

<template>
  <div class="flex h-screen bg-slate-50 dark:bg-slate-950 text-slate-900 dark:text-slate-100 transition-colors duration-200">
    <!-- Sidebar -->
    <aside class="w-64 bg-white dark:bg-slate-900 border-r border-slate-200 dark:border-slate-800 flex flex-col">
      <div class="p-6 flex items-center justify-between">
        <h1 class="text-xl font-bold tracking-tight text-blue-600 dark:text-blue-400">URFD</h1>
        <div :title="live.connected ? 'Connected' : 'Disconnected'">
          <Wifi v-if="live.connected" :size="16" class="text-green-500" />
          <WifiOff v-else :size="16" class="text-red-500" />
        </div>
      </div>
      
      <nav class="flex-1 px-4 space-y-1">
        <RouterLink to="/" class="flex items-center gap-3 px-3 py-2 rounded-lg hover:bg-slate-100 dark:hover:bg-slate-800 transition-colors" active-class="bg-blue-50 dark:bg-blue-900/30 text-blue-600 dark:text-blue-400">
          <Clock :size="20" />
          <span>Last Heard</span>
        </RouterLink>
        <RouterLink to="/nodes" class="flex items-center gap-3 px-3 py-2 rounded-lg hover:bg-slate-100 dark:hover:bg-slate-800 transition-colors" active-class="bg-blue-50 dark:bg-blue-900/30 text-blue-600 dark:text-blue-400">
          <Monitor :size="20" />
          <span>Nodes</span>
        </RouterLink>
        <RouterLink to="/users" class="flex items-center gap-3 px-3 py-2 rounded-lg hover:bg-slate-100 dark:hover:bg-slate-800 transition-colors" active-class="bg-blue-50 dark:bg-blue-900/30 text-blue-600 dark:text-blue-400">
          <Users :size="20" />
          <span>Users</span>
        </RouterLink>
        <RouterLink to="/peers" class="flex items-center gap-3 px-3 py-2 rounded-lg hover:bg-slate-100 dark:hover:bg-slate-800 transition-colors" active-class="bg-blue-50 dark:bg-blue-900/30 text-blue-600 dark:text-blue-400">
          <Share2 :size="20" />
          <span>Peers</span>
        </RouterLink>
        <RouterLink to="/modules" class="flex items-center gap-3 px-3 py-2 rounded-lg hover:bg-slate-100 dark:hover:bg-slate-800 transition-colors" active-class="bg-blue-50 dark:bg-blue-900/30 text-blue-600 dark:text-blue-400">
          <LayoutGrid :size="20" />
          <span>Modules</span>
        </RouterLink>
      </nav>

      <div class="p-4 border-t border-slate-200 dark:border-slate-800">
        <button @click="theme.toggleMode()" class="flex items-center gap-3 w-full px-3 py-2 rounded-lg hover:bg-slate-100 dark:hover:bg-slate-800 transition-colors">
          <Sun :size="20" v-if="theme.mode === 'light'" />
          <Moon :size="20" v-else-if="theme.mode === 'dark'" />
          <Monitor :size="20" v-else />
          <span class="capitalize">{{ theme.mode }} Mode</span>
        </button>
      </div>
    </aside>

    <!-- Main Content -->
    <main class="flex-1 overflow-auto p-8">
      <header class="mb-8 flex justify-between items-center">
        <div>
          <h2 class="text-2xl font-bold">Reflector Status</h2>
          <p class="text-slate-500 dark:text-slate-400">Real-time gateway monitoring</p>
        </div>
      </header>

      <div class="max-w-7xl mx-auto">
        <RouterView />
      </div>
    </main>
  </div>
</template>

<style>
/* Basic transition for theme switching */
.transition-colors {
  transition-property: color, background-color, border-color, text-decoration-color, fill, stroke;
  transition-timing-function: cubic-bezier(0.4, 0, 0.2, 1);
  transition-duration: 200ms;
}
</style>
