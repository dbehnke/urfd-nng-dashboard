import { defineStore } from 'pinia'
import { ref, computed } from 'vue'

export type VoiceState = 
  | 'disconnected'     // No WebSocket connection
  | 'listening'        // Ready to TX or RX
  | 'ptt_requesting'   // PTT requested, waiting for server grant
  | 'transmitting'     // Actively transmitting (server granted)
  | 'ptt_releasing'    // PTT release sent, waiting for server ack
  | 'rx_busy'          // Receiving audio from peer

export interface VoiceSession {
  callsign: string
  module: string | null
  state: VoiceState
  lastError: string | null
}

export const useVoiceStore = defineStore('voice', () => {
  // State
  const callsign = ref<string>('')
  const selectedModule = ref<string | null>(null)
  const state = ref<VoiceState>('disconnected')
  const lastError = ref<string | null>(null)
  const isEnabled = ref(false)
  const password = ref<string | null>(null)
  const passwordRequired = ref(false)
  const receiveGain = ref<number>(100) // Receive audio gain: 0-200%, default 100%

  // Computed
  const canTransmit = computed(() => {
    return isEnabled.value && 
           callsign.value.length > 0 && 
           selectedModule.value !== null && 
           state.value === 'listening'  // ONLY when listening
  })

  const session = computed<VoiceSession>(() => ({
    callsign: callsign.value,
    module: selectedModule.value,
    state: state.value,
    lastError: lastError.value
  }))

  // Actions
  const setCallsign = (cs: string) => {
    // Validate and normalize callsign
    const normalized = cs.trim().toUpperCase()
    
    // Basic callsign validation (3-10 alphanumeric characters)
    if (normalized.length >= 3 && normalized.length <= 10 && /^[A-Z0-9]+$/.test(normalized)) {
      callsign.value = normalized
      
      // Persist to localStorage
      localStorage.setItem('voice_callsign', normalized)
      
      lastError.value = null
      return true
    } else {
      lastError.value = 'Invalid callsign format (3-10 alphanumeric characters)'
      return false
    }
  }

  const setModule = (module: string | null) => {
    selectedModule.value = module
    
    // Persist to localStorage
    if (module) {
      localStorage.setItem('voice_module', module)
    } else {
      localStorage.removeItem('voice_module')
    }
  }

  const setState = (newState: VoiceState) => {
    state.value = newState
  }

  const setError = (error: string | null) => {
    lastError.value = error
  }

  const setEnabled = (enabled: boolean) => {
    isEnabled.value = enabled
  }

  const setPassword = (pwd: string | null) => {
    password.value = pwd
    
    // Persist to sessionStorage (not localStorage for security)
    if (pwd) {
      sessionStorage.setItem('voice_password', pwd)
    } else {
      sessionStorage.removeItem('voice_password')
    }
  }

  const setPasswordRequired = (required: boolean) => {
    passwordRequired.value = required
  }

  const setReceiveGain = (gain: number) => {
    // Clamp gain to 0-200% range
    const clampedGain = Math.max(0, Math.min(200, gain))
    receiveGain.value = clampedGain
    
    // Persist to localStorage
    localStorage.setItem('voice_receive_gain', clampedGain.toString())
  }

  const clearPassword = () => {
    password.value = null
    sessionStorage.removeItem('voice_password')
  }

  const loadFromStorage = () => {
    // Load persisted callsign and module
    const savedCallsign = localStorage.getItem('voice_callsign')
    const savedModule = localStorage.getItem('voice_module')
    const savedPassword = sessionStorage.getItem('voice_password')
    const savedGain = localStorage.getItem('voice_receive_gain')
    
    if (savedCallsign) {
      callsign.value = savedCallsign
    }
    
    if (savedModule) {
      selectedModule.value = savedModule
    }
    
    if (savedPassword) {
      password.value = savedPassword
    }
    
    if (savedGain) {
      const gain = parseInt(savedGain, 10)
      if (!isNaN(gain)) {
        receiveGain.value = Math.max(0, Math.min(200, gain))
      }
    }
  }

  const reset = () => {
    callsign.value = ''
    selectedModule.value = null
    state.value = 'disconnected'
    lastError.value = null
    password.value = null
    passwordRequired.value = false
    
    // Clear stored password from sessionStorage
    sessionStorage.removeItem('voice_password')
  }

  return {
    // State
    callsign,
    selectedModule,
    state,
    lastError,
    isEnabled,
    password,
    passwordRequired,
    receiveGain,
    
    // Computed
    canTransmit,
    session,
    
    // Actions
    setCallsign,
    setModule,
    setState,
    setError,
    setEnabled,
    setPassword,
    setPasswordRequired,
    setReceiveGain,
    clearPassword,
    loadFromStorage,
    reset
  }
})
