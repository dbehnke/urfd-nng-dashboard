<script setup lang="ts">
import { ref, onMounted, onUnmounted, watch, nextTick, computed } from 'vue'
import { usePlayerStore } from '../../stores/player'
import { useCallbookStore } from '../../stores/callbook'

import { 
  Play, 
  Pause, 
  SkipForward, 
  SkipBack, 
  Volume2, 
  Activity,
  X,
  Clock
} from 'lucide-vue-next'
import Timeline from './Timeline.vue'

const player = usePlayerStore()
const callbook = useCallbookStore()

// Module Filtering
const currentIndex = computed(() => {
    if (!player.currentTrack) return -1
    return player.playlist.findIndex(t => t.id === player.currentTrack?.id)
})

const visiblePlaylist = computed(() => {
    // Default Behavior (Show all context provided by parent)
    const end = Math.min(player.playlist.length, currentIndex.value + 11)
    return player.playlist.slice(0, end).reverse()
})

const timeToLive = computed(() => {
    if (!player.currentTrack) return "0:00"
    
    let totalSeconds = 0
    
    // Remaining time in current track
    if (player.duration > player.currentTime) {
        totalSeconds += (player.duration - player.currentTime)
    }
    
    // Duration of all newer tracks (Indices 0 to current-1)
    if (currentIndex.value > 0) {
        const newerTracks = player.playlist.slice(0, currentIndex.value)
        totalSeconds += newerTracks.reduce((sum, t) => sum + t.duration, 0)
    }
    
    if (totalSeconds < 1) return "LIVE"
    return "-" + formatTime(totalSeconds)
})

const canvasEl = ref<HTMLCanvasElement | null>(null)
let animationFrame: number

const drawVisualizer = () => {
    animationFrame = requestAnimationFrame(drawVisualizer)
    
    // Access global analyser
    const analyser = (window as any).__URFD_ANALYSER__ as AnalyserNode
    if (!analyser || !canvasEl.value) return
    
    const canvas = canvasEl.value
    const ctx = canvas.getContext('2d')
    if (!ctx) return
    
    const bufferLength = analyser.frequencyBinCount
    const dataArray = new Uint8Array(bufferLength)
    analyser.getByteTimeDomainData(dataArray)
    
    ctx.clearRect(0, 0, canvas.width, canvas.height)
    ctx.lineWidth = 2
    ctx.strokeStyle = '#3b82f6' // Blue-500
    ctx.beginPath()

    const sliceWidth = canvas.width * 1.0 / bufferLength
    let x = 0

    for (let i = 0; i < bufferLength; i++) {
        const v = (dataArray[i] ?? 128) / 128.0
        const y = v * canvas.height / 2

        if (i === 0) ctx.moveTo(x, y)
        else ctx.lineTo(x, y)

        x += sliceWidth
    }
    ctx.lineTo(canvas.width, canvas.height / 2)
    ctx.stroke()
}

onMounted(() => {
    drawVisualizer()
})

onUnmounted(() => {
    cancelAnimationFrame(animationFrame)
})

const formatTime = (seconds: number) => {
  if (!seconds || isNaN(seconds)) return "0:00"
  const m = Math.floor(seconds / 60)
  const s = Math.floor(seconds % 60)
  return `${m}:${s.toString().padStart(2, '0')}`
}

const formatPlaylistTime = (ts: number) => {
    if (!ts) return ''
    const d = new Date(ts)
    const now = new Date()
    const isToday = d.getDate() === now.getDate() && d.getMonth() === now.getMonth()
    
    if (isToday) return d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
    return d.toLocaleDateString([], { month: 'numeric', day: 'numeric' }) + ' ' + d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
}

const playlistContainer = ref<HTMLElement | null>(null)
const trackRefs = ref<Record<number, HTMLElement>>({})

const setTrackRef = (el: any, id: number) => {
    if (el) trackRefs.value[id] = el
}

