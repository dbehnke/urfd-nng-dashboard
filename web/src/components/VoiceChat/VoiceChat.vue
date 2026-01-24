<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, watch } from 'vue'
import { useVoiceStore } from '@/stores/voice'
import { useReflectorStore } from '@/stores/reflector'
import VoiceEngine from './VoiceEngine.vue'
import PTTButton from './PTTButton.vue'
import PasswordDialog from './PasswordDialog.vue'
import AudioLevelMeter from './AudioLevelMeter.vue'
import DataUsageIndicator from './DataUsageIndicator.vue'
import { Radio, AlertCircle, Lock, Unlock } from 'lucide-vue-next'

// Stores
const voiceStore = useVoiceStore()
const reflectorStore = useReflectorStore()

// Refs
const voiceEngine = ref<InstanceType<typeof VoiceEngine> | null>(null)
const callsignInput = ref('')
const showPasswordDialog = ref(false)
const pendingToggle = ref(false)
const countdown = ref(0)
let countdownInterval: number | null = null

// Computed
const isTransmitting = computed(() => {
  return voiceStore.state === 'transmitting' || 
         voiceStore.state === 'ptt_requesting' ||
         voiceStore.state === 'ptt_releasing'
})

const pttButtonDisabled = computed(() => {
  // Button should be enabled if:
  // 1. Can start transmitting (in listening state), OR
  // 2. Currently in any transmission-related state (can stop)
  // Button disabled only when: disconnected, rx_busy, or not configured
  if (voiceStore.state === 'disconnected') return true
  if (voiceStore.state === 'rx_busy') return true
  if (!voiceStore.isEnabled) return true
  if (voiceStore.callsign.length === 0) return true
  if (voiceStore.selectedModule === null) return true
  return false
})

const websocketUrl = computed(() => {
  const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
  return `${protocol}//${window.location.host}/ws/voice`
})

const transcodedModules = computed(() => {
  // Only show transcoded modules (not interlinked modules)
  // Based on reflector config, transcoded modules typically have VoiceEnable
  const modules = reflectorStore.modules || []
  
  // Filter to only transcoded modules
  // Assuming transcoded modules are in config.Voice or similar
  // For now, return all modules - this can be refined based on actual config structure
  return modules.map((m: any) => m.Name)
})

// Color indicator for gain level - centered at 100%
const gainColor = computed(() => {
  const gain = voiceStore.receiveGain
  // Center is green (75-125%)
  if (gain >= 75 && gain <= 125) return 'bg-green-500'
  // Left of center: blue gradient (below 75%)
  if (gain < 75) {
    if (gain < 25) return 'bg-blue-700' // Very low
    if (gain < 50) return 'bg-blue-600' // Low
    return 'bg-blue-500' // Slightly low
  }
  // Right of center: yellow/orange/red gradient (above 125%)
  if (gain <= 250) return 'bg-yellow-500' // Slightly high
  if (gain <= 500) return 'bg-orange-500' // High
  if (gain <= 750) return 'bg-red-500' // Very high
  return 'bg-red-700' // Extreme
})

const stateDisplay = computed(() => {
  switch (voiceStore.state) {
    case 'listening':
      return { text: '🎧 Listening', color: 'text-green-600' }
    case 'ptt_requesting':
      return { text: '⏳ Requesting...', color: 'text-yellow-600' }
    case 'transmitting':
      return { text: '📡 Transmitting', color: 'text-red-600' }
    case 'ptt_releasing':
      return { text: '⏳ Releasing...', color: 'text-yellow-600' }
    case 'rx_busy':
      return { text: '👂 RX Busy', color: 'text-blue-600' }
    case 'disconnected':
    default:
      return { text: '🔌 Disconnected', color: 'text-gray-600' }
  }
})

const canConnect = computed(() => {
  return voiceStore.callsign.length >= 3 && voiceStore.selectedModule !== null
})

const countdownDisplay = computed(() => {
  const minutes = Math.floor(countdown.value / 60)
  const seconds = countdown.value % 60
  return `${minutes}:${seconds.toString().padStart(2, '0')}`
})

