import { defineStore } from 'pinia'
import { ref, reactive } from 'vue'
import { useReflectorStore } from './reflector'
import { usePlayerStore } from './player'

export interface Hearing {
    id: number
    my: string
    ur: string
    rpt1: string
    rpt2: string
    module: string
    protocol: string
    created_at: string
    duration?: number
    status?: 'active' | 'ended'
    audio_file?: string
}

export const useLiveStore = defineStore('live', () => {
    const lastHeard = ref<Hearing[]>([])
    const connected = ref(false)
    const activeSessions = reactive<Record<number, number>>({}) // Session ID -> Last Seen Timestamp
    const reflector = useReflectorStore()
    const player = usePlayerStore()

    let ws: WebSocket | null = null

    const endOfHistory = ref(false)
    const isLoadingMore = ref(false)

    const loadMoreHistory = async (initial = false) => {
        if (isLoadingMore.value || (endOfHistory.value && !initial)) return

        isLoadingMore.value = true

        let url = '/api/history?limit=50'
        if (!initial && lastHeard.value.length > 0) {
            // Find lowest ID to use as cursor
            const minId = Math.min(...lastHeard.value.map(h => h.id).filter(id => id > 0))
            if (minId > 0 && minId !== Infinity) {
                url += `&cursor=${minId}`
            }
        } else if (initial) {
            // Reset state on initial load
            endOfHistory.value = false
            // Don't clear lastHeard immediately to avoid flash, overwrite on success
        }

        try {
            const res = await fetch(url)
            const data: Hearing[] = await res.json()

            if (data.length < 50) {
                endOfHistory.value = true
            }

            if (initial) {
                lastHeard.value = data
            } else {
                // Filter out duplicates just in case
                const newItems = data.filter(n => !lastHeard.value.some(e => e.id === n.id))
                lastHeard.value.push(...newItems)
            }
        } catch (err) {
            console.error("Failed to load history:", err)
        } finally {
            isLoadingMore.value = false
        }
    }

    const connect = () => {
        // Initial Load
        loadMoreHistory(true)

        const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
        const host = window.location.host
        const wsUrl = `${protocol}//${host}/ws`

        ws = new WebSocket(wsUrl)

        ws.onopen = () => {
            connected.value = true
        }

        ws.onclose = () => {
            connected.value = false
            setTimeout(connect, 3000)
        }


        ws.onmessage = (msg) => {
            const ev = JSON.parse(msg.data)

            if (ev.type === 'hearing' || ev.type === 'closing') {
                if ((ev.type === 'closing' || ev.status === 'ended') && ev.id) {
                    delete activeSessions[ev.id]
                    player.isRecording = Object.keys(activeSessions).length > 0 // Update recording state

                    // Update history entry with final duration
                    const h = lastHeard.value.find(x => x.id === ev.id)
                    if (h) {
                        h.duration = ev.duration
                        h.status = 'ended'
                        if (ev.protocol) h.protocol = ev.protocol

                        // Capture audio file
                        if (ev.recording) h.audio_file = ev.recording
                        if (ev.audio_file) h.audio_file = ev.audio_file
                    }
                    return
                }

                // Update active heartbeat by ID
                if (ev.id) {
                    // Safety: Before marking this ID as active, ensure no other session for the SAME callsign is active
                    for (const id in activeSessions) {
                        const existing = lastHeard.value.find(h => h.id === Number(id))
                        if (existing && existing.my === ev.my && existing.id !== ev.id) {
                            delete activeSessions[Number(id)]
                        }
                    }
                    activeSessions[ev.id] = Date.now()
                    player.isRecording = true
                }

                // De-duplicate: search if we already have this session
                const existingIndex = ev.id ? lastHeard.value.findIndex(h => h.id === ev.id) : -1

                if (existingIndex !== -1) {
                    // Update existing entry
                    const existing = lastHeard.value[existingIndex]
                    if (existing) {
                        if (ev.module && existing.module !== ev.module) existing.module = ev.module
                        if (ev.protocol && existing.protocol !== ev.protocol) existing.protocol = ev.protocol
                        if (ev.ur && !existing.ur) existing.ur = ev.ur
                        if (ev.rpt2 && !existing.rpt2) existing.rpt2 = ev.rpt2
                        if (ev.created_at && !existing.created_at) existing.created_at = ev.created_at

                        // closing event updates (duration/file handled above in closing block usually?)
                        // Wait, 'closing' type is handled inside the first if block above.
                        // But standard 'hearing' status=ended might slip through?
                        // Actually, the top block catches type=closing OR status=ended.
                        // So this block handles only updates to existing Active sessions.
                    }
                } else if (ev.type === 'hearing' && ev.id && ev.my) {
                    // New Entry
                    const newEntry: Hearing = {
                        id: ev.id,
                        my: ev.my,
                        ur: ev.ur || 'CQCQCQ',
                        rpt1: ev.rpt1 || '',
                        rpt2: ev.rpt2 || '',
                        module: ev.module || '',
                        protocol: ev.protocol || '',
                        created_at: ev.created_at || new Date().toISOString(),
                        duration: ev.duration || 0,
                        status: ev.status === 'active' ? 'active' : 'ended',
                        audio_file: ev.recording || ev.audio_file
                    }

                    lastHeard.value.unshift(newEntry)
                    if (lastHeard.value.length > 200) {
                        lastHeard.value.pop()
                    }
                }
            } else {
                reflector.handleEvent(ev)
            }
        }
    }

    // Cleanup stale sessions every second
    setInterval(() => {
        const now = Date.now()
        for (const id in activeSessions) {
            const lastSeen = activeSessions[id]
            if (lastSeen !== undefined && now - lastSeen > 45000) { // 45s timeout (fallback only)
                delete activeSessions[id]
            }
        }
    }, 1000)

    const isSessionActive = (id?: number) => {
        if (!id) return false
        return !!activeSessions[id]
    }

    return { lastHeard, connected, connect, activeSessions, isSessionActive, loadMoreHistory, endOfHistory, isLoadingMore }
})
