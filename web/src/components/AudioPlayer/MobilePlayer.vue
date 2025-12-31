<script setup lang="ts">
import { ref } from 'vue'
import { usePlayerStore } from '../../stores/player'
import { 
  Play, 
  Pause, 
  SkipForward, 
  SkipBack, 
  List,
  Clock
} from 'lucide-vue-next'

const player = usePlayerStore()
const showPlaylist = ref(false)

const formatTime = (seconds: number) => {
  if (!seconds || isNaN(seconds)) return "0:00"
  const m = Math.floor(seconds / 60)
  const s = Math.floor(seconds % 60)
  return `${m}:${s.toString().padStart(2, '0')}`
}
</script>

<template>
  <div v-if="player.currentTrack || player.isLiveMode" 
       class="bg-white dark:bg-slate-900 rounded-xl shadow-lg border border-slate-200 dark:border-slate-800 p-4 mb-4">
    
    <!-- Header Info -->
    <div class="flex items-start justify-between mb-4">
        <div class="flex-1 min-w-0">
            <h3 class="font-bold text-lg text-slate-900 dark:text-white truncate">
                {{ player.currentTrack?.callsign || 'Live Mode Ready' }}
            </h3>
            <p class="text-sm text-slate-500 truncate">
                {{ player.currentTrack?.description || 'Waiting for transmission...' }}
            </p>
        </div>
        <div class="flex items-center gap-2">
            <span v-if="player.isLiveMode" class="text-[10px] font-bold bg-red-500/10 text-red-500 px-2 py-1 rounded-full uppercase animate-pulse border border-red-500/20">
                LIVE
            </span>
            <div class="px-2 py-1 rounded bg-slate-100 dark:bg-slate-800 text-xs font-mono font-bold text-slate-600 dark:text-slate-400">
                {{ player.currentTrack?.module || '-' }}
            </div>
        </div>
    </div>

    <!-- Scrubber -->
    <div class="mb-4">
        <div class="flex items-center justify-between text-xs text-slate-400 font-mono mb-1">
            <span>{{ formatTime(player.currentTime) }}</span>
            <span>{{ formatTime(player.duration) }}</span>
        </div>
        <div class="h-2 bg-slate-100 dark:bg-slate-800 rounded-full relative overflow-hidden">
             <div class="absolute top-0 left-0 h-full bg-blue-500 rounded-full transition-all duration-200"
                  :style="{ width: `${(player.currentTime / (player.duration || 1)) * 100}%` }"></div>
        </div>
    </div>

    <!-- Controls -->
    <div class="flex items-center justify-between">
        <button @click="showPlaylist = !showPlaylist" 
                class="p-2 text-slate-400 hover:text-blue-500 transition-colors"
                :class="{'text-blue-500': showPlaylist}">
            <List :size="20" />
        </button>

        <div class="flex items-center gap-4">
            <button @click="player.playPrevious()" class="p-2 text-slate-400">
                <SkipBack :size="24" />
            </button>
            <button @click="player.togglePlay()" 
                    class="w-12 h-12 rounded-full bg-blue-600 text-white flex items-center justify-center shadow-lg active:scale-95 transition-transform">
                <Pause v-if="player.isPlaying" :size="24" class="fill-current" />
                <Play v-else :size="24" class="fill-current ml-1" />
            </button>
            <button @click="player.playNext()" class="p-2 text-slate-400">
                <SkipForward :size="24" />
            </button>
        </div>

        <button @click="player.toggleLiveMode()" 
                class="p-2 transition-colors"
                :class="player.isLiveMode ? 'text-red-500' : 'text-slate-400'">
            <Clock :size="20" />
        </button>
    </div>

    <!-- Playlist -->
    <div v-if="showPlaylist && player.playlist.length > 0" class="mt-4 pt-4 border-t border-slate-100 dark:border-slate-800">
        <div class="text-xs font-bold text-slate-400 uppercase tracking-wider mb-2">Up Next</div>
        <div class="space-y-1 max-h-48 overflow-y-auto">
            <div v-for="(track, index) in player.playlist" :key="track.id"
                 @click="player.play(track, player.playlist)"
                 class="flex items-center gap-3 p-2 rounded hover:bg-slate-50 dark:hover:bg-slate-800 cursor-pointer"
                 :class="{'bg-blue-50 dark:bg-blue-900/20': player.currentTrack?.id === track.id}">
                 <div class="w-6 text-center text-[10px] font-mono text-slate-400">{{ index + 1 }}</div>
                 <div class="flex-1 min-w-0">
                     <div class="text-sm font-medium truncate" 
                          :class="player.currentTrack?.id === track.id ? 'text-blue-600 dark:text-blue-400' : 'text-slate-700 dark:text-slate-300'">
                         {{ track.callsign }}
                     </div>
                 </div>
                 <div class="text-xs text-slate-400 font-mono">{{ formatTime(track.duration) }}</div>
            </div>
        </div>
    </div>

  </div>
</template>
