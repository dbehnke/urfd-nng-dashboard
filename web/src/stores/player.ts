import { defineStore } from 'pinia'
import { ref } from 'vue'

export interface Track {
    id: number
    url: string
    callsign: string
    module: string
    duration: number
    description: string
    timestamp: number
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
    const isUIOpen = ref(false)

    // Actions
    const play = (track: Track, context: Track[] = [], autoOpen: boolean = false) => {
        currentTrack.value = track
        isPlaying.value = true
        currentTime.value = 0 // Reset time immediately to prevent UI flicker
        if (context.length > 0) {
            playlist.value = [...context]
        }
        if (autoOpen) {
            isUIOpen.value = true
        }
    }

    const setContext = (tracks: Track[]) => {
        // Just populate the playlist, don't start playing
        // Used for auto-fill on empty state
        playlist.value = [...tracks]
    }

    const toggleUI = () => {
        isUIOpen.value = !isUIOpen.value
    }

    const togglePlay = () => {
        isPlaying.value = !isPlaying.value
    }

    const toggleLiveMode = () => {
        isLiveMode.value = !isLiveMode.value
        if (isLiveMode.value) {
            // If returning to live mode, check if we need to catch up?
            // Usually means "Stop playing old stuff, wait for new stuff"
            currentTrack.value = null
            isPlaying.value = false
            queue.value = []
        }
    }

    const toggleAgc = () => {
        isAgcEnabled.value = !isAgcEnabled.value
    }

    const handleNewRecording = (track: Track) => {
        // Add to playlist logic
        // If we have a playlist active, this new track belongs at the start (Newest)
        // Check if track is already there?
        if (playlist.value.length > 0 && playlist.value[0]?.id !== track.id) {
            playlist.value.unshift(track)
        }

        if (isLiveMode.value) {
            // Live Mode: Auto-play if not busy
            if (!isPlaying.value) {
                play(track, [], false) // Don't auto-open on live incoming
            } else {
                // Busy playing something else?
                // If we are "Live", we generally want to hear the latest.
                // But if we are mid-track, maybe queue it.
                queue.value.push(track)
            }
        } else {
            // History Mode: Just added to playlist (above), user will reach it eventually.
        }
    }

    const onTrackEnd = () => {
        // Priority: Queue (Live Mode) -> Next in Playlist (Newer)
        if (isLiveMode.value && queue.value.length > 0) {
            const next = queue.value.shift()
            if (next) {
                play(next, [], false)
                return
            }
        }

        // Play Next (Newer)
        playNext()
    }

    const playNext = () => {
        // "Next" means "Newer" (Time Forward)
        // Playlist is: [Newest (0), ..., Oldest (N)]
        // Current is at Index K.
        // Next track is Index K-1.

        if (!currentTrack.value || playlist.value.length === 0) {
            isPlaying.value = false
            return
        }

        const idx = playlist.value.findIndex(t => t.id === currentTrack.value?.id)
        if (idx === -1) return // Current track not in list?

        if (idx > 0) {
            const nextTrack = playlist.value[idx - 1]
            if (nextTrack) play(nextTrack, playlist.value, false) // Keep existing playlist, don't force open
        } else {
            // We are at Index 0 (Newest).
            // Nothing newer.
            // Go to idle / Live waiting state.
            // Don't clear current track immediately so UI shows what just finished?
            // Or usually players stop.
            isPlaying.value = false
            // isLiveMode.value = true? // Maybe auto-re-enable live mode?
        }
    }

    const playPrevious = () => {
        // "Previous" means "Older" (Time Backward) -> Index K+1
        if (!currentTrack.value || playlist.value.length === 0) return

        const idx = playlist.value.findIndex(t => t.id === currentTrack.value?.id)
        if (idx !== -1 && idx < playlist.value.length - 1) {
            const prevTrack = playlist.value[idx + 1]
            // Previous usually intentional, but if minimized maybe kept minimized?
            // "Prevent Player Pop-up on sequential track change"
            // Previous is usually user initiated, but let's stick to consistent non-pop.
            if (prevTrack) play(prevTrack, [], false)
        }
    }

    return {
        currentTrack,
        isPlaying,
        isLiveMode,
        isUIOpen,
        queue,
        playlist,
        volume,
        currentTime,
        duration,
        isAgcEnabled,
        isRecording,
        play,
        toggleUI,
        togglePlay,
        toggleLiveMode,
        toggleAgc,
        handleNewRecording,
        onTrackEnd,
        playNext,
        playPrevious,
        setContext
    }
})
