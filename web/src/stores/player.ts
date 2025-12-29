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
        // Next: Newer (up list) or Older (down list)?
        // Audio player convention: Next = Track N+1.
        // In LastHeard, list is ordered New -> Old. 
        // So Track 0 is newest. Track 1 is older.
        // Usually "Next" means "Play the one after this".
        // If I listen to T_now. Next -> T_before.
        // Let's enable "Auto-Play Next" to go down the list (Time reverse).
        if (!currentTrack.value || playlist.value.length === 0) return

        const idx = playlist.value.findIndex(t => t.id === currentTrack.value?.id)
        if (idx !== -1 && idx < playlist.value.length - 1) {
            play(playlist.value[idx + 1], playlist.value)
        }
    }

    const playPrevious = () => {
        // Previous: Newer (up list).
        if (!currentTrack.value || playlist.value.length === 0) return

        const idx = playlist.value.findIndex(t => t.id === currentTrack.value?.id)
        if (idx > 0) {
            play(playlist.value[idx - 1], playlist.value)
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
        onTrackEnd,
        playNext,
        playPrevious
    }
})
