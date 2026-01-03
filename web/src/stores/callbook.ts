import { defineStore } from 'pinia'
import { reactive } from 'vue'

export interface UserDetails {
    callsign: string
    name: string
    surname: string
    city: string
    state: string
    country: string
}

export const useCallbookStore = defineStore('callbook', () => {
    // Cache: Callsign -> Details
    const cache = reactive(new Map<string, UserDetails | null>())
    // Pending requests to dedup in-flight lookups
    const pending = new Map<string, Promise<UserDetails | null>>()

    const lookup = async (callsign: string): Promise<UserDetails | null> => {
        if (!callsign) return null

        callsign = callsign.toUpperCase().trim()

        // Return if in cache (even if null/not found)
        if (cache.has(callsign)) {
            return cache.get(callsign)!
        }

        // Dedup in-flight requests
        if (pending.has(callsign)) {
            return pending.get(callsign)!
        }

        const promise = (async () => {
            try {
                const res = await fetch(`/api/callbook/${callsign}`)
                if (res.ok) {
                    const data = await res.json()
                    const details: UserDetails = {
                        callsign: data.callsign,
                        name: data.name,
                        surname: data.surname,
                        city: data.city,
                        state: data.state,
                        country: data.country
                    }
                    cache.set(callsign, details)
                    return details
                }
            } catch (e) {
                console.warn(`Callbook lookup failed for ${callsign}`, e)
            }
            // Mark as null so we don't retry forever in this session
            cache.set(callsign, null)
            return null
        })()

        pending.set(callsign, promise)

        try {
            return await promise
        } finally {
            pending.delete(callsign)
        }
    }

    // Helper to get formatted string immediately (or trigger lookup if missing)
    // Returns "Loading..." or "City, State" etc.
    // NOTE: This triggers the async lookup as a side effect if missing!
    const getLocation = (callsign: string): string => {
        lookup(callsign)
        // Since lookup is async, we can't wait here. We rely on reactivity.
        // However, the function returns a promise. 
        // We need a reactive way to access the cache.
        // Better usage pattern: Component manually calls lookup() on mount/watch, 
        // then uses `get(callsign)` which accesses the reactive cache.

        if (cache.has(callsign)) {
            const details = cache.get(callsign)
            if (details) {
                const parts = [details.city, details.state, details.country].filter(Boolean)
                return parts.join(', ')
            }
            return '' // Not found
        }
        return '' // Loading
    }

    const getName = (callsign: string): string => {
        lookup(callsign) // Trigger side effect
        if (cache.has(callsign)) {
            const details = cache.get(callsign)
            if (details) {
                return [details.name, details.surname].filter(Boolean).join(' ')
            }
        }
        return ''
    }

    return {
        lookup,
        cache,
        getLocation,
        getName
    }
})
