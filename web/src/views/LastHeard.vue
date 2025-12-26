<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { useLiveStore } from '../stores/live'
import { useReflectorStore } from '../stores/reflector'

const live = useLiveStore()
const reflector = useReflectorStore()
const now = ref(Date.now())

const filterText = ref('')
const moduleFilter = ref('')

let timer: number

onMounted(() => {
  timer = window.setInterval(() => {
    now.value = Date.now()
  }, 100)
})

onUnmounted(() => {
  clearInterval(timer)
})

const parseDate = (ts?: string) => {
  if (!ts) return 0
  try {
    // Standardize Go format: "2023-12-26 18:27:49.123 +0000 UTC" -> "2023-12-26T18:27:49.123Z"
    let normalized = ts.trim()
    
    // If it's a Go string with multiple spaces, take first part (date) and second (time)
    if (normalized.includes(' ')) {
      const parts = normalized.split(/\s+/)
      if (parts.length >= 2) {
        normalized = parts[0] + 'T' + parts[1]
      }
    }

    const tParts = normalized.split('T')
    if (!normalized.includes('Z') && !normalized.includes('+') && tParts.length > 1) {
      if (normalized.includes('-') && normalized.indexOf('-', 10) !== -1) {
         // Has offset like -05:00 after the time, do nothing
      } else {
         normalized += 'Z'
      }
    }
    const d = new Date(normalized)
    return isNaN(d.getTime()) ? 0 : d.getTime()
  } catch (e) {
    return 0
  }
}

const formatTime = (ts?: string) => {
  const time = parseDate(ts)
  if (time === 0) return '-'
  return new Date(time).toLocaleTimeString()
}

const getDurationDisplay = (entry: any) => {
  if (entry.status === 'ended' || entry.duration > 0) {
    return `${entry.duration.toFixed(1)}s`
  }
  if (live.isSessionActive(entry.id)) {
    const time = parseDate(entry.created_at)
    const elapsed = (now.value - time) / 1000
    return `${elapsed.toFixed(1)}s`
  }
  return '-'
}

const filteredEntries = computed(() => {
  let entries = [...live.lastHeard]

  // Filter
  if (filterText.value) {
    const q = filterText.value.toLowerCase()
    entries = entries.filter(e => 
      (e.id && e.my) && // Defensive check
      (e.my?.toLowerCase().includes(q) || 
      e.ur?.toLowerCase().includes(q))
    )
  } else {
    // Always filter out invalid entries
    entries = entries.filter(e => e.id && e.my)
  }

  if (moduleFilter.value) {
    entries = entries.filter(e => e.module === moduleFilter.value)
  }

  // Sort: Active first, then by time DESC, then ID DESC for stability
  return entries.sort((a, b) => {
    const aActive = live.isSessionActive(a.id)
    const bActive = live.isSessionActive(b.id)

    if (aActive && !bActive) return -1
    if (!aActive && bActive) return 1

    const aTime = parseDate(a.created_at)
    const bTime = parseDate(b.created_at)

    if (aTime !== bTime) {
      return bTime - aTime
    }

    // Stabilize with ID if times are equal
    return b.id - a.id
  })
})

const clearFilters = () => {
  filterText.value = ''
  moduleFilter.value = ''
}
</script>

