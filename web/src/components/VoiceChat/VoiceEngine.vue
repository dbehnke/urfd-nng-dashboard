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

// Audio level monitoring
const rxAnalyser = ref<AnalyserNode | null>(null)
const txAnalyser = ref<AnalyserNode | null>(null)
const rxLevel = ref(0)
const txLevel = ref(0)
let levelMonitorInterval: number | null = null

// Connection recovery
const reconnectAttempts = ref(0)
const maxReconnectAttempts = 5
const reconnectDelay = 2000 // Start with 2 seconds
let reconnectTimeout: number | null = null
const shouldReconnect = ref(true)

// Session timeout enforcement
const maxTransmitDuration = 120000 // 120 seconds in milliseconds
let transmitStartTime: number | null = null
let transmitTimeoutHandle: number | null = null

// Diagnostic logging
interface DiagnosticEvent {
  timestamp: number
  type: string
  details: any
}

const diagnosticLog = ref<DiagnosticEvent[]>([])
const maxLogEntries = 100

// Media Session API for background audio and lock screen controls
const activeTalker = ref<string | null>(null)
const mediaSessionSupported = ref(false)

// Wake Lock API to prevent screen sleep during sessions
const wakeLock = ref<any>(null)
const wakeLockSupported = ref(false)

const logDiagnostic = (type: string, details: any = {}) => {
  const event: DiagnosticEvent = {
    timestamp: Date.now(),
    type,
    details
  }
  
  diagnosticLog.value.push(event)
  
  // Keep only recent entries
  if (diagnosticLog.value.length > maxLogEntries) {
    diagnosticLog.value.shift()
  }
  
  // Also log to console in development
  if (import.meta.env.DEV) {
    console.log(`[VoiceEngine] ${type}:`, details)
  }
}

// Export diagnostic log
const getDiagnosticLog = () => {
  return diagnosticLog.value.map(event => ({
    time: new Date(event.timestamp).toISOString(),
    type: event.type,
    details: event.details
  }))
}

// Initialize Media Session API for background audio support
const initMediaSession = () => {
  if (!('mediaSession' in navigator)) {
    console.log('Media Session API not supported')
    mediaSessionSupported.value = false
    return
  }
  
  mediaSessionSupported.value = true
  
  try {
    // Set metadata for lock screen / notification
    navigator.mediaSession.metadata = new MediaMetadata({
      title: 'URFD Voice Chat',
      artist: activeTalker.value ? `Listening to ${activeTalker.value}` : 'Ready to listen',
      album: `Module ${props.module || 'None'}`,
      artwork: [
        // Use a radio icon or app logo if available
        { src: '/favicon.ico', sizes: '96x96', type: 'image/png' }
      ]
    })
    
    // Set playback state
    navigator.mediaSession.playbackState = 'playing'
    
    // Note: Action handlers would go here for play/pause/stop
    // For now, we'll keep it simple as the audio is continuous
    
    logDiagnostic('media_session_initialized', {
      supported: true,
      module: props.module
    })
    
    console.log('Media Session API initialized')
  } catch (error) {
    console.error('Failed to initialize Media Session:', error)
    logDiagnostic('media_session_init_error', { error: String(error) })
  }
}

// Update Media Session metadata when active talker changes
const updateMediaSessionMetadata = (talker: string | null) => {
  activeTalker.value = talker
  
  if (!mediaSessionSupported.value) return
  
  try {
    if (navigator.mediaSession.metadata) {
      navigator.mediaSession.metadata.artist = talker 
        ? `Listening to ${talker}` 
        : props.module 
        ? `Monitoring Module ${props.module}` 
        : 'Ready to listen'
    }
  } catch (error) {
    console.error('Failed to update Media Session metadata:', error)
  }
}

// Set Media Session playback state
const setMediaSessionState = (state: 'playing' | 'paused' | 'none') => {
  if (!mediaSessionSupported.value) return
  
  try {
    navigator.mediaSession.playbackState = state
  } catch (error) {
    console.error('Failed to set Media Session state:', error)
  }
}

