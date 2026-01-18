<script setup lang="ts">
import { computed, ref, onMounted, onUnmounted } from 'vue'
import { useReflectorStore } from '../stores/reflector'
import { useCallbookStore } from '../stores/callbook'
import { useLiveStore } from '../stores/live'

const props = defineProps<{
  selectedModule?: string
}>()

const emit = defineEmits<{
  (e: 'select', module: string): void
}>()

const live = useLiveStore()
const reflector = useReflectorStore()
const callbook = useCallbookStore()

// Helper to compute time ago string
const timeAgo = (ts?: string) => {
  if (!ts) return ''
  // dependency on now.value to trigger reactivity
  now.value // Access property to track dependency 
  const date = new Date(ts)
  const diff = Math.floor((Date.now() - date.getTime()) / 1000)
  
  if (diff < 60) return `${diff}s ago`
  if (diff < 3600) return `${Math.floor(diff/60)}m ago`
  return `${Math.floor(diff/3600)}h ago`
}

const moduleStats = computed(() => {
  // Get all configured modules
  const modules = reflector.modules.map(m => m.Name).sort()
  // Add modules seen in history if not config'd (fallback)
  const historyModules = new Set(live.lastHeard.map(h => h.module).filter(m => m && m.length === 1))
  const allModules = Array.from(new Set([...modules, ...historyModules])).sort()
  
  return allModules.map(mod => {
    // Get latest entry for this module
    const latest = live.lastHeard.find(h => h.module === mod)
    const active = latest ? live.isSessionActive(latest.id) : false
    
    // Get description from reflector store
    const desc = reflector.modules.find(m => m.Name === mod)?.Description || ''

    let elapsed = ''
    if (active && latest) {
       const start = new Date(latest.created_at).getTime()
       elapsed = ((Date.now() - start) / 1000).toFixed(1) + 's'
    } else if (latest && latest.duration && latest.duration > 0) {
       // Show duration of last transmission if idle
       const duration = latest.duration ?? 0
       elapsed = duration.toFixed(1) + 's'
    }
    
    return {
      name: mod,
      description: desc,
      active,
      latest,
      timeAgo: latest ? timeAgo(latest.created_at) : '',
      elapsed
    }
  })
})

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
</script>

<template>
  <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-4 mb-6">
    <div 
      v-for="stat in moduleStats" 
      :key="stat.name"
      @click="emit('select', stat.name)"
      class="relative group cursor-pointer transition-all duration-300 rounded-xl overflow-hidden border"
      :class="[
        selectedModule === stat.name 
          ? 'ring-2 ring-blue-500 shadow-lg shadow-blue-500/20 bg-blue-50/50 dark:bg-blue-900/20 border-blue-200 dark:border-blue-800' 
          : 'bg-white dark:bg-slate-900 hover:shadow-md border-slate-200 dark:border-slate-800'
      ]"
    >
      <!-- Active Strip -->
      <div v-if="stat.active" class="absolute left-0 top-0 bottom-0 w-1.5 bg-red-500 animate-pulse"></div>
      
      <div class="p-4 pl-5">
        <!-- Header -->
        <div class="flex justify-between items-start mb-2">
          <div class="flex items-center gap-2">
            <span class="text-2xl font-black font-mono"
                  :class="stat.active ? 'text-red-500' : 'text-slate-700 dark:text-slate-300'">
              {{ stat.name }}
            </span>
            <span v-if="stat.description" class="text-xs text-slate-400 truncate max-w-[120px]" :title="stat.description">
              {{ stat.description }}
            </span>
          </div>
          
          <div class="text-xs font-mono font-medium" 
               :class="stat.active ? 'text-red-500 font-bold' : 'text-slate-400'">
            <span v-if="stat.active" class="flex items-center gap-1">
               <span class="animate-pulse">●</span> {{ stat.elapsed }}
            </span>
            <span v-else class="flex items-center gap-1">
               {{ stat.timeAgo }}
               <span v-if="stat.elapsed" class="opacity-50">• {{ stat.elapsed }}</span>
            </span>
          </div>
        </div>
        
        <!-- Content -->
        <div v-if="stat.latest" class="space-y-1">
          <div class="font-bold text-lg truncate leading-tight"
               :class="stat.active ? 'text-red-600 dark:text-red-400' : 'text-blue-600 dark:text-blue-400'">
            {{ stat.latest.my }}
            <div class="text-[10px] font-normal text-slate-500 dark:text-slate-400 truncate">
               {{ [callbook.getName(stat.latest.my), callbook.getLocation(stat.latest.my)].filter(Boolean).join(' · ') || '&nbsp;' }}
            </div>
          </div>
          <div class="text-xs text-slate-500 dark:text-slate-400 flex justify-between pt-1">
            <span class="truncate pr-2">{{ stat.latest.ur }}</span>
            <span class="font-mono opacity-70">{{ stat.latest.protocol }}</span>
          </div>
        </div>
        
        <!-- Empty State -->
        <div v-else class="h-[44px] flex items-center text-xs text-slate-300 italic">
          No recent activity
        </div>
      </div>
    </div>
  </div>
</template>
