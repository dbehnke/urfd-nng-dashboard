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
    const playlist = ref<Track[]>([])
    const queue = ref<Track[]>([])
    const volume = ref(1.0)
    const currentTime = ref(0)
    const duration = ref(0)
    const isAgcEnabled = ref(true)
    const isRecording = ref(false)

    // Actions
    const play = (track: Track, context: Track[] = []) => {
        currentTrack.value = track
        isPlaying.value = true
        if (context.length > 0) {
            playlist.value = context
        }
    }

    const togglePlay = () => {
        isPlaying.value = !isPlaying.value
    }

    const toggleLiveMode = () => {
        isLiveMode.value = !isLiveMode.value
    }

    const toggleAgc = () => {
        isAgcEnabled.value = !isAgcEnabled.value
    }

    const handleNewRecording = (track: Track) => {
        if (isLiveMode.value) {
            // New items are conceptually "first" in the playlist if it's reverse chrono.
            // If we are playing, and not busy, we play it.
            // If busy, we queue it.
            if (!isPlaying.value) {
                play(track)
            } else {
                queue.value.push(track)
            }
        }
    }

    const onTrackEnd = () => {
        // Priority: Queue (Live Mode) -> Next in Playlist
        if (queue.value.length > 0) {
            const next = queue.value.shift()
            if (next) {
                // Keep context if we want, or just play
                play(next)
                return
            }
        }

        // Auto-advance in playlist if Live Mode is OFF? Or usually if user clicked from list
        // "Next" in a list usually means "Next item". In a detailed list, that is index + 1?
        // Wait, LastHeard is Newest (0) to Oldest (N).
        // If I play (Index 0), the "next" logical track is Index 1 (older).
        // Let's assume standard behavior: Play through the list.
        if (currentTrack.value && playlist.value.length > 0) {
            playNext()
        } else {
            isPlaying.value = false
        }
    }

    const playNext = () => {
        // Next: Newer (up list, towards index 0)
        // User wants "Time Forward" (Historic -> Live)
        if (!currentTrack.value || playlist.value.length === 0) return

        const idx = playlist.value.findIndex(t => t.id === currentTrack.value?.id)
        if (idx > 0) {
            const nextTrack = playlist.value[idx - 1]
            if (nextTrack) play(nextTrack, playlist.value)
        } else if (idx === 0) {
            // We are at the newest track, and user wants "Next" (or auto-advance)
            // Go to "Live / Waiting" state
            currentTrack.value = null
            isPlaying.value = false
            isLiveMode.value = true
        }
    }

    const playPrevious = () => {
        // Previous: Older (down list, towards index N)
        if (!currentTrack.value || playlist.value.length === 0) return

        const idx = playlist.value.findIndex(t => t.id === currentTrack.value?.id)
        if (idx !== -1 && idx < playlist.value.length - 1) {
            const prevTrack = playlist.value[idx + 1]
            if (prevTrack) play(prevTrack, playlist.value)
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
        isAgcEnabled,
        isRecording,
        play,
        togglePlay,
        toggleLiveMode,
        toggleAgc,
        handleNewRecording,
        onTrackEnd,
        playNext,
        playPrevious
    }
})
