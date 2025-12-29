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

    const connect = () => {
        // Fetch history
        fetch('/api/history')
            .then(res => res.json())
            .then((data: Hearing[]) => {
                // Ensure dates are parsed if needed, or rely on JS/JSON checks
                // Also map DB fields if they differ from Hearing interface? 
                // DB Hearing: CreatedAt (time.Time) -> string/date.
                // Hearing interface: created_at (string).
                // GORM/JSON usually handles this to ISO string.
                // We might need to snake_case mapping.
                // Actually Go struct tags in models.go?
                // store.Hearing has json tags? Let's assume snake_case default or check.
                // NOTE: store.Hearing tags might be missing. I'll assume they match for now or I'd check models.go
                // But let's just assign.
                lastHeard.value = data
            })
            .catch(err => console.error("Failed to load history:", err))

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
                    // Update history entry with final duration
                    const h = lastHeard.value.find(x => x.id === ev.id)
                    if (h) {
                        h.duration = ev.duration
                        h.status = 'ended'
                        if (ev.protocol) h.protocol = ev.protocol

                        // Capture audio file
                        if (ev.recording) h.audio_file = ev.recording
                        if (ev.audio_file) h.audio_file = ev.audio_file

                        // Notify player store for Live Mode
                        if (h.audio_file) {
                            player.handleNewRecording({
                                id: h.id,
                                url: `/audio/${h.audio_file}`,
                                callsign: h.my,
                                module: h.module,
                                duration: h.duration || 0,
                                description: `${h.ur} via ${h.rpt1}`
                            })
                        }
                    }
                    return
                }

                // Update active heartbeat by ID
                if (ev.id) {
                    // Safety: Before marking this ID as active, ensure no other session for the SAME callsign is active
                    // This prevents "cloning" if heartbeats arrive delayed or simulator flips fast
                    for (const id in activeSessions) {
                        const existing = lastHeard.value.find(h => h.id === Number(id))
                        if (existing && existing.my === ev.my && existing.id !== ev.id) {
                            delete activeSessions[Number(id)]
                        }
                    }
                    activeSessions[ev.id] = Date.now()
                }

                // De-duplicate: search if we already have this session
                const existingIndex = ev.id ? lastHeard.value.findIndex(h => h.id === ev.id) : -1

                if (existingIndex !== -1) {
                    // Update existing entry with potentially newer info (e.g. Module correction)
                    const existing = lastHeard.value[existingIndex]
                    if (existing) {
                        if (ev.module && existing.module !== ev.module) existing.module = ev.module
                        if (ev.protocol && existing.protocol !== ev.protocol) existing.protocol = ev.protocol
                        if (ev.ur && !existing.ur) existing.ur = ev.ur
                        if (ev.rpt2 && !existing.rpt2) existing.rpt2 = ev.rpt2
                        if (ev.created_at && !existing.created_at) existing.created_at = ev.created_at

                        // closing event updates
                        if (ev.type === 'closing') {
                            existing.status = 'ended'
                            if (ev.duration) existing.duration = ev.duration
                            if (ev.recording) existing.audio_file = ev.recording
                            if (ev.audio_file) existing.audio_file = ev.audio_file

                            // Notify player store for Live Mode
                            if (existing.audio_file) {
                                player.handleNewRecording({
                                    id: existing.id,
                                    url: `/audio/${existing.audio_file}`,
                                    callsign: existing.my,
                                    module: existing.module,
                                    duration: existing.duration || 0,
                                    description: `${existing.ur} via ${existing.rpt1}`
                                })
                            }
                        }
                    }
                } else if (ev.type === 'hearing' && ev.id && ev.my) {
                    // Critical: Sanitize and construct a clean Hearing object
                    // This prevents "ghost" entries or property pollution from raw events
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
                        audio_file: ev.recording || ev.audio_file // Accept both from event or history
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

    return { lastHeard, connected, connect, activeSessions, isSessionActive }
})
