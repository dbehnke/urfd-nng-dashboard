<script setup lang="ts">
import { computed, ref, onMounted, watch } from 'vue'
import { usePlayerStore } from '../../stores/player'

const props = defineProps<{
  filter?: string
}>()

const player = usePlayerStore()
const container = ref<HTMLElement | null>(null)
const hoveredBlock = ref<any>(null)
const tooltipX = ref(0) // Relative to container

const setHoveredBlock = (block: any, event: MouseEvent) => {
    hoveredBlock.value = block
    if (container.value && event.target) {
        const containerRect = container.value.getBoundingClientRect()
        const targetRect = (event.target as HTMLElement).getBoundingClientRect()
        tooltipX.value = (targetRect.left - containerRect.left) + (targetRect.width / 2)
    }
}

// Config
const PIXELS_PER_SECOND = 2
const MIN_BLOCK_WIDTH = 4

const blocks = computed(() => {
    // 1. Get List (Clone)
    let list = [...player.playlist]
    
    // 2. Filter (if needed)
    if (props.filter && props.filter !== 'All') {
        list = list.filter(t => t.module === props.filter)
    }
    
    // 3. Reverse for display (Oldest -> Newest)
    list = list.reverse()
    
    return list.map((track, index) => {
        const width = Math.max(MIN_BLOCK_WIDTH, track.duration * PIXELS_PER_SECOND)
        const isCurrent = player.currentTrack?.id === track.id
        
        return {
            track,
            width,
            isCurrent,
            index,
            colorClass: isCurrent ? 'bg-blue-500' : 'bg-slate-300 dark:bg-slate-700 hover:bg-slate-400 dark:hover:bg-slate-600'
        }
    })
})

const formatAxisTime = (ts: number | undefined) => {
    if (!ts) return ''
    const d = new Date(ts)
    const now = new Date()
    const isToday = d.getDate() === now.getDate() && d.getMonth() === now.getMonth()
    
    if (isToday) return d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
    return d.toLocaleDateString([], { month: 'numeric', day: 'numeric' }) + ' ' + d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
}

const scrollToCurrent = () => {
    if (!container.value || !player.currentTrack) return
    const el = document.getElementById(`timeline-block-${player.currentTrack.id}`)
    if (el) {
        el.scrollIntoView({ behavior: 'smooth', block: 'nearest', inline: 'center' })
    }
}

watch(() => player.currentTrack, () => {
    setTimeout(scrollToCurrent, 100)
})

watch(() => props.filter, () => {
    setTimeout(scrollToCurrent, 100)
    // Clear hover if filter changes
    hoveredBlock.value = null
})

onMounted(() => {
    setTimeout(scrollToCurrent, 500)
})

const formatTime = (seconds: number) => {
  if (!seconds) return "0s"
  if (seconds < 60) return `${seconds.toFixed(1)}s`
  return `${(seconds/60).toFixed(1)}m`
}
</script>

<template>
  <!-- Increased height to h-20 (80px) to prevent clipping of axis labels -->
  <div class="w-full h-20 bg-slate-50 dark:bg-slate-900 border-b border-slate-200 dark:border-slate-800 relative group shrink-0">
      <!-- Scroll Container -->
      <div ref="container" 
           class="absolute inset-0 overflow-x-auto flex items-center px-[50%] no-scrollbar scroll-smooth"
           @mouseleave="hoveredBlock = null">
          
          <div class="flex items-end h-10 gap-[2px] mt-[-10px]">
              <div v-for="block in blocks" 
                   :key="block.track.id"
                   :id="`timeline-block-${block.track.id}`"
                   @click="player.play(block.track, player.playlist, true)"
                   @mouseenter="(e) => setHoveredBlock(block, e)"
                   class="relative rounded-sm transition-all cursor-pointer group/block shrink-0"
                   :class="block.colorClass"
                   :style="{ width: `${block.width}px`, height: block.isCurrent ? '100%' : '60%' }">
                  
                  <!-- Axis Label (Every 5th block) -->
                  <div v-if="block.index % 5 === 0" 
                       class="absolute top-full mt-2 left-0 text-[10px] text-slate-400 font-mono whitespace-nowrap pointer-events-none">
                      {{ formatAxisTime(block.track.timestamp) }}
                  </div>
              </div>
          </div>
          
      </div>

      <!-- Center Marker/Play head -->
      <div class="absolute left-1/2 top-0 bottom-0 w-px bg-red-500/50 pointer-events-none z-10"></div>

      <!-- Hover Tooltip -->
      <div v-if="hoveredBlock"
           class="absolute bottom-full mb-2 z-50 whitespace-nowrap pointer-events-none transition-all duration-75 ease-out"
           :style="{ left: tooltipX + 'px', transform: 'translateX(-50%)' }">
           <div class="bg-slate-800 text-white text-xs rounded px-3 py-2 shadow-xl border border-slate-700 flex flex-col items-center">
               <span class="font-bold text-sm text-blue-400">{{ hoveredBlock.track.callsign }}</span>
               <div class="flex items-center gap-1.5 mt-0.5">
                    <span class="font-mono bg-slate-700 px-1 rounded text-[10px]">Mod {{ hoveredBlock.track.module }}</span>
                    <span class="opacity-75">&middot; {{ formatTime(hoveredBlock.track.duration) }}</span>
               </div>
               <span class="opacity-75 text-[10px] mt-1 text-slate-400 font-mono">{{ formatAxisTime(hoveredBlock.track.timestamp) }}</span>
           </div>
           <!-- Arrow -->
           <div class="w-2 h-2 bg-slate-800 border-r border-b border-slate-700 rotate-45 mx-auto -mt-1"></div>
      </div>
  </div>
</template>

<style scoped>
.no-scrollbar::-webkit-scrollbar {
  display: none;
}
.no-scrollbar {
  -ms-overflow-style: none;
  scrollbar-width: none;
}
</style>
