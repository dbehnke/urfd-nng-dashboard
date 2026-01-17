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
  const password = ref<string | null>(null)
  const passwordRequired = ref(false)

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

  const loadFromStorage = () => {
    // Load persisted callsign and module
    const savedCallsign = localStorage.getItem('voice_callsign')
    const savedModule = localStorage.getItem('voice_module')
    const savedPassword = sessionStorage.getItem('voice_password')
    
    if (savedCallsign) {
      callsign.value = savedCallsign
    }
    
    if (savedModule) {
      selectedModule.value = savedModule
    }
    
    if (savedPassword) {
      password.value = savedPassword
    }
  }

  const reset = () => {
    callsign.value = ''
    selectedModule.value = null
    state.value = 'disconnected'
    lastError.value = null
    password.value = null
    passwordRequired.value = false
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
    loadFromStorage,
    reset
  }
})
