import { defineStore } from 'pinia'
import { ref } from 'vue'

export interface Track {
    id: number
    url: string
    callsign: string
    module: string
    duration: number
    description: string
}

export const usePlayerStore = defineStore('player', () => {
    // State
    const currentTrack = ref<Track | null>(null)
    const isPlaying = ref(false)
    const isLiveMode = ref(true) // Default to live mode
    const queue = ref<Track[]>([])
    const volume = ref(1.0)
    const currentTime = ref(0)
    const duration = ref(0)

    // Actions
    const play = (track: Track) => {
        // If playing the same track, do nothing or toggle?
        // Let's assume play means start this track now.
        currentTrack.value = track
        isPlaying.value = true
        // If we manually play a track, should we disable live mode?
        // Often yes, otherwise new tracks interrupt.
        // Or we keep live mode but this just plays one old track?
        // Let's pause Live Mode temporarily if user clicks an old track?
        // For now, let's keep it simple.
    }

    const togglePlay = () => {
        isPlaying.value = !isPlaying.value
    }

    const toggleLiveMode = () => {
        isLiveMode.value = !isLiveMode.value
    }

    const handleNewRecording = (track: Track) => {
        if (isLiveMode.value) {
            // If we are idle, play immediately
            if (!isPlaying.value) {
                play(track)
            } else {
                // Enqueue? Or just let it go?
                // "Live Mode usually means jump to newest"
                // But if someone is speaking long, we might want to queue it.
                // Let's add to queue.
                queue.value.push(track)
            }
        }
    }

    const onTrackEnd = () => {
        isPlaying.value = false
        if (queue.value.length > 0) {
            const next = queue.value.shift()
            if (next) {
                play(next)
            }
        }
    }

    return {
        currentTrack,
        isPlaying,
        isLiveMode,
        queue,
        volume,
        currentTime,
        duration,
        play,
        togglePlay,
        toggleLiveMode,
        handleNewRecording,
        onTrackEnd
    }
})
