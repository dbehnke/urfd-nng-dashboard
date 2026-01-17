<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'

// Props
const props = defineProps<{
  websocketUrl: string
  module: string | null
  callsign: string
  isTransmitting: boolean
}>()

// Emits
const emit = defineEmits<{
  stateChange: [state: 'listening' | 'transmitting' | 'rx_busy' | 'disconnected']
  error: [message: string]
}>()

// State
const ws = ref<WebSocket | null>(null)
const audioContext = ref<AudioContext | null>(null)
const opusDecoder = ref<any>(null)
const isConnected = ref(false)

// Initialize Web Audio API and Opus decoder
const initAudio = async () => {
  try {
    // Create AudioContext
    audioContext.value = new AudioContext({ sampleRate: 8000 })
    
    // Initialize Opus decoder (dynamically import)
    const OpusModule = await import('libopus.js')
    opusDecoder.value = await OpusModule.default()
    
    console.log('Audio engine initialized', {
      sampleRate: audioContext.value.sampleRate,
      state: audioContext.value.state
    })
  } catch (error) {
    console.error('Failed to initialize audio:', error)
    emit('error', `Audio initialization failed: ${error}`)
  }
}

// Connect to WebSocket
const connect = () => {
  if (!props.module || !props.callsign) {
    console.warn('Cannot connect: module or callsign missing')
    return
  }

  try {
    ws.value = new WebSocket(props.websocketUrl)
    
    ws.value.onopen = () => {
      console.log('WebSocket connected')
      isConnected.value = true
      
      // Send voice_start message
      const startMsg = {
        type: 'voice_start',
        module: props.module,
        callsign: props.callsign
      }
      ws.value?.send(JSON.stringify(startMsg))
    }
    
    ws.value.onmessage = async (event) => {
      try {
        const data = JSON.parse(event.data)
        
        switch (data.type) {
          case 'audio_data':
            await handleAudioData(data)
            break
          case 'voice_state':
            emit('stateChange', data.state)
            break
          default:
            console.log('Unknown message type:', data.type)
        }
      } catch (error) {
        console.error('Error handling WebSocket message:', error)
      }
    }
    
    ws.value.onerror = (error) => {
      console.error('WebSocket error:', error)
      emit('error', 'WebSocket connection error')
    }
    
    ws.value.onclose = () => {
      console.log('WebSocket disconnected')
      isConnected.value = false
      emit('stateChange', 'disconnected')
    }
  } catch (error) {
    console.error('Failed to connect WebSocket:', error)
    emit('error', `Connection failed: ${error}`)
  }
}

// Disconnect from WebSocket
const disconnect = () => {
  if (ws.value) {
    // Send voice_stop message
    const stopMsg = { type: 'voice_stop' }
    ws.value.send(JSON.stringify(stopMsg))
    
    ws.value.close()
    ws.value = null
  }
  isConnected.value = false
}

// Handle incoming audio data
const handleAudioData = async (data: { opus: string, from: string }) => {
  if (!audioContext.value || !opusDecoder.value) {
    console.warn('Audio not initialized')
    return
  }

  try {
    // Decode base64 Opus data
    const opusData = Uint8Array.from(atob(data.opus), c => c.charCodeAt(0))
    
    // Decode Opus to PCM
    // Note: This is a simplified version - actual implementation depends on libopus.js API
    const pcmData = await decodeOpus(opusData)
    
    // Play PCM audio
    if (pcmData) {
      playAudio(pcmData)
    }
  } catch (error) {
    console.error('Error decoding audio:', error)
  }
}

// Decode Opus data to PCM
const decodeOpus = async (opusData: Uint8Array): Promise<Float32Array | null> => {
  if (!opusDecoder.value) return null
  
  try {
    // This is a placeholder - actual implementation depends on libopus.js API
    // The decoder should be configured for 8kHz mono, 20ms frames (160 samples)
    // libopus.js API may vary, this needs to be adjusted based on actual API
    const decoded = opusDecoder.value.decode(opusData, 160, false)
    return new Float32Array(decoded)
  } catch (error) {
    console.error('Opus decode error:', error)
    return null
  }
}

// Play PCM audio through Web Audio API
const playAudio = (pcmData: Float32Array) => {
  if (!audioContext.value) return
  
  try {
    // Create audio buffer
    const audioBuffer = audioContext.value.createBuffer(
      1, // mono
      pcmData.length,
      audioContext.value.sampleRate
    )
    
    // Copy PCM data to buffer
    audioBuffer.getChannelData(0).set(pcmData)
    
    // Create buffer source and play
    const source = audioContext.value.createBufferSource()
    source.buffer = audioBuffer
    source.connect(audioContext.value.destination)
    source.start()
  } catch (error) {
    console.error('Error playing audio:', error)
  }
}

// Lifecycle hooks
onMounted(async () => {
  await initAudio()
})

onUnmounted(() => {
  disconnect()
  if (audioContext.value) {
    audioContext.value.close()
  }
})

// Watch for module/callsign changes to reconnect
// Note: In production, use watch() from vue to handle this
defineExpose({
  connect,
  disconnect,
  isConnected
})
</script>

<template>
  <!-- This is a headless component - no UI -->
  <div style="display: none;"></div>
</template>