// Request Wake Lock to prevent screen sleep
const requestWakeLock = async () => {
  if (!('wakeLock' in navigator)) {
    console.log('Wake Lock API not supported')
    wakeLockSupported.value = false
    return
  }
  
  wakeLockSupported.value = true
  
  // Only request if we don't already have one
  if (wakeLock.value !== null) {
    return
  }
  
  try {
    wakeLock.value = await (navigator as any).wakeLock.request('screen')
    
    wakeLock.value.addEventListener('release', () => {
      console.log('Wake Lock released')
      logDiagnostic('wake_lock_released', {})
    })
    
    logDiagnostic('wake_lock_acquired', {})
    console.log('Wake Lock acquired')
  } catch (error) {
    console.error('Failed to acquire Wake Lock:', error)
    logDiagnostic('wake_lock_error', { error: String(error) })
    wakeLock.value = null
  }
}

// Release Wake Lock
const releaseWakeLock = async () => {
  if (wakeLock.value === null) return
  
  try {
    await wakeLock.value.release()
    wakeLock.value = null
    logDiagnostic('wake_lock_released_manual', {})
    console.log('Wake Lock released manually')
  } catch (error) {
    console.error('Failed to release Wake Lock:', error)
    wakeLock.value = null
  }
}

// Initialize Web Audio API and Opus decoder
const initAudio = async () => {
  try {
    logDiagnostic('audio_init_start', {})
    
    // Create AudioContext
    audioContext.value = new AudioContext({ sampleRate: 8000 })
    
    // Create analyser nodes for audio level monitoring
    rxAnalyser.value = audioContext.value.createAnalyser()
    rxAnalyser.value.fftSize = 256
    rxAnalyser.value.smoothingTimeConstant = 0.8
    
    // Initialize Opus decoder (dynamically import)
    const OpusModule = await import('libopus.js')
    opusDecoder.value = await OpusModule.default()
    
    // Start audio level monitoring
    startLevelMonitoring()
    
    // Initialize Media Session API for background playback
    initMediaSession()
    
    logDiagnostic('audio_init_success', {
      sampleRate: audioContext.value.sampleRate,
      state: audioContext.value.state
    })
    
    console.log('Audio engine initialized', {
      sampleRate: audioContext.value.sampleRate,
      state: audioContext.value.state
    })
  } catch (error) {
    logDiagnostic('audio_init_error', { error: String(error) })
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

    // Create analyser for TX audio monitoring
    if (audioContext.value && !txAnalyser.value) {
      txAnalyser.value = audioContext.value.createAnalyser()
      txAnalyser.value.fftSize = 256
      txAnalyser.value.smoothingTimeConstant = 0.8
      
      // Connect media stream to analyser
      const source = audioContext.value.createMediaStreamSource(mediaStream.value)
      source.connect(txAnalyser.value)
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

// Start audio level monitoring
const startLevelMonitoring = () => {
  if (levelMonitorInterval) return
  
  levelMonitorInterval = window.setInterval(() => {
    // Monitor RX level
    if (rxAnalyser.value && isReceivingAudio.value) {
      rxLevel.value = getAudioLevel(rxAnalyser.value)
    } else {
      rxLevel.value = 0
    }
    
    // Monitor TX level
    if (txAnalyser.value && currentState.value === 'transmitting') {
      txLevel.value = getAudioLevel(txAnalyser.value)
    } else {
      txLevel.value = 0
    }
  }, 50) // Update every 50ms
}

// Stop audio level monitoring
const stopLevelMonitoring = () => {
  if (levelMonitorInterval) {
    clearInterval(levelMonitorInterval)
    levelMonitorInterval = null
  }
  rxLevel.value = 0
  txLevel.value = 0
}

// Get audio level from analyser (0-100)
const getAudioLevel = (analyser: AnalyserNode): number => {
  const dataArray = new Uint8Array(analyser.frequencyBinCount)
  analyser.getByteFrequencyData(dataArray)
  
  // Calculate RMS (root mean square) level
  let sum = 0
  for (let i = 0; i < dataArray.length; i++) {
    const value = dataArray[i]
    if (value !== undefined) {
      sum += value * value
    }
  }
  const rms = Math.sqrt(sum / dataArray.length)
  
  // Convert to 0-100 scale
  return (rms / 255) * 100
}

// Connect to WebSocket
const connect = () => {
  if (!props.module || !props.callsign) {
    console.warn('Cannot connect: module or callsign missing')
    logDiagnostic('connect_abort', { reason: 'missing_credentials' })
    return
  }

  // Enable reconnection for this session
  shouldReconnect.value = true

  // Clear any pending reconnect
  if (reconnectTimeout) {
    clearTimeout(reconnectTimeout)
    reconnectTimeout = null
  }

  try {
    logDiagnostic('ws_connect_attempt', { 
      url: props.websocketUrl, 
      module: props.module,
      callsign: props.callsign
    })
    
    ws.value = new WebSocket(props.websocketUrl)
    
    ws.value.onopen = () => {
      console.log('WebSocket connected')
      isConnected.value = true
      reconnectAttempts.value = 0 // Reset reconnect counter on successful connection
      
      logDiagnostic('ws_connected', { 
        module: props.module,
        callsign: props.callsign
      })
      
      // Set Media Session to playing when connected
      setMediaSessionState('playing')
      
      // Request Wake Lock to keep screen on
      requestWakeLock()
      
      // Send voice_start message
      const startMsg = {
        type: 'voice_start',
        module: props.module,
        callsign: props.callsign
      }
      ws.value?.send(JSON.stringify(startMsg))
      logDiagnostic('ws_voice_start_sent', startMsg)
    }
    
    ws.value.onmessage = async (event) => {
      try {
        const data = JSON.parse(event.data)
        
        switch (data.type) {
          case 'audio_data':
            isReceivingAudio.value = true
            
            // Update Media Session with active talker
            if (data.from && data.from !== activeTalker.value) {
              updateMediaSessionMetadata(data.from)
            }
            
            await handleAudioData(data)
            break
          case 'voice_state':
            currentState.value = data.state
            
            // Update receiving flag based on state
            if (data.state === 'rx_busy') {
              isReceivingAudio.value = true
            } else if (data.state === 'listening') {
              isReceivingAudio.value = false
              // Clear active talker when returning to listening
              updateMediaSessionMetadata(null)
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
    
    ws.value.onclose = (event) => {
      console.log('WebSocket disconnected', event.code, event.reason)
      isConnected.value = false
      currentState.value = 'disconnected'
      isReceivingAudio.value = false
      emit('stateChange', 'disconnected')
      
      // Set Media Session to paused when disconnected
      setMediaSessionState('paused')
      updateMediaSessionMetadata(null)
      
      // Release Wake Lock when disconnected
      releaseWakeLock()
      
      // Attempt reconnection if appropriate
      if (shouldReconnect.value && !event.wasClean && reconnectAttempts.value < maxReconnectAttempts) {
        attemptReconnect()
      }
    }
  } catch (error) {
    console.error('Failed to connect WebSocket:', error)
    emit('error', `Connection failed: ${error}`)
  }
}

// Attempt to reconnect with exponential backoff
const attemptReconnect = () => {
  if (reconnectTimeout) return // Already attempting
  
  reconnectAttempts.value++
  const delay = reconnectDelay * Math.pow(2, reconnectAttempts.value - 1) // Exponential backoff
  
  console.log(`Attempting reconnect ${reconnectAttempts.value}/${maxReconnectAttempts} in ${delay}ms`)
  emit('error', `Connection lost. Reconnecting in ${delay / 1000}s... (attempt ${reconnectAttempts.value}/${maxReconnectAttempts})`)
  
  reconnectTimeout = window.setTimeout(() => {
    reconnectTimeout = null
    connect()
  }, delay)
}

// Cancel reconnection attempts
const cancelReconnect = () => {
  if (reconnectTimeout) {
    clearTimeout(reconnectTimeout)
    reconnectTimeout = null
  }
  reconnectAttempts.value = 0
}

// Disconnect from WebSocket
const disconnect = () => {
  shouldReconnect.value = false // Prevent automatic reconnection
  cancelReconnect()
  
  if (ws.value) {
    // Send voice_stop message
    const stopMsg = { type: 'voice_stop' }
    ws.value.send(JSON.stringify(stopMsg))
    
    ws.value.close()
    ws.value = null
  }
  isConnected.value = false
  
  // Set Media Session to none when disconnecting
  setMediaSessionState('none')
  updateMediaSessionMetadata(null)
  
  // Release Wake Lock
  releaseWakeLock()
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
    
    // Create buffer source and connect through analyser for RX level monitoring
    const source = audioContext.value.createBufferSource()
    source.buffer = audioBuffer
    
    if (rxAnalyser.value) {
      source.connect(rxAnalyser.value)
      rxAnalyser.value.connect(audioContext.value.destination)
    } else {
      source.connect(audioContext.value.destination)
    }
    
    source.start()
  } catch (error) {
    console.error('Error playing audio:', error)
  }
}

// Start PTT (Push-to-Talk)
const startPTT = async (password?: string): Promise<boolean> => {
  if (!isConnected.value) {
    emit('error', 'Not connected to server')
    logDiagnostic('ptt_start_failed', { reason: 'not_connected' })
    return false
  }

  // Half-duplex: Check if currently receiving audio
  if (isReceivingAudio.value || currentState.value === 'rx_busy') {
    emit('error', 'Cannot transmit while receiving audio (half-duplex mode)')
    logDiagnostic('ptt_start_failed', { reason: 'rx_busy' })
    return false
  }

  // Request microphone permission if not already granted
  const hasPermission = await requestMicPermission()
  if (!hasPermission) {
    logDiagnostic('ptt_start_failed', { reason: 'no_mic_permission' })
    return false
  }

  if (!opusEncoder.value) {
    emit('error', 'Encoder not initialized')
    logDiagnostic('ptt_start_failed', { reason: 'encoder_not_initialized' })
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
    
    // Start transmit timer
    transmitStartTime = Date.now()
    transmitTimeoutHandle = window.setTimeout(() => {
      console.warn('Max transmit duration reached, stopping PTT')
      emit('error', `Maximum transmit duration (${maxTransmitDuration / 1000}s) reached`)
      logDiagnostic('ptt_timeout', { duration: maxTransmitDuration })
      stopPTT()
    }, maxTransmitDuration)
    
    currentState.value = 'transmitting'
    logDiagnostic('ptt_started', { 
      module: props.module,
      callsign: props.callsign,
      hasPassword: !!password
    })
    console.log('PTT started')
    return true
  } catch (error) {
    console.error('Failed to start PTT:', error)
    emit('error', `Failed to start transmit: ${error}`)
    logDiagnostic('ptt_start_error', { error: String(error) })
    return false
  }
}

// Stop PTT
const stopPTT = () => {
  if (!opusEncoder.value) return

  // Clear transmit timeout
  if (transmitTimeoutHandle) {
    clearTimeout(transmitTimeoutHandle)
    transmitTimeoutHandle = null
  }

  // Log transmit duration
  let duration = 0
  if (transmitStartTime) {
    duration = Date.now() - transmitStartTime
    console.log(`PTT stopped after ${(duration / 1000).toFixed(1)}s`)
    transmitStartTime = null
  }

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
    logDiagnostic('ptt_stopped', { 
      duration,
      module: props.module,
      callsign: props.callsign
    })
    console.log('PTT stopped')
  } catch (error) {
    console.error('Failed to stop PTT:', error)
    logDiagnostic('ptt_stop_error', { error: String(error) })
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
  
  // Re-acquire Wake Lock when page becomes visible (important for mobile)
  document.addEventListener('visibilitychange', async () => {
    if (document.visibilityState === 'visible' && isConnected.value) {
      // Wake Lock is automatically released when page becomes hidden
      // Re-acquire it when page becomes visible again
      await requestWakeLock()
    }
  })
})

onUnmounted(() => {
  stopLevelMonitoring()
  cancelReconnect()
  
  // Clear transmit timeout
  if (transmitTimeoutHandle) {
    clearTimeout(transmitTimeoutHandle)
    transmitTimeoutHandle = null
  }
  
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
  
  // Clear Media Session
  setMediaSessionState('none')
  
  // Release Wake Lock
  releaseWakeLock()
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
  isReceivingAudio,
  rxLevel,
  txLevel,
  getDiagnosticLog,
  diagnosticLog,
  activeTalker,
  mediaSessionSupported,
  wakeLockSupported
})
</script>

<template>
  <!-- This is a headless component - no UI -->
  <div style="display: none;"></div>
</template>
