import { defineStore } from 'pinia'
import { ref } from 'vue'

export interface Client {
    Callsign: string
    Protocol: string
    OnModule: string
    ConnectTime: string
}

export interface User {
    Callsign: string
    LastHeard: string
    OnModule: string
    ViaPeer: string
}

export interface Peer {
    Callsign: string
    Protocol: string
    ConnectTime: string
}

export const useReflectorStore = defineStore('reflector', () => {
    const clients = ref<Client[]>([])
    const users = ref<User[]>([])
    const peers = ref<Peer[]>([])
    const config = ref<Record<string, any>>({})

    const updateState = (state: any) => {
        if (state.Clients) clients.value = state.Clients
        if (state.Users) users.value = state.Users
        if (state.Peers) peers.value = state.Peers
        if (state.Configure) config.value = state.Configure
    }

    const handleEvent = (ev: any) => {
        if (ev.type === 'state') {
            updateState(ev)
        }
        // We can also handle client_connect/disconnect incrementally here
    }

    return { clients, users, peers, config, handleEvent }
})
