<script setup lang="ts">
import { computed, ref, onMounted, watch } from 'vue'
import { usePlayerStore } from '../../stores/player'

const player = usePlayerStore()
const container = ref<HTMLElement | null>(null)

// Config
const PIXELS_PER_SECOND = 2
const MIN_BLOCK_WIDTH = 4

// Generate blocks from playlist
// Playlist is Newest (0) -> Oldest (N)
// We want to render: Oldest (Left) -> Newest (Right)
const blocks = computed(() => {
    // Reverse logic for display order: Oldest first
    const list = [...player.playlist].reverse()
    
    return list.map((track, index) => {
        const width = Math.max(MIN_BLOCK_WIDTH, track.duration * PIXELS_PER_SECOND)
        const isCurrent = player.currentTrack?.id === track.id

        // Wait, if playing Index K. 
        // Oldest is Index N. 
        // Newest is Index 0.
        // We are playing K.
        // Index < K are newer (future).
        // Index > K are older (past).
        
        // In the reversed list:
        // [Oldest, ..., K, ..., Newest]
        // Past -> Future
        
        return {
            track,
            width,
            isCurrent,
            index,
            // Class for coloring
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
    
    // Find the current block element?
    // Or just calculate offset.
    // Easier to use scrollIntoView if we render refs.
    const el = document.getElementById(`timeline-block-${player.currentTrack.id}`)
    if (el) {
        el.scrollIntoView({ behavior: 'smooth', block: 'nearest', inline: 'center' })
    }
}

watch(() => player.currentTrack, () => {
    // Auto scroll to generic position?
    setTimeout(scrollToCurrent, 100)
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
  <div class="w-full h-16 bg-slate-100 dark:bg-slate-900 border-b border-slate-200 dark:border-slate-800 relative group">
      
      <!-- Scroll Container -->
      <div ref="container" 
           class="absolute inset-0 overflow-x-auto overflow-y-hidden flex items-center px-[50%] no-scrollbar scroll-smooth">
          
          <div class="flex items-end h-10 gap-[2px]">
              <div v-for="block in blocks" 
                   :key="block.track.id"
                   :id="`timeline-block-${block.track.id}`"
                   @click="player.play(block.track, player.playlist)"
                   class="relative rounded-sm transition-all cursor-pointer group/block shrink-0"
                   :class="block.colorClass"
                   :style="{ width: `${block.width}px`, height: block.isCurrent ? '100%' : '60%' }">
                   
                  <!-- Tooltip -->
                  <div class="absolute bottom-full left-1/2 -translate-x-1/2 mb-2 hidden group-hover/block:block z-50 whitespace-nowrap">
                      <div class="bg-slate-800 text-white text-xs rounded px-2 py-1 shadow-lg flex flex-col items-center">
                          <span class="font-bold">{{ block.track.callsign }}</span>
                          <span class="opacity-75">{{ block.track.module }} &middot; {{ formatTime(block.track.duration) }}</span>
                          <span class="opacity-75 text-[10px]">{{ formatAxisTime(block.track.timestamp) }}</span>
                      </div>
                      <!-- Arrow -->
                      <div class="w-2 h-2 bg-slate-800 rotate-45 mx-auto -mt-1"></div>
                  </div>
                  
                  <!-- Axis Label (Every ~100px or distinct minute) -->
                  <div v-if="block.index % 5 === 0" 
                       class="absolute top-full mt-2 left-0 text-[10px] text-slate-400 font-mono whitespace-nowrap">
                      {{ formatAxisTime(block.track.timestamp) }}
                  </div>
                  
                  <!-- Axis Label (Every ~100px or distinct minute) -->
                  <div v-if="block.index % 5 === 0" 
                       class="absolute top-full mt-2 left-0 text-[10px] text-slate-400 font-mono whitespace-nowrap">
                      {{ formatAxisTime(block.track.timestamp) }}
                  </div>
                  
                  <!-- Axis Label (Every ~100px or distinct minute) -->
                  <!-- Simple heuristic: Show if track starts a new minute? Or just first block? -->
                  <!-- Since blocks are small, let's just show logic: index % 5 == 0? -->
                  <!-- Better: if there's enough space. -->
                  <div v-if="block.index % 5 === 0" 
                       class="absolute top-full mt-2 left-0 text-[10px] text-slate-400 font-mono whitespace-nowrap">
                      {{ formatAxisTime(block.track.timestamp) }}
                  </div>
              </div>
          </div>
          
      </div>

      <!-- Center Marker/Play head? -->
      <div class="absolute left-1/2 top-0 bottom-0 w-px bg-red-500/50 pointer-events-none z-10"></div>
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
