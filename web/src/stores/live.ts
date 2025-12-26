import { defineStore } from 'pinia'
import { ref, reactive } from 'vue'
import { useReflectorStore } from './reflector'

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
}

export const useLiveStore = defineStore('live', () => {
    const lastHeard = ref<Hearing[]>([])
    const connected = ref(false)
    const activeSessions = reactive<Record<number, number>>({}) // Session ID -> Last Seen Timestamp
    const reflector = useReflectorStore()

    let ws: WebSocket | null = null

    const connect = () => {
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

            if (ev.type === 'hearing') {
                if (ev.status === 'ended' && ev.id) {
                    delete activeSessions[ev.id]
                    // Update history entry with final duration
                    const h = lastHeard.value.find(x => x.id === ev.id)
                    if (h) {
                        h.duration = ev.duration
                        h.status = 'ended'
                        if (ev.protocol) h.protocol = ev.protocol
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
                const exists = ev.id ? lastHeard.value.some(h => h.id === ev.id) : false

                if (!exists) {
                    lastHeard.value.unshift(ev)
                    if (lastHeard.value.length > 100) {
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
            if (lastSeen !== undefined && now - lastSeen > 4000) { // 4s timeout (slightly more than backend's 3s)
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
