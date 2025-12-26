<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'
import { useLiveStore } from '../stores/live'

const live = useLiveStore()
const now = ref(Date.now())

let timer: number

onMounted(() => {
  timer = window.setInterval(() => {
    now.value = Date.now()
  }, 100)
})

onUnmounted(() => {
  clearInterval(timer)
})

const formatTime = (ts?: string) => {
  if (!ts) return '-'
  return new Date(ts).toLocaleTimeString()
}

const getDurationDisplay = (entry: any) => {
  if (entry.status === 'ended' || entry.duration > 0) {
    return `${entry.duration.toFixed(1)}s`
  }
  if (live.isSessionActive(entry.id)) {
    const elapsed = (now.value - new Date(entry.created_at).getTime()) / 1000
    return `${elapsed.toFixed(1)}s`
  }
  return '-'
}
</script>

<template>
  <div class="bg-white dark:bg-slate-900 rounded-xl shadow-sm border border-slate-200 dark:border-slate-800 overflow-hidden">
    <div class="p-6 border-b border-slate-200 dark:border-slate-800 flex justify-between items-center">
      <h3 class="text-lg font-semibold">Activity Log</h3>
      <span class="text-xs font-medium px-2.5 py-0.5 rounded-full bg-blue-100 text-blue-800 dark:bg-blue-900/30 dark:text-blue-400">
        {{ live.lastHeard.length }} Entries
      </span>
    </div>
    
    <div class="overflow-x-auto">
      <table class="w-full text-left border-collapse">
        <thead class="bg-slate-50 dark:bg-slate-800/50">
          <tr>
            <th class="px-6 py-4 font-medium text-slate-500 text-sm">Time</th>
            <th class="px-6 py-4 font-medium text-slate-500 text-sm">Callsign</th>
            <th class="px-6 py-4 font-medium text-slate-500 text-sm">Target</th>
            <th class="px-6 py-4 font-medium text-slate-500 text-sm">Module</th>
            <th class="px-6 py-4 font-medium text-slate-500 text-sm">Route</th>
            <th class="px-6 py-4 font-medium text-slate-500 text-sm">Protocol</th>
            <th class="px-6 py-4 font-medium text-slate-500 text-sm">Duration</th>
          </tr>
        </thead>
        <tbody class="divide-y divide-slate-100 dark:divide-slate-800">
          <tr v-for="entry in live.lastHeard" :key="entry.id" 
              class="hover:bg-slate-50 dark:hover:bg-slate-800/30 transition-colors"
              :class="{'bg-red-50/50 dark:bg-red-900/10': live.isSessionActive(entry.id)}">
            <td class="px-6 py-4 text-sm text-slate-500" :class="{'text-red-600 dark:text-red-400 font-medium': live.isSessionActive(entry.id)}">
              {{ formatTime(entry.created_at) }}
            </td>
            <td class="px-6 py-4 font-bold transition-colors"
                :class="live.isSessionActive(entry.id) ? 'text-red-600 dark:text-red-400' : 'text-blue-600 dark:text-blue-400'">
              {{ entry.my }}
              <span v-if="live.isSessionActive(entry.id)" class="ml-2 inline-block w-2 h-2 bg-red-500 rounded-full animate-pulse"></span>
            </td>
            <td class="px-6 py-4 text-sm" :class="{'text-red-500 dark:text-red-300': live.isSessionActive(entry.id)}">
              {{ entry.ur }}
            </td>
            <td class="px-6 py-4">
              <span class="px-2 py-1 rounded text-xs font-mono uppercase transition-colors"
                    :class="live.isSessionActive(entry.id) ? 'bg-red-100 dark:bg-red-900/40 text-red-700 dark:text-red-300' : 'bg-slate-100 dark:bg-slate-800'">
                {{ entry.module }}
              </span>
            </td>
            <td class="px-6 py-4 text-sm text-slate-500" :class="{'text-red-400': live.isSessionActive(entry.id)}">
              {{ entry.rpt2 }}
            </td>
            <td class="px-6 py-4">
              <span class="px-2 py-1 rounded text-xs font-semibold uppercase"
                    :class="live.isSessionActive(entry.id) ? 'bg-red-500 text-white' : 'bg-blue-100 dark:bg-blue-900/30 text-blue-800 dark:text-blue-400'">
                {{ entry.protocol }}
              </span>
            </td>
            <td class="px-6 py-4 text-sm font-mono" :class="live.isSessionActive(entry.id) ? 'text-red-600' : 'text-slate-500'">
              {{ getDurationDisplay(entry) }}
            </td>
          </tr>
          <tr v-if="live.lastHeard.length === 0">
            <td colspan="6" class="px-6 py-12 text-center text-slate-400 italic">No transmissions heard yet...</td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>
