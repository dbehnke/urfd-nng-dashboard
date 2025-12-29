<script setup lang="ts">
import { ref, watch, computed } from 'vue'
import { usePlayerStore } from '../stores/player'
import { 
  Play, 
  Pause, 
  SkipForward, 
  SkipBack, 
  Volume2, 
  Activity
} from 'lucide-vue-next'

const player = usePlayerStore()
const audioEl = ref<HTMLAudioElement | null>(null)

// Computed
const trackTitle = computed(() => {
  if (!player.currentTrack) return 'Waiting for transmission...'
  return `${player.currentTrack.callsign} (${player.currentTrack.module})`
})

const trackSubtitle = computed(() => {
  if (!player.currentTrack) return 'Live Mode Active'
  return player.currentTrack.description
})

// Watchers
watch(() => player.currentTrack, (newTrack) => {
  if (newTrack && audioEl.value) {
    audioEl.value.src = newTrack.url
    if (player.isPlaying) {
      audioEl.value.play().catch(e => console.error("Playback failed", e))
    }
  }
})

watch(() => player.isPlaying, (playing) => {
  if (!audioEl.value) return
  if (playing) {
      // Check if we have source
      if (audioEl.value.src) {
        audioEl.value.play().catch(e => console.error("Playback failed", e))
      }
  } else {
    audioEl.value.pause()
  }
})

watch(() => player.volume, (vol) => {
  if (audioEl.value) {
    audioEl.value.volume = vol
  }
})

// Handlers
const onTimeUpdate = () => {
  if (audioEl.value) {
    player.currentTime = audioEl.value.currentTime
    player.duration = audioEl.value.duration
  }
}

const onEnded = () => {
  player.onTrackEnd()
}

const togglePlay = () => {
    player.togglePlay()
}

const toggleLive = () => {
    player.toggleLiveMode()
}

const seek = (e: Event) => {
  const target = e.target as HTMLInputElement
  if (audioEl.value) {
    audioEl.value.currentTime = parseFloat(target.value)
  }
}

const formatTime = (seconds: number) => {
  if (!seconds || isNaN(seconds)) return "0:00"
  const m = Math.floor(seconds / 60)
  const s = Math.floor(seconds % 60)
  return `${m}:${s.toString().padStart(2, '0')}`
}
</script>

<template>
  <div class="fixed bottom-0 left-0 right-0 bg-white dark:bg-slate-900 border-t border-slate-200 dark:border-slate-800 shadow-lg z-50 transition-transform duration-300"
       :class="{'translate-y-full': !player.currentTrack && !player.isLiveMode}">
    
    <!-- Audio Element (Hidden) -->
    <audio ref="audioEl" 
           @timeupdate="onTimeUpdate" 
           @ended="onEnded"
           preload="auto"></audio>

    <div class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-3">
      <div class="flex items-center gap-4">
        
        <!-- Track Info -->
        <div class="flex-1 min-w-0">
          <div class="flex items-center gap-2">
            <h3 class="text-sm font-bold text-slate-900 dark:text-white truncate">
              {{ trackTitle }}
            </h3>
            <span v-if="player.isLiveMode" 
                  class="px-1.5 py-0.5 rounded text-[10px] font-bold uppercase bg-red-100 text-red-600 dark:bg-red-900/30 dark:text-red-400 animate-pulse">
              LIVE
            </span>
          </div>
          <p class="text-xs text-slate-500 dark:text-slate-400 truncate">
            {{ trackSubtitle }}
          </p>
        </div>

        <!-- Controls (Center) -->
        <div class="flex flex-col items-center gap-1 flex-1">
          <div class="flex items-center gap-4">
            <button class="p-2 text-slate-400 hover:text-slate-600 dark:hover:text-slate-300 transition-colors"
                    title="Previous (Coming Soon)">
              <SkipBack :size="20" />
            </button>

            <button @click="togglePlay" 
                    class="p-3 bg-blue-600 hover:bg-blue-700 text-white rounded-full shadow-md transition-transform active:scale-95 flex items-center justify-center">
              <Pause v-if="player.isPlaying" :size="24" class="fill-current" />
              <Play v-else :size="24" class="fill-current ml-0.5" />
            </button>

            <button class="p-2 text-slate-400 hover:text-slate-600 dark:hover:text-slate-300 transition-colors"
                    title="Next (Skip)">
              <SkipForward :size="20" />
            </button>
          </div>
        </div>

        <!-- Live Mode & Volume -->
        <div class="flex items-center gap-4 flex-1 justify-end">
            <button @click="toggleLive" 
                    class="flex items-center gap-1.5 px-3 py-1.5 rounded-full text-xs font-bold transition-colors"
                    :class="player.isLiveMode ? 'bg-red-50 text-red-600 dark:bg-red-900/20 dark:text-red-400' : 'bg-slate-100 text-slate-500 dark:bg-slate-800 dark:text-slate-400'">
                <Activity :size="14" />
                <span>{{ player.isLiveMode ? 'LIVE ON' : 'LIVE OFF' }}</span>
            </button>

            <div class="hidden sm:flex items-center gap-2 w-24">
                <Volume2 :size="16" class="text-slate-400" />
                <input type="range" 
                       min="0" max="1" step="0.01" 
                       v-model.number="player.volume"
                       class="w-full h-1 bg-slate-200 dark:bg-slate-700 rounded-lg appearance-none cursor-pointer accent-blue-600">
            </div>
        </div>
      </div>

      <!-- Scrubber -->
      <div class="flex items-center gap-3 mt-2 text-xs text-slate-400 font-mono">
        <span>{{ formatTime(player.currentTime) }}</span>
        <div class="relative flex-1 h-3 group flex items-center">
            <input type="range" 
                   :min="0" 
                   :max="player.duration || 100" 
                   :value="player.currentTime"
                   @input="seek"
                   class="absolute w-full h-1 bg-slate-200 dark:bg-slate-700 rounded-lg appearance-none cursor-pointer accent-blue-600 z-10">
                   <!-- Buffer bar logic could go here -->
        </div>
        <span>{{ formatTime(player.duration) }}</span>
      </div>
    </div>
  </div>
</template>
