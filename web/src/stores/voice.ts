import { defineStore } from 'pinia'
import { ref, computed } from 'vue'

export type VoiceState = 'disconnected' | 'listening' | 'transmitting' | 'rx_busy'

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

  // Computed
  const canTransmit = computed(() => {
    return isEnabled.value && 
           callsign.value.length > 0 && 
           selectedModule.value !== null && 
           state.value === 'listening'
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

  const loadFromStorage = () => {
    // Load persisted callsign and module
    const savedCallsign = localStorage.getItem('voice_callsign')
    const savedModule = localStorage.getItem('voice_module')
    
    if (savedCallsign) {
      callsign.value = savedCallsign
    }
    
    if (savedModule) {
      selectedModule.value = savedModule
    }
  }

  const reset = () => {
    callsign.value = ''
    selectedModule.value = null
    state.value = 'disconnected'
    lastError.value = null
  }

  return {
    // State
    callsign,
    selectedModule,
    state,
    lastError,
    isEnabled,
    
    // Computed
    canTransmit,
    session,
    
    // Actions
    setCallsign,
    setModule,
    setState,
    setError,
    setEnabled,
    loadFromStorage,
    reset
  }
})