// Auto-scroll playlist to current track
watch(() => player.currentTrack, async (newTrack) => {
    if (!newTrack) return
    await nextTick()
    const el = trackRefs.value[newTrack.id]
    if (el && playlistContainer.value) {
        // Scroll with some padding to keep it centered
        const container = playlistContainer.value
        const top = el.offsetTop - container.offsetTop
        const height = el.offsetHeight
        const containerHeight = container.clientHeight
        
        container.scrollTo({
            top: top - (containerHeight / 2) + (height / 2),
            behavior: 'smooth'
        })
    }
}, { immediate: true })

// Quick fix: Add seek logic later. For now, read-only scrubber or Engine needs to watch currentTime?
// Watching currentTime in Engine is circular.
// Use `player.seek(time)` action.
</script>

<template>
  <div v-if="player.isUIOpen" 
       class="absolute top-16 right-0 w-full sm:w-[500px] bg-white/95 dark:bg-slate-900/95 backdrop-blur-xl border-l border-b border-l-slate-200 border-b-slate-200 dark:border-l-slate-800 dark:border-b-slate-800 shadow-2xl z-50 transition-all duration-300 flex flex-col max-h-[calc(100vh-4rem)]">

       
       <!-- Header / Close -->
       <div class="flex items-center justify-between px-4 py-3 border-b border-slate-200 dark:border-slate-800 shrink-0">
           <span class="text-xs font-bold text-slate-500 dark:text-slate-400 uppercase tracking-widest">Player</span>
           <button @click="player.toggleUI()" class="text-slate-400 hover:text-slate-900 dark:hover:text-white transition-colors">
               <X :size="16" />
           </button>
       </div>


       <!-- Main Player Info -->
       <div class="p-6">
           <div class="flex items-start justify-between gap-4">
               <div class="min-w-0 flex-1">
                   <h2 class="text-xl sm:text-2xl font-bold text-slate-900 dark:text-white truncate leading-none mb-1">
                       {{ player.currentTrack?.callsign || 'Waiting...' }}
                   </h2>
                   <!-- Callbook Info -->
                   <div v-if="player.currentTrack?.callsign" class="text-sm text-slate-600 dark:text-slate-300 font-medium leading-tight mb-1">
                        {{ callbook.getName(player.currentTrack.callsign) }}
                        <div class="text-[11px] text-slate-500 font-normal">
                            {{ callbook.getLocation(player.currentTrack.callsign) }}
                        </div>
                   </div>
                   <div class="flex items-center gap-2 mt-1">
                        <span class="text-blue-600 dark:text-blue-400 font-medium text-sm">
                            Module {{ player.currentTrack?.module || '-' }}
                        </span>
                        <span v-if="player.isLiveMode" class="text-[10px] font-bold bg-red-500/10 text-red-600 dark:text-red-400 px-1.5 py-0.5 rounded uppercase animate-pulse">
                            LIVE
                        </span>
                   </div>
                   <p class="text-slate-500 dark:text-slate-400 text-sm mt-1 truncate">
                       {{ player.currentTrack?.description || 'Reflector ready for digital voice.' }}
                   </p>
               </div>
               
               <!-- Visualizer -->
               <div class="w-24 h-12 bg-slate-100 dark:bg-slate-950 rounded border border-slate-200 dark:border-slate-800/50 shrink-0">
                    <canvas ref="canvasEl" width="100" height="50" class="w-full h-full opacity-60 dark:opacity-100"></canvas>
               </div>
           </div>

           <!-- Progress Info (No Bar) -->
           <div class="mt-6 flex items-center justify-between text-xs font-mono text-slate-500">
               <span>{{ formatTime(player.currentTime) }} <span class="text-slate-400 dark:text-slate-600">/</span> {{ formatTime(player.duration) }}</span>
               <span :class="{'text-red-500 font-bold': timeToLive === 'LIVE'}">{{ timeToLive }}</span>
           </div>

           <!-- Controls -->
           <div class="mt-6 flex items-center justify-between">
               
               <!-- Left: Volume -->
               <div class="flex items-center gap-2 group">
                   <Volume2 :size="18" class="text-slate-400 group-hover:text-blue-400 transition-colors" />
                   <input type="range" min="0" max="1" step="0.1" v-model.number="player.volume" 
                          class="w-20 h-1 bg-slate-700 rounded-lg appearance-none cursor-pointer accent-blue-500">
               </div>

               <!-- Center: Transport -->
               <div class="flex items-center gap-4">
                   <button @click="player.playPrevious()" class="text-slate-400 hover:text-white transition-colors">
                       <SkipBack :size="24" />
                   </button>
                   <button @click="player.togglePlay()" 
                           class="w-14 h-14 rounded-full bg-blue-600 hover:bg-blue-500 text-white flex items-center justify-center shadow-lg shadow-blue-900/20 transition-all active:scale-95">
                       <Pause v-if="player.isPlaying" :size="28" class="fill-current" />
                       <Play v-else :size="28" class="fill-current ml-1" />
                   </button>
                   <button @click="player.playNext()" class="text-slate-400 hover:text-white transition-colors">
                       <SkipForward :size="24" />
                   </button>
               </div>

               <!-- Right: Modes -->
               <div class="flex items-center gap-2">
                   <button @click="player.toggleAgc()" :class="player.isAgcEnabled ? 'text-blue-400' : 'text-slate-600'" title="AGC">
                       <Activity :size="18" class="rotate-90" />
                   </button>
                   <button @click="player.toggleLiveMode()" 
                           :class="player.isLiveMode ? 'text-red-500' : 'text-slate-600'" 
                           title="Live Mode">
                       <Clock :size="18" />
                   </button>
               </div>
           </div>
       </div>

       <!-- Timeline (Moved Here) -->
       <Timeline />

       <!-- Playlist (Simple List) -->
       <div class="border-t border-slate-200 dark:border-slate-800 flex-1 overflow-y-auto bg-slate-50/50 dark:bg-slate-950/50" ref="playlistContainer">
           <div class="px-4 py-2 text-xs font-bold text-slate-500 uppercase tracking-wider sticky top-0 bg-white/90 dark:bg-slate-900/90 backdrop-blur z-10 flex items-center justify-between">
               <span>Up Next</span>
               <!-- Context Label -->
               <span class="text-blue-600 dark:text-blue-400 cursor-default" title="Current Playlist Filter">
                   {{ player.contextName }}
               </span>
           </div>
           <div v-for="(track, index) in visiblePlaylist" :key="track.id" 
                 :ref="(el) => setTrackRef(el, track.id)"
                 @click="player.play(track, player.playlist, true)"
                 class="px-4 py-3 flex items-center justify-between hover:bg-slate-100 dark:hover:bg-white/5 cursor-pointer transition-colors border-b border-slate-200 dark:border-white/5 last:border-0 relative"
                 :class="{'bg-slate-100 dark:bg-slate-900': player.currentTrack?.id === track.id}">
                 
                 <!-- Active Indicator Line -->
                 <div v-if="player.currentTrack?.id === track.id" 
                      class="absolute left-0 top-0 bottom-0 w-1 bg-red-500 shadow-[0_0_10px_rgba(239,68,68,0.5)]"></div>

                 <div class="flex items-center gap-3 pl-2">
                     <div>
                         <div class="text-sm font-medium text-slate-700 dark:text-slate-200" :class="{'text-red-600 dark:text-red-400': player.currentTrack?.id === track.id}">
                             {{ track.callsign }}
                         </div>
                         <div class="text-xs text-slate-500 flex items-center gap-2">
                             <span>Module {{ track.module }} &middot; {{ formatTime(track.duration) }}</span>
                             <span v-if="track.timestamp" class="text-slate-400 dark:text-slate-600">&middot; {{ formatPlaylistTime(track.timestamp) }}</span>
                         </div>
                     </div>
                 </div>

                 <div v-if="player.currentTrack?.id === track.id" class="text-red-500">
                     <Activity :size="16" class="animate-pulse" />
                 </div>
                 <!-- Count to Live (Index) -->
                 <div v-else class="text-xs font-mono font-bold text-slate-300 dark:text-slate-700">
                     {{ visiblePlaylist.length - 1 - index > 0 ? visiblePlaylist.length - 1 - index : 'LIVE' }}
                 </div>

            </div>
       </div>

  </div>
</template>
