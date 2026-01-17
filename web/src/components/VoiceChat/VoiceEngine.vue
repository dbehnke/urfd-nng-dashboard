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
const opusEncoder = ref<any>(null)
const mediaStream = ref<MediaStream | null>(null)
const micPermissionGranted = ref(false)
const micPermissionDenied = ref(false)
const isConnected = ref(false)
const currentState = ref<'listening' | 'transmitting' | 'rx_busy' | 'disconnected'>('disconnected')
const isReceivingAudio = ref(false)

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

// Request microphone permission
const requestMicPermission = async (): Promise<boolean> => {
  if (micPermissionGranted.value) {
    return true // Already granted
  }

  if (micPermissionDenied.value) {
    emit('error', 'Microphone permission was previously denied. Please enable it in browser settings.')
    return false
  }

  try {
    console.log('Requesting microphone permission...')
    
    // Request microphone access with echo cancellation
    const stream = await navigator.mediaDevices.getUserMedia({
      audio: {
        echoCancellation: true,
        noiseSuppression: true,
        autoGainControl: true,
        sampleRate: 8000,
        channelCount: 1
      }
    })

    mediaStream.value = stream
    micPermissionGranted.value = true
    console.log('Microphone permission granted')
    
    // Initialize Opus encoder for transmit
    await initOpusEncoder()
    
    return true
  } catch (error: any) {
    console.error('Microphone permission denied:', error)
    
    if (error.name === 'NotAllowedError' || error.name === 'PermissionDeniedError') {
      micPermissionDenied.value = true
      emit('error', 'Microphone permission denied. Please allow microphone access to transmit.')
    } else if (error.name === 'NotFoundError') {
      emit('error', 'No microphone found. Please connect a microphone to transmit.')
    } else {
      emit('error', `Microphone error: ${error.message}`)
    }
    
    return false
  }
}

// Initialize Opus encoder for transmit
const initOpusEncoder = async () => {
  try {
    // Import opus-recorder
    const { default: Recorder } = await import('opus-recorder')
    
    if (!mediaStream.value) {
      throw new Error('No media stream available')
    }

    // Create Opus recorder
    opusEncoder.value = new Recorder({
      encoderPath: '/opus-recorder/encoderWorker.min.js',
      encoderSampleRate: 8000,
      encoderApplication: 2048, // VOIP application
      streamPages: true,
      numberOfChannels: 1,
      encoderComplexity: 10,
      encoderBitRate: 12000, // 12kbps as per spec
      encoderFrameSize: 20, // 20ms frames
      sourceNode: mediaStream.value
    })

    // Handle encoded data
    opusEncoder.value.ondataavailable = (typedArray: Uint8Array) => {
      sendAudioData(typedArray)
    }

    console.log('Opus encoder initialized')
  } catch (error) {
    console.error('Failed to initialize Opus encoder:', error)
    emit('error', `Encoder initialization failed: ${error}`)
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
            isReceivingAudio.value = true
            await handleAudioData(data)
            break
          case 'voice_state':
            currentState.value = data.state
            
            // Update receiving flag based on state
            if (data.state === 'rx_busy') {
              isReceivingAudio.value = true
            } else if (data.state === 'listening') {
              isReceivingAudio.value = false
            }
            
            emit('stateChange', data.state)
            break
          case 'ptt_denied':
            console.warn('PTT denied:', data.reason)
            emit('error', `PTT denied: ${data.reason}. Active talker: ${data.active_talker || 'unknown'}`)
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
      currentState.value = 'disconnected'
      isReceivingAudio.value = false
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

// Start PTT (Push-to-Talk)
const startPTT = async (password?: string): Promise<boolean> => {
  if (!isConnected.value) {
    emit('error', 'Not connected to server')
    return false
  }

  // Half-duplex: Check if currently receiving audio
  if (isReceivingAudio.value || currentState.value === 'rx_busy') {
    emit('error', 'Cannot transmit while receiving audio (half-duplex mode)')
    return false
  }

  // Request microphone permission if not already granted
  const hasPermission = await requestMicPermission()
  if (!hasPermission) {
    return false
  }

  if (!opusEncoder.value) {
    emit('error', 'Encoder not initialized')
    return false
  }

  try {
    // Send PTT press message
    const pttMsg: any = {
      type: 'ptt_press',
      module: props.module,
      callsign: props.callsign
    }
    
    if (password) {
      pttMsg.password = password
    }
    
    ws.value?.send(JSON.stringify(pttMsg))
    
    // Start recording
    opusEncoder.value.start()
    
    currentState.value = 'transmitting'
    console.log('PTT started')
    return true
  } catch (error) {
    console.error('Failed to start PTT:', error)
    emit('error', `Failed to start transmit: ${error}`)
    return false
  }
}

// Stop PTT
const stopPTT = () => {
  if (!opusEncoder.value) return

  try {
    // Stop recording
    opusEncoder.value.stop()
    
    // Send PTT release message
    const pttMsg = {
      type: 'ptt_release',
      module: props.module,
      callsign: props.callsign
    }
    ws.value?.send(JSON.stringify(pttMsg))
    
    currentState.value = 'listening'
    console.log('PTT stopped')
  } catch (error) {
    console.error('Failed to stop PTT:', error)
  }
}

// Send encoded audio data to server
const sendAudioData = (opusData: Uint8Array) => {
  if (!ws.value || ws.value.readyState !== WebSocket.OPEN) {
    console.warn('WebSocket not open, cannot send audio')
    return
  }

  try {
    // Convert Opus data to base64
    const base64 = btoa(String.fromCharCode(...opusData))
    
    const audioMsg = {
      type: 'audio_data',
      module: props.module,
      callsign: props.callsign,
      opus: base64
    }
    
    ws.value.send(JSON.stringify(audioMsg))
  } catch (error) {
    console.error('Failed to send audio data:', error)
  }
}

// Lifecycle hooks
onMounted(async () => {
  await initAudio()
})

onUnmounted(() => {
  disconnect()
  
  // Stop encoder if running
  if (opusEncoder.value) {
    try {
      opusEncoder.value.stop()
    } catch (e) {
      // Ignore errors during cleanup
    }
  }
  
  // Stop media stream tracks
  if (mediaStream.value) {
    mediaStream.value.getTracks().forEach(track => track.stop())
  }
  
  // Close audio context
  if (audioContext.value) {
    audioContext.value.close()
  }
})

// Watch for module/callsign changes to reconnect
// Note: In production, use watch() from vue to handle this
defineExpose({
  connect,
  disconnect,
  isConnected,
  requestMicPermission,
  startPTT,
  stopPTT,
  micPermissionGranted,
  micPermissionDenied,
  currentState,
  isReceivingAudio
})
</script>

<template>
  <!-- This is a headless component - no UI -->
  <div style="display: none;"></div>
</template>
