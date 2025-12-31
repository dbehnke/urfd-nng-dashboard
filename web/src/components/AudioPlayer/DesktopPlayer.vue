<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'
import { usePlayerStore } from '../../stores/player'
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

// Quick fix: Add seek logic later. For now, read-only scrubber or Engine needs to watch currentTime?
// Watching currentTime in Engine is circular.
// Use `player.seek(time)` action.
</script>

<template>
  <div v-if="player.isUIOpen" 
       class="absolute top-16 right-0 w-full sm:w-[500px] bg-slate-900/95 backdrop-blur-xl border-l border-b border-l-slate-800 border-b-slate-800 shadow-2xl z-50 transition-all duration-300">
       
       <!-- Header / Close -->
       <div class="flex items-center justify-between px-4 py-2 border-b border-slate-800">
           <span class="text-xs font-bold text-slate-400 uppercase tracking-widest">Player</span>
           <button @click="player.toggleUI()" class="text-slate-400 hover:text-white transition-colors">
               <X :size="16" />
           </button>
       </div>

       <!-- Timeline -->
       <Timeline />
       
       <!-- Main Player Info -->
       <div class="p-6">
           <div class="flex items-start justify-between gap-4">
               <div>
                   <h2 class="text-2xl font-bold text-white truncate max-w-[300px]">
                       {{ player.currentTrack?.callsign || 'Waiting...' }}
                   </h2>
                   <div class="flex items-center gap-2 mt-1">
                        <span class="text-blue-400 font-medium text-sm">
                            Module {{ player.currentTrack?.module || '-' }}
                        </span>
                        <span v-if="player.isLiveMode" class="text-[10px] font-bold bg-red-500/20 text-red-500 px-1.5 py-0.5 rounded uppercase animate-pulse">
                            LIVE
                        </span>
                   </div>
                   <p class="text-slate-500 text-sm mt-2">
                       {{ player.currentTrack?.description || 'Reflector ready for digital voice.' }}
                   </p>
               </div>
               
               <!-- Visualizer -->
               <div class="w-24 h-12 bg-slate-950 rounded border border-slate-800/50">
                    <canvas ref="canvasEl" width="100" height="50" class="w-full h-full"></canvas>
               </div>
           </div>

           <!-- Progress -->
           <div class="mt-6 flex items-center gap-3 text-xs font-mono text-slate-500">
               <span>{{ formatTime(player.currentTime) }}</span>
               <div class="flex-1 h-1 bg-slate-800 rounded-full relative overflow-hidden">
                   <div class="absolute top-0 left-0 h-full bg-blue-500 rounded-full w-0"
                        :style="{ width: `${(player.currentTime / (player.duration || 1)) * 100}%` }"></div>
               </div>
               <span>{{ formatTime(player.duration) }}</span>
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

       <!-- Playlist (Simple List) -->
       <div class="border-t border-slate-800 max-h-[300px] overflow-y-auto bg-slate-950/50">
           <div class="px-4 py-2 text-xs font-bold text-slate-500 uppercase tracking-wider sticky top-0 bg-slate-900/90 backdrop-blur z-10">
               Up Next
           </div>
           <div v-for="(track, index) in player.playlist" :key="track.id" 
                @click="player.play(track, player.playlist)"
                class="px-4 py-3 flex items-center justify-between hover:bg-white/5 cursor-pointer transition-colors border-b border-white/5 last:border-0"
                :class="{'bg-blue-500/10': player.currentTrack?.id === track.id}">
                
                <div class="flex items-center gap-3">
                    <div class="w-8 text-center text-xs font-mono text-slate-600">
                        {{ index + 1 }}
                    </div>
                    <div>
                        <div class="text-sm font-medium text-slate-200" :class="{'text-blue-400': player.currentTrack?.id === track.id}">
                            {{ track.callsign }}
                        </div>
                        <div class="text-xs text-slate-500">
                            Module {{ track.module }} &middot; {{ formatTime(track.duration) }}
                        </div>
                    </div>
                </div>

                <div v-if="player.currentTrack?.id === track.id" class="text-blue-400">
                    <Activity :size="16" class="animate-pulse" />
                </div>
           </div>
       </div>

  </div>
</template>