// Methods
const handleCallsignSubmit = () => {
  if (voiceStore.setCallsign(callsignInput.value)) {
    // Callsign valid - connect if we also have a module
    if (canConnect.value && voiceEngine.value) {
      voiceEngine.value.connect()
    }
  }
}

const handleModuleChange = (event: Event) => {
  const target = event.target as HTMLSelectElement
  const module = target.value || null
  voiceStore.setModule(module)
  
  // Reconnect if already connected
  if (voiceEngine.value?.isConnected && module) {
    voiceEngine.value.disconnect()
    setTimeout(() => {
      voiceEngine.value?.connect()
    }, 100)
  }
}

const handlePTTToggle = async () => {
  console.log('[VoiceChat] Toggle clicked, state:', voiceStore.state)
  
  if (voiceStore.state === 'transmitting' || voiceStore.state === 'ptt_releasing') {
    // Currently transmitting or releasing, stop it
    console.log('[VoiceChat] Stopping transmission...')
    voiceEngine.value?.stopPTT()
  } else if (voiceStore.state === 'ptt_requesting') {
    // User clicked again while waiting for server grant - cancel the request
    console.log('[VoiceChat] Cancelling PTT request...')
    voiceEngine.value?.cancelPTTRequest()
  } else if (voiceStore.state === 'listening') {
    // Not transmitting, start it
    if (!voiceStore.canTransmit) {
      console.warn('Cannot transmit:', voiceStore.state)
      return
    }
    
    // Check if we have a password stored
    if (!voiceStore.password) {
      // Need to prompt for password
      pendingToggle.value = true
      showPasswordDialog.value = true
      return
    }
    
    // Start transmitting with stored password
    console.log('[VoiceChat] Starting transmission...')
    await voiceEngine.value?.startPTT(voiceStore.password)
  }
}

const startCountdown = () => {
  // Get max duration from engine (in seconds)
  const maxDuration = voiceEngine.value?.maxTransmitDuration || 180000
  countdown.value = Math.floor(maxDuration / 1000)
  
  // Update countdown every second
  countdownInterval = window.setInterval(() => {
    countdown.value--
    if (countdown.value <= 0) {
      stopCountdown()
    }
  }, 1000)
}

const stopCountdown = () => {
  if (countdownInterval) {
    clearInterval(countdownInterval)
    countdownInterval = null
  }
  countdown.value = 0
}

const handleStateChange = (newState: 'listening' | 'transmitting' | 'rx_busy' | 'disconnected' | 'ptt_requesting' | 'ptt_releasing') => {
  console.log('[VoiceChat] State change:', newState)
  voiceStore.setState(newState)
  
  // Manage countdown timer based on state
  if (newState === 'transmitting' && !countdownInterval) {
    startCountdown()
  } else if (newState !== 'transmitting' && countdownInterval) {
    stopCountdown()
  }
}

const handleError = (message: string) => {
  voiceStore.setError(message)
  console.error('Voice error:', message)
}

const handlePasswordSubmit = async (password: string) => {
  // Save password to store
  voiceStore.setPassword(password)
  showPasswordDialog.value = false
  
  // If toggle was pending, start transmitting now
  if (pendingToggle.value && voiceEngine.value) {
    pendingToggle.value = false
    await voiceEngine.value.startPTT(password)
  }
}

const handlePasswordCancel = () => {
  showPasswordDialog.value = false
  pendingToggle.value = false
}

const handleClearPassword = () => {
  voiceStore.clearPassword()
  voiceStore.setError(null)
}

// Lifecycle
onMounted(() => {
  // Load saved callsign/module from storage
  voiceStore.loadFromStorage()
  callsignInput.value = voiceStore.callsign
})

// Update gain in voice engine when store value changes
watch(() => voiceStore.receiveGain, (newGain) => {
  if (voiceEngine.value && voiceEngine.value.setReceiveGain) {
    voiceEngine.value.setReceiveGain(newGain)
  }
})

onUnmounted(() => {
  // Clean up countdown interval
  stopCountdown()
})

