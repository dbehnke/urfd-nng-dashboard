<script setup lang="ts">
import { computed } from 'vue'
import { useReflectorStore } from '../stores/reflector'
import { formatTimeSince } from '../utils/time'

const reflector = useReflectorStore()

const nodesByModule = computed(() => {
  const groups: Record<string, any[]> = {}
  reflector.clients.forEach(client => {
    const mod = client.OnModule || '?'
    if (!groups[mod]) groups[mod] = []
    groups[mod].push(client)
  })
  // Sort modules alphabetically
  return Object.keys(groups).sort().map(mod => ({
    name: mod,
    clients: groups[mod]
  }))
})
</script>

<template>
  <div class="grid grid-cols-1 lg:grid-cols-2 gap-8">
    <div v-for="group in nodesByModule" :key="group.name" class="bg-white dark:bg-slate-900 rounded-xl shadow-sm border border-slate-200 dark:border-slate-800 flex flex-col">
      <div class="p-6 border-b border-slate-200 dark:border-slate-800 flex justify-between items-center bg-slate-50/50 dark:bg-slate-800/30">
        <div class="w-10 h-10 rounded-lg bg-blue-600 flex items-center justify-center text-white font-bold text-xl shadow-lg shadow-blue-500/20">
          {{ group.name }}
        </div>
        <span class="text-xs font-medium px-2.5 py-0.5 rounded-full bg-blue-100 text-blue-800 dark:bg-blue-900/30 dark:text-blue-400">
          {{ group.clients?.length || 0 }} Nodes
        </span>
      </div>
      
      <div class="overflow-x-auto">
        <table class="w-full text-left border-collapse">
          <thead>
            <tr class="text-xs font-semibold text-slate-500 uppercase tracking-wider">
              <th class="px-6 py-4">Callsign</th>
              <th class="px-6 py-4">Protocol</th>
              <th class="px-6 py-4 text-right">Connected</th>
            </tr>
          </thead>
          <tbody class="divide-y divide-slate-100 dark:divide-slate-800">
            <tr v-for="node in group.clients" :key="node.Callsign" class="hover:bg-slate-50/50 dark:hover:bg-slate-800/50 transition-colors group">
              <td class="px-6 py-4">
                <div class="font-bold text-blue-600 dark:text-blue-400 group-hover:scale-105 transition-transform origin-left inline-block">
                  {{ node.Callsign }}
                </div>
              </td>
              <td class="px-6 py-4">
                <span class="px-2 py-0.5 rounded bg-slate-100 dark:bg-slate-800 text-[10px] font-mono font-bold uppercase tracking-tighter text-slate-500">
                  {{ node.Protocol }}
                </span>
              </td>
              <td class="px-6 py-4 text-right text-sm text-slate-500">
                {{ formatTimeSince(node.ConnectTime) }}
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>
    
    <div v-if="reflector.clients.length === 0" class="col-span-full py-20 text-center text-slate-400 italic bg-white dark:bg-slate-900 rounded-2xl border-2 border-dashed border-slate-200 dark:border-slate-800">
      <div class="mb-4 opacity-50">
        <svg class="w-12 h-12 mx-auto" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M19 11H5m14 0a2 2 0 012 2v6a2 2 0 01-2 2H5a2 2 0 01-2-2v-6a2 2 0 012-2m14 0V9a2 2 0 00-2-2M5 11V9a2 2 0 002-2m0 0V5a2 2 0 012-2h6a2 2 0 012 2v2M7 7h10" />
        </svg>
      </div>
      No nodes connected to reflector.
    </div>
  </div>
</template>
