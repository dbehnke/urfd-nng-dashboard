<script setup lang="ts">
import { computed } from 'vue'
import { useReflectorStore } from '../stores/reflector'
import { useThemeStore } from '../stores/theme'

const reflector = useReflectorStore()
const theme = useThemeStore()

const displayModules = computed(() => {
  // 1. Start with configured modules
  const configModules = theme.config.reflector.modules || {}
  const allModules = new Map<string, { Name: string, Description: string }>()

  // Add from config
  for (const [name, desc] of Object.entries(configModules)) {
    allModules.set(name, { Name: name, Description: desc })
  }

  // 2. Add/Refresh from dynamic NNG state
  for (const m of reflector.modules) {
    const existing = allModules.get(m.Name)
    // Dynamic Description wins only if config entry is missing (or we prefer config override)
    // Actually, config override should win.
    const desc = configModules[m.Name] || m.Description
    allModules.set(m.Name, { Name: m.Name, Description: desc })
  }

  // Convert to array and sort
  return Array.from(allModules.values()).sort((a, b) => a.Name.localeCompare(b.Name))
})

const getUserCount = (moduleName: string) => {
  return reflector.users.filter(u => u.OnModule === moduleName).length
}
</script>

<template>
  <div class="bg-white dark:bg-slate-900 rounded-2xl shadow-sm border border-slate-200 dark:border-slate-800 overflow-hidden">
    <div class="p-6 border-b border-slate-100 dark:border-slate-800 bg-slate-50/30 dark:bg-slate-800/20">
      <h3 class="text-lg font-bold flex items-center gap-2">
        <span class="w-2 h-6 bg-blue-600 rounded-full"></span>
        Available Modules
      </h3>
    </div>
    
    <div class="overflow-x-auto">
      <table class="w-full text-left border-collapse">
        <thead>
          <tr class="text-xs font-semibold text-slate-500 uppercase tracking-wider border-b border-slate-100 dark:border-slate-800">
            <th class="px-8 py-4">Module</th>
            <th class="px-8 py-4">Description</th>
            <th class="px-8 py-4 text-center">Active Users</th>
          </tr>
        </thead>
        <tbody class="divide-y divide-slate-100 dark:divide-slate-800">
          <tr v-for="m in displayModules" :key="m.Name" class="hover:bg-slate-50/50 dark:hover:bg-slate-800/50 transition-colors group">
            <td class="px-8 py-5">
              <div class="w-10 h-10 rounded-lg bg-blue-600 flex items-center justify-center text-white font-bold text-xl shadow-lg shadow-blue-500/20">
                {{ m.Name }}
              </div>
            </td>
            <td class="px-8 py-5 text-slate-500 dark:text-slate-400">
              {{ m.Description || 'No description available' }}
            </td>
            <td class="px-8 py-5 text-center">
              <div class="inline-flex items-center gap-2 px-3 py-1 rounded-full text-sm font-bold transition-all duration-300"
                   :class="getUserCount(m.Name) > 0 
                            ? 'bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-400 scale-110 shadow-sm' 
                            : 'bg-slate-100 text-slate-400 dark:bg-slate-800 dark:text-slate-600'">
                <div v-if="getUserCount(m.Name) > 0" class="w-2 h-2 rounded-full bg-green-500 animate-pulse"></div>
                {{ getUserCount(m.Name) }}
              </div>
            </td>
          </tr>
          
          <tr v-if="displayModules.length === 0">
            <td colspan="3" class="px-8 py-20 text-center text-slate-400 italic bg-white dark:bg-slate-900">
              <div class="mb-4 opacity-50">
                <svg class="w-12 h-12 mx-auto" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M19 11H5m14 0a2 2 0 012 2v6a2 2 0 01-2 2H5a2 2 0 01-2-2v-6a2 2 0 012-2m14 0V9a2 2 0 00-2-2M5 11V9a2 2 0 002-2m0 0V5a2 2 0 012-2h6a2 2 0 012 2v2M7 7h10" />
                </svg>
              </div>
              Loading module information...
            </td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>
