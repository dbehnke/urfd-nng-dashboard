<script setup lang="ts">
import { ref, watch } from 'vue'
import { usePlayerStore } from '../../stores/player'

const player = usePlayerStore()
const audioEl = ref<HTMLAudioElement | null>(null)

// Web Audio API
let audioContext: AudioContext | null = null
let sourceNode: MediaElementAudioSourceNode | null = null
let gainNode: GainNode | null = null
let compressorNode: DynamicsCompressorNode | null = null
let analyserNode: AnalyserNode | null = null

// Expose analyser for visualizers? 
// Stores usually don't hold complex objects like AnalyserNode.
// We might need to keep visualizer logic local or use a global singleton for AudioContext.
// For now, let's keep the AudioContext here. 
// If specific UI needs visualization, we might need a way to access it.
// The Desktop Player has a visualizer. The Mobile Player doesn't (based on "simplified").

// Initialize Audio Context
const initAudioContext = () => {
    if (!audioContext && audioEl.value) {
        const AudioContextClass = window.AudioContext || (window as any).webkitAudioContext
        audioContext = new AudioContextClass()
        sourceNode = audioContext.createMediaElementSource(audioEl.value)
        gainNode = audioContext.createGain()
        compressorNode = audioContext.createDynamicsCompressor()
        analyserNode = audioContext.createAnalyser()
        
        // Configure Analyser
        analyserNode.fftSize = 256
        
        // Configure Compressor
        compressorNode.threshold.value = -24
        compressorNode.knee.value = 30
        compressorNode.ratio.value = 12
        compressorNode.attack.value = 0.003
        compressorNode.release.value = 0.25

        updateRouting();
        
        // Initial volume
        gainNode.gain.value = player.volume;
        
        // Share analyser with Store? Or attach to window for components to find?
        // Ideally pass via provide/inject if components are children?
        // But Engine is sibling.
        // Let's attach to the store as a non-reactive property if possible, or simple global.
        // eslint-disable-next-line @typescript-eslint/no-explicit-any
        (window as any).__URFD_ANALYSER__ = analyserNode as any
    } else if (audioContext?.state === 'suspended') {
        audioContext.resume()
    }
}

const updateRouting = () => {
    if (!audioContext || !sourceNode || !gainNode || !compressorNode || !analyserNode) return
    
    sourceNode.disconnect()
    compressorNode.disconnect()
    gainNode.disconnect()
    analyserNode.disconnect()
    
    let currentNode: AudioNode = sourceNode

    if (player.isAgcEnabled) {
        sourceNode.connect(compressorNode)
        currentNode = compressorNode
    }

    currentNode.connect(analyserNode)
    analyserNode.connect(gainNode)
    gainNode.connect(audioContext.destination)
}

watch(() => player.isAgcEnabled, () => {
    updateRouting()
})

watch(() => player.volume, (vol) => {
    if (gainNode) {
        gainNode.gain.value = vol
    } else if (audioEl.value) {
        audioEl.value.volume = Math.min(vol, 1.0)
    }
})

// Playback Logic
const isBlocked = ref(false)

const attemptPlay = async () => {
    if (!audioEl.value) return
    try {
        await audioEl.value.play()
        isBlocked.value = false
    } catch (e: any) {
        console.error("Playback failed", e)
        if (e.name === 'NotAllowedError') {
            isBlocked.value = true
        }
    }
}

watch(() => player.currentTrack, (newTrack) => {
  if (newTrack && audioEl.value) {
    audioEl.value.src = newTrack.url
    initAudioContext() 
    if (player.isPlaying) {
      attemptPlay()
    }
    updateMediaSession()
  }
})

watch(() => player.isPlaying, (playing) => {
  if (!audioEl.value) return
  initAudioContext()
  if (playing) {
      if (audioEl.value.src) attemptPlay()
  } else {
      audioEl.value.pause()
      isBlocked.value = false
  }
  updateMediaSession()
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

// Media Session
const updateMediaSession = () => {
    if (!('mediaSession' in navigator) || !player.currentTrack) return

    navigator.mediaSession.metadata = new MediaMetadata({
        title: player.currentTrack.callsign,
        artist: player.currentTrack.description,
        album: `Module ${player.currentTrack.module}`,
        artwork: [
            { src: '/icon.png', sizes: '512x512', type: 'image/png' },
        ]
    })

    navigator.mediaSession.setActionHandler('play', () => player.togglePlay())
    navigator.mediaSession.setActionHandler('pause', () => player.togglePlay())
    navigator.mediaSession.setActionHandler('previoustrack', () => player.playPrevious())
    navigator.mediaSession.setActionHandler('nexttrack', () => player.playNext())
}

// Global "Resume" Overlay injection? 
// If blocked, we need user interaction.
// The Engine doesn't have UI. 
// We should expose `isBlocked` to the Store so UI components can show a "Resume" button?
// Or just keep a minimal UI here for the "Resume Overlay" which is critical?
// Let's add `isBlocked` to store? 
// Or render the Resume overlay here in a Teleport to body?
</script>

<template>
  <div class="hidden">
    <audio ref="audioEl" 
           @timeupdate="onTimeUpdate" 
           @ended="onEnded"
           preload="auto"></audio>
           
     <!-- Resume block overlay -->
     <Teleport to="body">
         <div v-if="isBlocked" 
              class="fixed top-20 left-1/2 -translate-x-1/2 z-[60] animate-bounce">
              <button @click="attemptPlay" 
                      class="bg-red-600 text-white px-6 py-3 rounded-full shadow-xl font-bold flex items-center gap-2 hover:bg-red-700 transition-colors">
                  <svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polygon points="5 3 19 12 5 21 5 3"></polygon></svg>
                  Resume Playback
              </button>
         </div>
     </Teleport>
  </div>
</template>
