/**
 * API service for managing per-user, per-module voice gain preferences
 */

export interface UserGainPreference {
  id: number
  created_at: string
  updated_at: string
  callsign: string
  module: string
  gain: number
  last_heard: string
}

/**
 * Retrieve saved gain preference for a specific callsign+module combination
 * @param callsign User's callsign (e.g., "KF8S")
 * @param module Module letter (e.g., "A", "D", "M")
 * @returns Saved gain (0-1000) or null if not found
 */
export async function getUserGain(callsign: string, module: string): Promise<number | null> {
  try {
    const response = await fetch(`/api/voice/gain/${callsign}/${module}`)
    if (response.status === 404) {
      return null // No saved preference
    }
    if (!response.ok) {
      throw new Error(`Failed to fetch gain preference: ${response.statusText}`)
    }
    const data: UserGainPreference = await response.json()
    return data.gain
  } catch (error) {
    console.error(`Failed to get gain for ${callsign}/${module}:`, error)
    return null
  }
}

/**
 * Save or update gain preference for a specific callsign+module combination
 * @param callsign User's callsign
 * @param module Module letter
 * @param gain Gain value (0-1000)
 * @returns true if saved successfully, false otherwise
 */
export async function saveUserGain(callsign: string, module: string, gain: number): Promise<boolean> {
  try {
    const response = await fetch('/api/voice/gain', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ callsign, module, gain })
    })
    
    if (!response.ok) {
      throw new Error(`Failed to save gain preference: ${response.statusText}`)
    }
    
    return true
  } catch (error) {
    console.error(`Failed to save gain for ${callsign}/${module}:`, error)
    return false
  }
}