<template>
  <div class="space-y-6">
    <!-- Filters Card -->
    <div class="bg-white dark:bg-slate-900 p-4 rounded-xl shadow-sm border border-slate-200 dark:border-slate-800 flex flex-wrap items-center gap-4">
      <div class="relative flex-1 min-w-[200px]">
        <div class="absolute inset-y-0 left-0 pl-3 flex items-center pointer-events-none text-slate-400">
          <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
          </svg>
        </div>
        <input v-model="filterText" type="text" placeholder="Search callsign or target..." 
               class="w-full pl-10 pr-4 py-2 bg-slate-50 dark:bg-slate-800 border border-slate-200 dark:border-slate-700 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-blue-500 transition-all">
      </div>

      <div class="flex items-center gap-2">
        <span class="text-xs font-bold text-slate-400 uppercase tracking-wider">Module</span>
        <select v-model="moduleFilter" id="moduleFilter" class="bg-slate-50 dark:bg-slate-800 border border-slate-200 dark:border-slate-700 rounded-lg text-sm py-2 px-3 focus:outline-none focus:ring-2 focus:ring-blue-500 transition-all">
          <option value="">All</option>
          <option v-for="m in reflector.modules" :key="m.Name" :value="m.Name">{{ m.Name }}</option>
          <!-- Fallback if modules not loaded -->
          <option v-if="reflector.modules.length === 0" v-for="m in 'ABC'.split('')" :key="m" :value="m">{{ m }}</option>
        </select>
      </div>

      <button @click="clearFilters" 
              class="px-4 py-2 text-sm font-medium text-slate-600 dark:text-slate-400 hover:bg-slate-100 dark:hover:bg-slate-800 rounded-lg transition-colors flex items-center gap-2">
        <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
        </svg>
        Clear
      </button>
    </div>

    <!-- Mobile Card View (Visible < 640px) -->
    <div class="block sm:hidden space-y-4">
      <div v-for="entry in filteredEntries.slice(0, 26)" :key="entry.id"
           class="bg-white dark:bg-slate-900 rounded-xl p-4 shadow-sm border border-slate-200 dark:border-slate-800 relative overflow-hidden"
           :class="{'border-red-200 dark:border-red-900/50 bg-red-50/50 dark:bg-red-900/10': live.isSessionActive(entry.id)}">
        
        <!-- Active Indicator Strip -->
        <div v-if="live.isSessionActive(entry.id)" 
             class="absolute left-0 top-0 bottom-0 w-1 bg-red-500 animate-pulse"></div>

        <div class="flex justify-between items-start mb-3 pl-2">
          <div>
            <div class="text-xs text-slate-500 font-mono mb-0.5">{{ formatTime(entry.created_at) }}</div>
            <div class="font-bold text-lg leading-none flex items-center gap-2"
                 :class="live.isSessionActive(entry.id) ? 'text-red-600 dark:text-red-400' : 'text-blue-600 dark:text-blue-400'">
              {{ entry.my }}
              <span v-if="live.isSessionActive(entry.id)" class="inline-block w-2 h-2 bg-red-500 rounded-full animate-pulse"></span>
            </div>
          </div>
          <span class="px-2 py-1 rounded text-[10px] font-bold uppercase ml-2"
                :class="live.isSessionActive(entry.id) ? 'bg-red-500 text-white' : 'bg-blue-100 dark:bg-blue-900/30 text-blue-800 dark:text-blue-400'">
            {{ entry.protocol }}
          </span>
        </div>

        <div class="grid grid-cols-2 gap-2 pl-2 text-sm">
          <div>
            <span class="text-xs text-slate-400 uppercase tracking-wider block">Target</span>
            <span class="font-medium text-slate-700 dark:text-slate-200">{{ entry.ur }}</span>
          </div>
          <div class="text-right">
             <span class="text-xs text-slate-400 uppercase tracking-wider block">Duration</span>
             <span class="font-mono font-medium" :class="live.isSessionActive(entry.id) ? 'text-red-600 font-bold' : 'text-slate-500'">
               {{ getDurationDisplay(entry) }}
             </span>
          </div>
        </div>

        <div class="mt-3 pt-3 border-t border-slate-100 dark:border-slate-800 flex items-center justify-between pl-2">
           <div class="flex items-center gap-2">
             <span class="text-xs text-slate-400">Via</span>
             <span class="text-sm font-medium text-slate-600 dark:text-slate-300">{{ entry.rpt2 }}</span>
           </div>
           
           <span class="px-2 py-0.5 rounded text-[10px] font-bold uppercase bg-slate-100 dark:bg-slate-800 text-slate-500"
                 :class="{'!bg-red-100 !text-red-700 dark:!bg-red-900/30 dark:!text-red-300': live.isSessionActive(entry.id)}">
             Module {{ entry.module }}
           </span>
        </div>
      </div>

      <!-- Empty State Mobile -->
      <div v-if="filteredEntries.length === 0" class="text-center py-12 text-slate-400 italic">
        No active transmissions
      </div>
    </div>

    <!-- Table Card (Hidden < 640px) -->
    <div class="hidden sm:block bg-white dark:bg-slate-900 rounded-xl shadow-sm border border-slate-200 dark:border-slate-800 overflow-hidden text-slate-700 dark:text-slate-200">
      <div class="p-6 border-b border-slate-200 dark:border-slate-800 flex justify-between items-center">
        <h3 class="text-lg font-semibold">Activity Log</h3>
        <span class="text-xs font-medium px-2.5 py-0.5 rounded-full bg-blue-100 text-blue-800 dark:bg-blue-900/30 dark:text-blue-400">
          {{ filteredEntries.length }} Entries
        </span>
      </div>
      
      <div class="overflow-x-auto">
        <table class="w-full text-left border-collapse">
          <thead class="bg-slate-50 dark:bg-slate-800/50">
            <tr>
              <th class="px-6 py-4 font-medium text-slate-500 text-sm">Time</th>
              <th class="px-6 py-4 font-medium text-slate-500 text-sm">Callsign</th>
              <th class="px-6 py-4 font-medium text-slate-500 text-sm">Target</th>
              <th class="px-6 py-4 font-medium text-slate-500 text-sm text-center">Module</th>
              <th class="px-6 py-4 font-medium text-slate-500 text-sm">Route</th>
              <th class="px-6 py-4 font-medium text-slate-500 text-sm">Protocol</th>
              <th class="px-6 py-4 font-medium text-slate-500 text-sm text-right">Duration</th>
            </tr>
          </thead>
          <tbody class="divide-y divide-slate-100 dark:divide-slate-800">
            <tr v-for="entry in filteredEntries" :key="entry.id" 
                class="hover:bg-slate-50 dark:hover:bg-slate-800/30 transition-colors"
                :class="{'bg-red-50/30 dark:bg-red-900/10': live.isSessionActive(entry.id)}">
              <td class="px-6 py-4 text-sm text-slate-500" :class="{'text-red-600 dark:text-red-400 font-medium': live.isSessionActive(entry.id)}">
                {{ formatTime(entry.created_at) }}
              </td>
              <td class="px-6 py-4 font-bold transition-colors"
                  :class="live.isSessionActive(entry.id) ? 'text-red-600 dark:text-red-400' : 'text-blue-600 dark:text-blue-400'">
                {{ entry.my }}
                <span v-if="live.isSessionActive(entry.id)" class="ml-2 inline-block w-2 h-2 bg-red-500 rounded-full animate-pulse"></span>
              </td>
              <td class="px-6 py-4 text-sm font-medium" :class="live.isSessionActive(entry.id) ? 'text-red-500 dark:text-red-300' : 'text-slate-700 dark:text-slate-200'">
                {{ entry.ur }}
              </td>
              <td class="px-6 py-4 text-center">
                <span class="px-2.5 py-1 rounded-md text-xs font-mono font-bold uppercase transition-colors inline-block min-w-[24px]"
                      :class="live.isSessionActive(entry.id) ? 'bg-red-100 dark:bg-red-900/40 text-red-700 dark:text-red-300' : 'bg-slate-100 dark:bg-slate-800 text-slate-600 dark:text-slate-400'">
                  {{ entry.module }}
                </span>
              </td>
              <td class="px-6 py-4 text-sm text-slate-500" :class="{'text-red-400': live.isSessionActive(entry.id)}">
                {{ entry.rpt2 }}
              </td>
              <td class="px-6 py-4">
                <span class="px-2 py-0.5 rounded text-[10px] font-bold uppercase"
                      :class="live.isSessionActive(entry.id) ? 'bg-red-500 text-white' : 'bg-blue-100 dark:bg-blue-900/30 text-blue-800 dark:text-blue-400'">
                  {{ entry.protocol }}
                </span>
              </td>
              <td class="px-6 py-4 text-sm font-mono text-right" :class="live.isSessionActive(entry.id) ? 'text-red-600 font-bold' : 'text-slate-500'">
                {{ getDurationDisplay(entry) }}
              </td>
            </tr>
            <tr v-if="filteredEntries.length === 0">
              <td colspan="7" class="px-6 py-20 text-center text-slate-400 italic">
                <div class="mb-2 opacity-20">
                  <svg class="w-12 h-12 mx-auto" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
                  </svg>
                </div>
                No matching transmissions found...
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>
  </div>
</template>
