<script setup lang="ts">
import { ref, computed, onMounted, watch } from 'vue'
import { useVoiceStore } from '@/stores/voice'
import { useReflectorStore } from '@/stores/reflector'
import VoiceEngine from './VoiceEngine.vue'
import PTTButton from './PTTButton.vue'
import PasswordDialog from './PasswordDialog.vue'
import AudioLevelMeter from './AudioLevelMeter.vue'
import DataUsageIndicator from './DataUsageIndicator.vue'
import { Radio, AlertCircle } from 'lucide-vue-next'

// Stores
const voiceStore = useVoiceStore()
const reflectorStore = useReflectorStore()

// Refs
const voiceEngine = ref<InstanceType<typeof VoiceEngine> | null>(null)
const callsignInput = ref('')
const isTransmitting = ref(false)
const showPasswordDialog = ref(false)
const pendingPTT = ref(false)

// Computed
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

const stateDisplay = computed(() => {
  switch (voiceStore.state) {
    case 'listening':
      return { text: 'Listening', color: 'text-green-600' }
    case 'transmitting':
      return { text: 'Transmitting', color: 'text-red-600' }
    case 'rx_busy':
      return { text: 'RX Busy', color: 'text-yellow-600' }
    case 'disconnected':
    default:
      return { text: 'Disconnected', color: 'text-gray-600' }
  }
})

const canConnect = computed(() => {
  return voiceStore.callsign.length >= 3 && voiceStore.selectedModule !== null
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

const handlePTTDown = async () => {
  if (!voiceStore.canTransmit) {
    console.warn('Cannot transmit:', voiceStore.state)
    return
  }
  
  // Check if we have a password stored
  if (!voiceStore.password) {
    // Need to prompt for password
    pendingPTT.value = true
    showPasswordDialog.value = true
    return
  }
  
  // Start PTT with stored password
  const success = await voiceEngine.value?.startPTT(voiceStore.password)
  if (success) {
    isTransmitting.value = true
  }
}

const handlePTTUp = () => {
  if (!isTransmitting.value) {
    return
  }
  
  isTransmitting.value = false
  voiceEngine.value?.stopPTT()
}

const handleStateChange = (newState: 'listening' | 'transmitting' | 'rx_busy' | 'disconnected') => {
  voiceStore.setState(newState)
}

const handleError = (message: string) => {
  voiceStore.setError(message)
  console.error('Voice error:', message)
}

const handlePasswordSubmit = async (password: string) => {
  // Save password to store
  voiceStore.setPassword(password)
  showPasswordDialog.value = false
  
  // If PTT was pending, start it now
  if (pendingPTT.value && voiceEngine.value) {
    pendingPTT.value = false
    const success = await voiceEngine.value.startPTT(password)
    if (success) {
      isTransmitting.value = true
    }
  }
}

const handlePasswordCancel = () => {
  showPasswordDialog.value = false
  pendingPTT.value = false
}

// Lifecycle
onMounted(() => {
  // Load saved callsign/module from storage
  voiceStore.loadFromStorage()
  callsignInput.value = voiceStore.callsign
  
  // Check if voice is enabled via reflector config
  const config = reflectorStore.config || {}
  const voiceEnabled = config.VoiceEnable === true
  voiceStore.setEnabled(voiceEnabled)
  
  if (!voiceEnabled) {
    voiceStore.setError('Voice chat not enabled on reflector')
  }
})

// Watch for callsign/module changes to auto-connect
watch([() => voiceStore.callsign, () => voiceStore.selectedModule], ([cs, mod]) => {
  if (cs && mod && voiceEngine.value && !voiceEngine.value.isConnected) {
    voiceEngine.value.connect()
  }
})
</script>

<template>
  <div class="flex flex-col gap-4 p-4 bg-white dark:bg-gray-800 rounded-lg shadow-md">
    <!-- Header -->
    <div class="flex items-center gap-2">
      <Radio :size="20" class="text-blue-600" />
      <h3 class="text-lg font-semibold text-gray-900 dark:text-gray-100">Voice Chat</h3>
      <div class="ml-auto flex items-center gap-2">
        <span :class="['text-sm font-medium', stateDisplay.color]">
          {{ stateDisplay.text }}
        </span>
      </div>
    </div>

    <!-- Error Display -->
    <div v-if="voiceStore.lastError" class="flex items-start gap-2 p-3 bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 rounded">
      <AlertCircle :size="18" class="text-red-600 dark:text-red-400 flex-shrink-0 mt-0.5" />
      <p class="text-sm text-red-800 dark:text-red-300">{{ voiceStore.lastError }}</p>
    </div>

    <!-- Callsign Input -->
    <div class="flex flex-col gap-1">
      <label for="callsign" class="text-sm font-medium text-gray-700 dark:text-gray-300">
        Your Callsign
      </label>
      <div class="flex gap-2">
        <input
          id="callsign"
          v-model="callsignInput"
          type="text"
          placeholder="KC1XXX"
          maxlength="10"
          class="flex-1 px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 focus:outline-none focus:ring-2 focus:ring-blue-500"
          @keyup.enter="handleCallsignSubmit"
          :disabled="!voiceStore.isEnabled"
        />
        <button
          @click="handleCallsignSubmit"
          :disabled="!voiceStore.isEnabled || callsignInput.length < 3"
          class="px-4 py-2 bg-blue-600 hover:bg-blue-700 disabled:bg-gray-300 disabled:cursor-not-allowed text-white rounded-md transition-colors"
        >
          Set
        </button>
      </div>
    </div>

    <!-- Module Selector -->
    <div class="flex flex-col gap-1">
      <label for="module" class="text-sm font-medium text-gray-700 dark:text-gray-300">
        Module
      </label>
      <select
        id="module"
        :value="voiceStore.selectedModule || ''"
        @change="handleModuleChange"
        :disabled="!voiceStore.isEnabled"
        class="px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 focus:outline-none focus:ring-2 focus:ring-blue-500"
      >
        <option value="">Select module...</option>
        <option v-for="module in transcodedModules" :key="module" :value="module">
          Module {{ module }}
        </option>
      </select>
    </div>

    <!-- Audio Level Meters -->
    <div v-if="voiceStore.callsign && voiceStore.selectedModule" class="flex flex-col gap-2 px-2">
      <AudioLevelMeter 
        :level="voiceEngine?.rxLevel || 0" 
        label="RX"
      />
      <AudioLevelMeter 
        :level="voiceEngine?.txLevel || 0" 
        label="TX"
      />
    </div>

    <!-- Data Usage Indicator -->
    <div v-if="voiceStore.callsign && voiceStore.selectedModule && voiceEngine?.isConnected" class="px-2">
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

    <!-- PTT Button -->
    <div class="flex justify-center pt-2">
      <PTTButton
        :disabled="!voiceStore.canTransmit"
        :transmitting="isTransmitting"
        @ptt-down="handlePTTDown"
        @ptt-up="handlePTTUp"
      />
    </div>

    <!-- Help Text -->
    <p class="text-xs text-gray-500 dark:text-gray-400 text-center">
      Click and hold or press Space to transmit
    </p>

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