// Watch for reflector config changes to update voice enabled status
watch(() => reflectorStore.config, (config) => {
  if (config && config.VoiceEnable !== undefined) {
    const voiceEnabled = config.VoiceEnable === true
    voiceStore.setEnabled(voiceEnabled)
    
    if (!voiceEnabled) {
      voiceStore.setError('Voice chat not enabled on reflector')
    } else {
      // Clear the error if voice becomes enabled
      if (voiceStore.lastError === 'Voice chat not enabled on reflector') {
        voiceStore.setError(null)
      }
    }
  }
}, { deep: true, immediate: true })

// Watch for callsign/module changes to auto-connect
watch([() => voiceStore.callsign, () => voiceStore.selectedModule], ([cs, mod]) => {
  if (cs && mod && voiceEngine.value && !voiceEngine.value.isConnected) {
    voiceEngine.value.connect()
  }
})
</script>

<template>
  <div class="flex flex-col gap-3 p-3 bg-white dark:bg-gray-800 rounded-lg shadow-md">
    <!-- Header -->
    <div class="flex items-center gap-2">
      <Radio :size="18" class="text-blue-600" />
      <h3 class="text-base font-semibold text-gray-900 dark:text-gray-100">Web Transceiver</h3>
      <div class="ml-auto flex items-center gap-2">
        <span :class="['text-xs font-medium', stateDisplay.color]">
          {{ stateDisplay.text }}
        </span>
      </div>
    </div>

    <!-- Error Display -->
    <div v-if="voiceStore.lastError" class="flex items-start gap-2 p-2 bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 rounded">
      <AlertCircle :size="16" class="text-red-600 dark:text-red-400 flex-shrink-0 mt-0.5" />
      <p class="text-xs text-red-800 dark:text-red-300">{{ voiceStore.lastError }}</p>
    </div>

    <!-- Callsign Input -->
    <div class="flex flex-col gap-1.5">
      <label for="callsign" class="text-xs font-medium text-gray-700 dark:text-gray-300">
        Your Callsign
      </label>
      <input
        id="callsign"
        v-model="callsignInput"
        type="text"
        placeholder="KC1XXX"
        maxlength="10"
        class="w-full px-2.5 py-1.5 text-sm border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 focus:outline-none focus:ring-2 focus:ring-blue-500"
        @keyup.enter="handleCallsignSubmit"
        :disabled="!voiceStore.isEnabled"
      />
      <button
        @click="handleCallsignSubmit"
        :disabled="!voiceStore.isEnabled || callsignInput.length < 3"
        class="w-full px-3 py-1.5 text-sm bg-blue-600 hover:bg-blue-700 disabled:bg-gray-400 disabled:dark:bg-gray-600 disabled:cursor-not-allowed text-white font-medium rounded-md transition-colors shadow-sm"
      >
        Set Callsign
      </button>
    </div>

    <!-- Module Selector -->
    <div class="flex flex-col gap-1">
      <label for="module" class="text-xs font-medium text-gray-700 dark:text-gray-300">
        Module
      </label>
      <select
        id="module"
        :value="voiceStore.selectedModule || ''"
        @change="handleModuleChange"
        :disabled="!voiceStore.isEnabled"
        class="px-2.5 py-1.5 text-sm border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 focus:outline-none focus:ring-2 focus:ring-blue-500"
      >
        <option value="">Select module...</option>
        <option v-for="module in transcodedModules" :key="module" :value="module">
          Module {{ module }}
        </option>
      </select>
    </div>

    <!-- Password Lock Icon (only show if module selected) -->
    <div v-if="voiceStore.selectedModule" class="flex justify-center">
      <button
        @click="handleClearPassword"
        v-if="voiceStore.password"
        class="p-1.5 hover:bg-gray-100 dark:hover:bg-gray-700 rounded-full transition-colors"
        title="Password saved. Click to clear."
      >
        <Lock class="w-4 h-4 text-green-600 dark:text-green-500" />
      </button>
      <div
        v-else
        class="p-1.5"
        title="No password saved"
      >
        <Unlock class="w-4 h-4 text-gray-400 dark:text-gray-600" />
      </div>
    </div>

    <!-- Audio Level Meters -->
    <div v-if="voiceStore.callsign && voiceStore.selectedModule" class="flex flex-col gap-1.5">
      <AudioLevelMeter 
        :level="voiceEngine?.rxLevel || 0" 
        label="RX"
      />
      <AudioLevelMeter 
        :level="voiceEngine?.txLevel || 0" 
        label="TX"
      />
    </div>

    <!-- Receive Gain Control -->
    <div v-if="voiceStore.callsign && voiceStore.selectedModule" class="flex flex-col gap-1.5">
      <div class="flex items-center justify-between">
        <div class="flex items-center gap-1.5">
          <label for="rx-gain" class="text-xs font-medium text-gray-700 dark:text-gray-300">
            Receive Volume
          </label>
          <!-- Color indicator dot -->
          <span :class="['w-2 h-2 rounded-full', gainColor]"></span>
        </div>
        <div class="flex items-center gap-1.5">
          <input
            id="auto-gain"
            type="checkbox"
            :checked="voiceStore.autoGainControl"
            @change="voiceStore.setAutoGainControl(($event.target as HTMLInputElement).checked)"
            class="w-3.5 h-3.5 text-blue-600 bg-gray-100 border-gray-300 rounded focus:ring-blue-500 dark:focus:ring-blue-600 dark:ring-offset-gray-800 focus:ring-2 dark:bg-gray-700 dark:border-gray-600"
          />
          <label for="auto-gain" class="text-xs text-gray-600 dark:text-gray-400 cursor-pointer">
            Auto
          </label>
        </div>
      </div>
      <input
        id="rx-gain"
        type="range"
        min="0"
        max="1000"
        step="50"
        :value="voiceStore.receiveGain"
        :disabled="voiceStore.autoGainControl"
        @input="voiceStore.setReceiveGain(parseInt(($event.target as HTMLInputElement).value))"
        class="w-full h-1.5 bg-gray-200 dark:bg-gray-700 rounded-lg appearance-none cursor-pointer accent-blue-600 disabled:opacity-50 disabled:cursor-not-allowed"
      />
      <div class="flex justify-between text-xs text-gray-500 dark:text-gray-400">
        <span>|</span>
        <span>|</span>
        <span>|</span>
        <span>|</span>
        <span>|</span>
      </div>
    </div>

    <!-- Data Usage Indicator -->
    <div v-if="voiceStore.callsign && voiceStore.selectedModule && voiceEngine?.isConnected">
      <DataUsageIndicator :get-data-usage="() => voiceEngine?.getDataUsage() || {
        bytesReceived: 0,
        bytesSent: 0,
        totalBytes: 0,
        totalKB: 0,
        totalMB: 0,
        duration: 0,
        rateKbps: 0
      }" />
    </div>

    <!-- Countdown Timer (when transmitting) -->
    <div v-if="isTransmitting && countdown > 0" class="flex items-center justify-center gap-2 py-2 px-4 bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 rounded">
      <span class="text-2xl font-mono font-bold text-red-600 dark:text-red-400">
        {{ countdownDisplay }}
      </span>
      <span class="text-xs text-red-600 dark:text-red-400">remaining</span>
    </div>

    <!-- PTT Button -->
    <div class="flex justify-center pt-2">
      <PTTButton
        :disabled="pttButtonDisabled"
        :transmitting="isTransmitting"
        @toggle="handlePTTToggle"
      />
    </div>

    <!-- Voice Engine (headless) -->
    <VoiceEngine
      ref="voiceEngine"
      :websocket-url="websocketUrl"
      :module="voiceStore.selectedModule"
      :callsign="voiceStore.callsign"
      :is-transmitting="isTransmitting"
      @state-change="handleStateChange"
      @error="handleError"
    />

    <!-- Password Dialog -->
    <PasswordDialog
      :show="showPasswordDialog"
      @submit="handlePasswordSubmit"
      @cancel="handlePasswordCancel"
    />
  </div>
</template>
