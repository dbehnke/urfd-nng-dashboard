<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'
import { useVoiceStore } from '@/stores/voice'

// Props
const props = defineProps<{
  websocketUrl: string
  module: string | null
  callsign: string
  isTransmitting: boolean
}>()

// Emits
const emit = defineEmits<{
  stateChange: [state: 'listening' | 'transmitting' | 'rx_busy' | 'disconnected' | 'ptt_requesting' | 'ptt_releasing']
  error: [message: string]
}>()

// State
const voiceStore = useVoiceStore()
const ws = ref<WebSocket | null>(null)
const sessionId = ref<string>('') // Our session ID for echo prevention
const audioContext = ref<AudioContext | null>(null)
const opusDecoder = ref<any>(null)
const opusEncoder = ref<any>(null)
let opusDecoderReady = false
const mediaStream = ref<MediaStream | null>(null)
const micPermissionGranted = ref(false)
const micPermissionDenied = ref(false)
const isConnected = ref(false)
const currentState = ref<'listening' | 'transmitting' | 'rx_busy' | 'disconnected' | 'ptt_requesting' | 'ptt_releasing'>('disconnected')
const isReceivingAudio = ref(false)

// Audio level monitoring
const rxAnalyser = ref<AnalyserNode | null>(null)
const rxGainNode = ref<GainNode | null>(null) // User-adjustable gain control
const txAnalyser = ref<AnalyserNode | null>(null)
const rxLevel = ref(0)
const txLevel = ref(0)
let levelMonitorInterval: number | null = null

// Audio playback scheduling for continuous playback
let audioPlaybackTime = 0
let lastAudioReceiveTime = 0
let audioTimeoutHandle: number | null = null
const audioTimeout = 500 // Clear receiving state after 500ms of no audio

// Connection recovery
const reconnectAttempts = ref(0)
const maxReconnectAttempts = 5
const reconnectDelay = 2000 // Start with 2 seconds
let reconnectTimeout: number | null = null
const shouldReconnect = ref(true)

// Session timeout enforcement
const maxTransmitDuration = ref(180000) // Default 180 seconds in milliseconds
let transmitStartTime: number | null = null
let transmitTimeoutHandle: number | null = null
let pttStopping = false // Flag to indicate PTT is in the process of stopping

// PTT request timeout
const PTT_REQUEST_TIMEOUT = 2000 // 2 seconds
let pttRequestTimeoutHandle: number | null = null
const PTT_RELEASE_TIMEOUT = 2000 // 2 seconds
let pttReleaseTimeoutHandle: number | null = null

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

// Data usage tracking
const bytesReceived = ref(0)
const bytesSent = ref(0)
const sessionStartTime = ref<number | null>(null)

const getDataUsage = () => {
  // Add null safety in case this is called after component unmount
  const received = bytesReceived.value ?? 0
  const sent = bytesSent.value ?? 0
  const totalBytes = received + sent
  const totalKB = (totalBytes / 1024).toFixed(2)
  const totalMB = (totalBytes / (1024 * 1024)).toFixed(2)
  
  const startTime = sessionStartTime.value ?? 0
  const duration = startTime ? (Date.now() - startTime) / 1000 : 0
  
  const rateKbps = duration > 0 
    ? ((totalBytes * 8) / (duration * 1000)).toFixed(2)
    : '0'
  
  return {
    bytesReceived: received,
    bytesSent: sent,
    totalBytes,
    totalKB: parseFloat(totalKB),
    totalMB: parseFloat(totalMB),
    duration: Math.round(duration),
    rateKbps: parseFloat(rateKbps)
  }
}

const resetDataUsage = () => {
  bytesReceived.value = 0
  bytesSent.value = 0
  sessionStartTime.value = null
}

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

// Resume AudioContext (required for iOS Safari)
const resumeAudioContext = async () => {
  if (!audioContext.value) return
  
  try {
    if (audioContext.value.state === 'suspended') {
      await audioContext.value.resume()
      logDiagnostic('audio_context_resumed', {
        state: audioContext.value.state
      })
      console.log('AudioContext resumed from suspended state')
    }
  } catch (error) {
    console.error('Failed to resume AudioContext:', error)
    logDiagnostic('audio_context_resume_error', { error: String(error) })
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
    
    // Create gain node for user-adjustable receive volume
    // Default: 100% = 1.0 gain (no change)
    rxGainNode.value = audioContext.value.createGain()
    rxGainNode.value.gain.value = voiceStore.receiveGain / 100
    
    // Connect the audio chain once during initialization
    // Chain: [buffer sources] -> gainNode -> analyser -> destination
    rxGainNode.value.connect(rxAnalyser.value)
    rxAnalyser.value.connect(audioContext.value.destination)
    
    // Initialize Opus decoder (dynamically import)
    const { OpusDecoder } = await import('opus-decoder')
    opusDecoder.value = new OpusDecoder({
      sampleRate: 8000,
      channels: 1
    })
    // Wait for WASM to be ready
    await opusDecoder.value.ready
    opusDecoderReady = true
    
    // Start audio level monitoring
    startLevelMonitoring()
    
    // Initialize Media Session API for background playback
    initMediaSession()
    
    logDiagnostic('audio_init_success', {
      sampleRate: audioContext.value.sampleRate,
      state: audioContext.value.state,
      receiveGain: voiceStore.receiveGain
    })
    
    console.log('Audio engine initialized', {
      sampleRate: audioContext.value.sampleRate,
      state: audioContext.value.state,
      receiveGain: voiceStore.receiveGain
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

// Extract raw Opus packets from Ogg container
// Ogg page parser - uses segment table to split packets correctly
const extractOpusFromOgg = (oggData: Uint8Array): Uint8Array[] => {
  const packets: Uint8Array[] = []
  let offset = 0
  
  while (offset < oggData.length) {
    // Check for "OggS" magic number
    if (offset + 27 > oggData.length) break
    if (oggData[offset] !== 0x4F || oggData[offset + 1] !== 0x67 || 
        oggData[offset + 2] !== 0x67 || oggData[offset + 3] !== 0x53) {
      console.warn('[VoiceEngine] Invalid Ogg page at offset', offset)
      break
    }
    
    // Read number of page segments (at offset 26)
    const numSegments = oggData[offset + 26]
    if (!numSegments || offset + 27 + numSegments > oggData.length) break
    
    // Read segment table - each segment describes packet boundaries
    const segmentTable = []
    for (let i = 0; i < numSegments; i++) {
      const segmentSize = oggData[offset + 27 + i]
      if (segmentSize === undefined) break
      segmentTable.push(segmentSize)
    }
    
    // Extract the payload (skip header + segment table)
    const payloadStart = offset + 27 + numSegments
    const totalPayloadSize = segmentTable.reduce((sum, size) => sum + size, 0)
    if (payloadStart + totalPayloadSize > oggData.length) break
    
    // Use segment table to split payload into individual packets
    let payloadOffset = payloadStart
    let currentPacket: number[] = []
    
    for (const segmentSize of segmentTable) {
      // Add this segment to current packet
      const segment = oggData.slice(payloadOffset, payloadOffset + segmentSize)
      currentPacket.push(...segment)
      payloadOffset += segmentSize
      
      // If segment is < 255, packet is complete
      if (segmentSize < 255) {
        const packetData = new Uint8Array(currentPacket)
        
        // Skip OpusHead and OpusTags pages
        if (packetData.length > 8) {
          const header = String.fromCharCode(...Array.from(packetData.slice(0, 8)))
          if (!header.startsWith('OpusHead') && !header.startsWith('OpusTags')) {
            // This is actual audio data
            packets.push(packetData)
          }
        }
        
        // Start new packet
        currentPacket = []
      }
    }
    
    // Move to next page
    offset = payloadStart + totalPayloadSize
  }
  
  return packets
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
    // Using Ogg container with minimal batching for real-time transmission
    // maxFramesPerPage=1 ensures each 20ms frame is sent immediately
    // Extract config to bypass TypeScript checking for maxFramesPerPage (exists in opus-recorder but not in types)
    const recorderConfig: any = {
      encoderPath: '/opus-recorder/encoderWorker.min.js',
      encoderSampleRate: 8000,
      encoderApplication: 2048, // VOIP application  
      streamPages: true, // Use Ogg container for compatibility
      maxFramesPerPage: 1, // Send immediately (1 frame = 20ms, well below 80ms AllStar timeout)
      numberOfChannels: 1,
      encoderComplexity: 10,
      encoderBitRate: 12000, // 12kbps as per spec
      encoderFrameSize: 20, // 20ms frames
      sourceNode: mediaStream.value
    }
    opusEncoder.value = new Recorder(recorderConfig)

    // Handle encoded data - extract raw Opus packets from Ogg pages
    opusEncoder.value.ondataavailable = (typedArray: Uint8Array) => {
      console.log('[VoiceEngine] Ogg page received, size:', typedArray.length, 'bytes')
      // Extract Opus packets from Ogg container
      const opusPackets = extractOpusFromOgg(typedArray)
      for (const packet of opusPackets) {
        console.log('[VoiceEngine] Extracted Opus packet, size:', packet.length, 'bytes')
        sendAudioData(packet)
      }
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
const connect = async () => {
  if (!props.module || !props.callsign) {
    console.warn('Cannot connect: module or callsign missing')
    logDiagnostic('connect_abort', { reason: 'missing_credentials' })
    return
  }

  // Enable reconnection for this session
  shouldReconnect.value = true
  
  // Generate unique session ID for echo prevention
  sessionId.value = `web-${Date.now()}-${Math.random().toString(36).substring(7)}`
  console.log('[VoiceEngine] Generated session ID:', sessionId.value)

  // Clear any pending reconnect
  if (reconnectTimeout) {
    clearTimeout(reconnectTimeout)
    reconnectTimeout = null
  }

  try {
    // Resume AudioContext if suspended (iOS requirement)
    await resumeAudioContext()
    
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
      
      // Reset data usage counters and start tracking
      resetDataUsage()
      sessionStartTime.value = Date.now()
      
      logDiagnostic('ws_connected', { 
        module: props.module,
        callsign: props.callsign
      })
      
      // Set Media Session to playing when connected
      setMediaSessionState('playing')
      
      // Request Wake Lock to keep screen on
      requestWakeLock()
      
      // Send voice_start message with session ID
      const startMsg = {
        type: 'voice_start',
        module: props.module,
        callsign: props.callsign,
        session_id: sessionId.value  // Include session ID for echo prevention
      }
      ws.value?.send(JSON.stringify(startMsg))
      
      // Track sent bytes
      const msgSize = JSON.stringify(startMsg).length
      bytesSent.value += msgSize
      
      logDiagnostic('ws_voice_start_sent', startMsg)
    }
    
    ws.value.onmessage = async (event) => {
      try {
        // Track received bytes
        const msgSize = typeof event.data === 'string' 
          ? event.data.length 
          : event.data.size || 0
        bytesReceived.value += msgSize
        
        const data = JSON.parse(event.data)
        
        switch (data.type) {
          case 'voice_config':
            // Receive and store config from server
            if (data.max_tx_duration) {
              maxTransmitDuration.value = data.max_tx_duration * 1000 // Convert seconds to ms
              console.log('Received voice config: max_tx_duration =', data.max_tx_duration, 'seconds')
            }
            // Also handle state if included
            if (data.state) {
              currentState.value = data.state
              emit('stateChange', data.state)
            }
            break
          case 'audio_data':
          case 'peer_audio':
            // Handle both reflector audio (audio_data) and peer-to-peer audio (peer_audio)
            console.log('[VoiceEngine] Received audio:', {
              type: data.type,
              from: data.from,
              fromSessionId: data.from_session_id,
              mySessionId: sessionId.value,
              isOwnSession: data.from_session_id === sessionId.value
            })
            
            // ✅ CRITICAL: If BOTH from and fromSessionId are missing, this is likely echo
            // The reflector is sending back audio without proper sender info
            if (!data.from && !data.from_session_id) {
              console.warn('[VoiceEngine] ⚠️ BLOCKING audio with no sender info (likely reflector echo)')
              break
            }
            
            // ✅ PRIMARY FILTER: Skip if this is our own session
            if (data.from_session_id && data.from_session_id === sessionId.value) {
              console.log('[VoiceEngine] ✅ Skipping own audio (session:', data.from_session_id, ')')
              break
            }
            
            // FALLBACK: Also check callsign for older messages or reflector audio
            if (data.from && data.from === props.callsign) {
              console.log('[VoiceEngine] ✅ Skipping own audio via callsign (from:', data.from, ')')
              break
            }
            
            // DEFENSIVE: If no session ID but we're transmitting, skip it
            if (!data.from_session_id && currentState.value === 'transmitting') {
              console.warn('[VoiceEngine] ⚠️ Skipping audio without session ID while transmitting (likely echo)')
              break
            }
            
            isReceivingAudio.value = true
            
            // Update Media Session with active talker
            if (data.from && data.from !== activeTalker.value) {
              updateMediaSessionMetadata(data.from)
            }
            
            await handleAudioData(data)
            break
          case 'voice_state':
            // Clear PTT request timeout if active
            if (pttRequestTimeoutHandle) {
              clearTimeout(pttRequestTimeoutHandle)
              pttRequestTimeoutHandle = null
            }
            
            // Clear PTT release timeout if active
            if (pttReleaseTimeoutHandle) {
              clearTimeout(pttReleaseTimeoutHandle)
              pttReleaseTimeoutHandle = null
            }
            
            const newState = data.state
            const oldState = currentState.value
            currentState.value = newState
            
            // ✅ START encoder when server grants PTT
            if (newState === 'transmitting' && oldState === 'ptt_requesting') {
              if (opusEncoder.value) {
                console.log('[VoiceEngine] Server granted PTT, starting encoder')
                // Start with 20ms timeSlice to match frame size and prevent AllStar 80ms timeout
                opusEncoder.value.start(20)
                
                // Start transmit timer
                transmitStartTime = Date.now()
                transmitTimeoutHandle = window.setTimeout(() => {
                  console.warn('Max transmit duration reached, stopping PTT')
                  emit('error', `Maximum transmit duration (${maxTransmitDuration.value / 1000}s) reached`)
                  logDiagnostic('ptt_timeout', { duration: maxTransmitDuration.value })
                  stopPTT()
                }, maxTransmitDuration.value)
                
                logDiagnostic('ptt_granted', { 
                  module: props.module,
                  callsign: props.callsign
                })
              }
            }
            
            // ✅ STOP encoder when server confirms release
            if (newState === 'listening' && oldState === 'ptt_releasing') {
              if (opusEncoder.value) {
                try {
                  console.log('[VoiceEngine] Server confirmed release, stopping encoder')
                  opusEncoder.value.stop()
                } catch (e) {
                  console.error('Error stopping encoder:', e)
                }
              }
            }
            
            // Update receiving flag based on state
            if (newState === 'rx_busy') {
              isReceivingAudio.value = true
            } else if (newState === 'listening') {
              isReceivingAudio.value = false
              // Clear active talker when returning to listening
              updateMediaSessionMetadata(null)
            }
            
            emit('stateChange', newState)
            break
          case 'ptt_denied':
            // Clear timeout
            if (pttRequestTimeoutHandle) {
              clearTimeout(pttRequestTimeoutHandle)
              pttRequestTimeoutHandle = null
            }
            
            // Parse active talker from reason string
            // Format: "PTT denied - KC1XXX is transmitting"
            let deniedByCallsign = 'unknown'
            const match = data.reason?.match(/PTT denied - (.+?) is transmitting/)
            if (match) {
              deniedByCallsign = match[1]
            }
            
            // Update media session to show who's talking
            updateMediaSessionMetadata(deniedByCallsign)
            
            // Show error to user
            console.warn('PTT denied:', data.reason)
            emit('error', `Module busy - ${deniedByCallsign} is transmitting`)
            
            // ✅ Reset state to listening if we were requesting
            if (currentState.value === 'ptt_requesting') {
              currentState.value = 'listening'
              emit('stateChange', 'listening')
            }
            
            logDiagnostic('ptt_denied', { 
              reason: data.reason, 
              active_talker: deniedByCallsign 
            })
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
      
      // Clear audio timeout and reset playback time
      if (audioTimeoutHandle) {
        clearTimeout(audioTimeoutHandle)
        audioTimeoutHandle = null
      }
      audioPlaybackTime = 0
      
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
  
  // ✅ Clear PTT request timeout
  if (pttRequestTimeoutHandle) {
    clearTimeout(pttRequestTimeoutHandle)
    pttRequestTimeoutHandle = null
  }
  
  // ✅ Clear PTT release timeout
  if (pttReleaseTimeoutHandle) {
    clearTimeout(pttReleaseTimeoutHandle)
    pttReleaseTimeoutHandle = null
  }
  
  // Clear audio timeout
  if (audioTimeoutHandle) {
    clearTimeout(audioTimeoutHandle)
    audioTimeoutHandle = null
  }
  
  if (ws.value) {
    // Send voice_stop message
    const stopMsg = { type: 'voice_stop' }
    ws.value.send(JSON.stringify(stopMsg))
    
    ws.value.close()
    ws.value = null
  }
  isConnected.value = false
  
  // Reset audio state
  isReceivingAudio.value = false
  audioPlaybackTime = 0
  
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
  if (!opusDecoder.value || !opusDecoderReady) return null
  
  try {
    // Use the new opus-decoder API
    // The decoder is configured for 8kHz mono in initAudio()
    const { channelData } = opusDecoder.value.decodeFrame(opusData)
    
    // channelData is an array of Float32Arrays (one per channel)
    // Since we're using mono (1 channel), we return the first channel
    if (channelData && channelData.length > 0) {
      return channelData[0]
    }
    
    return null
  } catch (error) {
    console.error('Opus decode error:', error)
    return null
  }
}

// Play PCM audio through Web Audio API with scheduled playback for continuous audio
const playAudio = (pcmData: Float32Array) => {
  if (!audioContext.value) return
  
  try {
    const currentTime = audioContext.value.currentTime
    
    // Initialize playback time on first packet or if we fell behind
    if (audioPlaybackTime === 0 || audioPlaybackTime < currentTime) {
      audioPlaybackTime = currentTime
    }
    
    // Update gain from voice store (allows dynamic gain adjustment)
    if (rxGainNode.value) {
      rxGainNode.value.gain.value = voiceStore.receiveGain / 100
    }
    
    // Create audio buffer at 8000 Hz (the actual sample rate of the PCM data)
    // The browser will automatically resample to the AudioContext's sample rate during playback
    const audioBuffer = audioContext.value.createBuffer(
      1, // mono
      pcmData.length,
      8000 // Opus decoder output is always 8000 Hz
    )
    
    // Copy PCM data to buffer
    audioBuffer.getChannelData(0).set(pcmData)
    
    // Create buffer source and connect to the audio chain
    // Audio chain (connected once during init): source -> gainNode -> analyser -> destination
    const source = audioContext.value.createBufferSource()
    source.buffer = audioBuffer
    
    // Connect this buffer source to the gain node (or fallback to analyser/destination)
    if (rxGainNode.value) {
      source.connect(rxGainNode.value)
    } else if (rxAnalyser.value) {
      source.connect(rxAnalyser.value)
    } else {
      source.connect(audioContext.value.destination)
    }
    
    // Schedule playback at the next scheduled time for continuous audio
    source.start(audioPlaybackTime)
    
    // Advance playback time by the duration of this buffer
    audioPlaybackTime += audioBuffer.duration
    
    // Update last audio receive time and reset timeout
    lastAudioReceiveTime = Date.now()
    resetAudioTimeout()
    
    // Auto-cleanup when audio finishes
    source.onended = () => {
      // Check if we should clear receiving state (no audio for audioTimeout ms)
      checkAudioTimeout()
    }
  } catch (error) {
    console.error('Error playing audio:', error)
  }
}

// Reset audio timeout - clears receiving state if no audio received
const resetAudioTimeout = () => {
  if (audioTimeoutHandle) {
    clearTimeout(audioTimeoutHandle)
  }
  audioTimeoutHandle = window.setTimeout(() => {
    checkAudioTimeout()
  }, audioTimeout)
}

// Check if we should clear receiving state
const checkAudioTimeout = () => {
  const timeSinceLastAudio = Date.now() - lastAudioReceiveTime
  if (timeSinceLastAudio >= audioTimeout && isReceivingAudio.value) {
    console.log('[VoiceEngine] Audio timeout - clearing receiving state')
    isReceivingAudio.value = false
    audioPlaybackTime = 0 // Reset playback time for next transmission
    
    // Also clear rx_busy state if stuck
    if (currentState.value === 'rx_busy') {
      console.log('[VoiceEngine] Clearing stuck rx_busy state')
      currentState.value = 'listening'
      emit('stateChange', 'listening')
    }
    
    logDiagnostic('audio_timeout', { timeSinceLastAudio })
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
    // Resume AudioContext if suspended (iOS requirement)
    await resumeAudioContext()
    
    // ✅ Set requesting state (don't set transmitting yet!)
    currentState.value = 'ptt_requesting'
    emit('stateChange', 'ptt_requesting')
    
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
    
    // ✅ Start timeout - if server doesn't respond in 2s, assume denied
    pttRequestTimeoutHandle = window.setTimeout(() => {
      if (currentState.value === 'ptt_requesting') {
        currentState.value = 'listening'
        emit('stateChange', 'listening')
        emit('error', 'PTT request timed out - server did not respond')
        logDiagnostic('ptt_request_timeout', {})
      }
    }, PTT_REQUEST_TIMEOUT)
    
    logDiagnostic('ptt_requested', { 
      module: props.module,
      callsign: props.callsign,
      hasPassword: !!password
    })
    
    // ✅ DON'T start encoder here - wait for voice_state: 'transmitting'
    
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
  console.log('[VoiceEngine] stopPTT called, state:', currentState.value)
  
  if (!opusEncoder.value || pttStopping) {
    console.log('[VoiceEngine] No encoder or already stopping, returning early')
    return
  }

  // Only allow stopping if we're actually transmitting
  if (currentState.value !== 'transmitting') {
    console.log('[VoiceEngine] Not transmitting, ignoring stopPTT')
    return
  }

  // Set stopping flag to prevent re-entry
  pttStopping = true

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
    // ✅ IMMEDIATELY send ptt_release (no delay!)
    if (ws.value && ws.value.readyState === WebSocket.OPEN) {
      const pttMsg = {
        type: 'ptt_release',
        module: props.module,
        callsign: props.callsign
      }
      console.log('[VoiceEngine] Sending ptt_release message (immediately)')
      ws.value.send(JSON.stringify(pttMsg))
    } else {
      console.warn('[VoiceEngine] WebSocket not open, cannot send ptt_release')
    }
    
    // ✅ Set releasing state (encoder will be stopped when server confirms)
    currentState.value = 'ptt_releasing'
    pttStopping = false
    console.log('[VoiceEngine] Emitting stateChange: ptt_releasing')
    emit('stateChange', 'ptt_releasing')
    
    // ✅ Start timeout - if server doesn't respond in 2s, force stop
    pttReleaseTimeoutHandle = window.setTimeout(() => {
      if (currentState.value === 'ptt_releasing') {
        console.warn('[VoiceEngine] PTT release timeout - forcing stop')
        if (opusEncoder.value) {
          try {
            opusEncoder.value.stop()
          } catch (e) {
            console.error('Error stopping encoder on timeout:', e)
          }
        }
        currentState.value = 'listening'
        emit('stateChange', 'listening')
        logDiagnostic('ptt_release_timeout', {})
      }
    }, PTT_RELEASE_TIMEOUT)
    
    logDiagnostic('ptt_stopped', { 
      duration,
      module: props.module,
      callsign: props.callsign
    })
    
    // Server will send voice_state: 'listening' to confirm release
    // Encoder will be stopped in the voice_state handler
    
  } catch (error) {
    pttStopping = false
    console.error('Failed to stop PTT:', error)
    logDiagnostic('ptt_stop_error', { error: String(error) })
  }
}

// Cancel PTT request (if user clicks button again while waiting for server)
const cancelPTTRequest = () => {
  console.log('[VoiceEngine] Cancelling PTT request')
  
  // Clear the timeout
  if (pttRequestTimeoutHandle) {
    clearTimeout(pttRequestTimeoutHandle)
    pttRequestTimeoutHandle = null
  }
  
  // Reset state to listening
  if (currentState.value === 'ptt_requesting') {
    currentState.value = 'listening'
    emit('stateChange', 'listening')
    logDiagnostic('ptt_request_cancelled', {})
  }
}

// Send encoded audio data to server
const sendAudioData = (opusData: Uint8Array) => {
  if (!ws.value || ws.value.readyState !== WebSocket.OPEN) {
    console.warn('[VoiceEngine] WebSocket not open, cannot send audio')
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
    
    console.log('[VoiceEngine] Sending audio_data packet, base64 length:', base64.length)
    ws.value.send(JSON.stringify(audioMsg))
    
    // Track sent bytes
    const msgSize = JSON.stringify(audioMsg).length
    bytesSent.value += msgSize
  } catch (error) {
    console.error('Failed to send audio data:', error)
  }
}

// Set receive gain (0-200%)
const setReceiveGain = (gain: number) => {
  if (!rxGainNode.value) return
  
  // Clamp to 0-200% range
  const clampedGain = Math.max(0, Math.min(200, gain))
  
  // Update gain node (convert percentage to linear gain)
  rxGainNode.value.gain.value = clampedGain / 100
  
  console.log(`[VoiceEngine] Receive gain set to ${clampedGain}% (${rxGainNode.value.gain.value.toFixed(2)}x)`)
}

// Lifecycle hooks
onMounted(async () => {
  await initAudio()
  
  // Re-acquire Wake Lock when page becomes visible (important for mobile)
  document.addEventListener('visibilitychange', async () => {
    if (document.visibilityState === 'visible') {
      // Resume AudioContext if suspended (iOS requirement)
      await resumeAudioContext()
      
      if (isConnected.value) {
        // Wake Lock is automatically released when page becomes hidden
        // Re-acquire it when page becomes visible again
        await requestWakeLock()
      } else if (shouldReconnect.value && props.module && props.callsign) {
        // Auto-reconnect if we were connected before going to background
        console.log('App resumed from background, attempting reconnection...')
        logDiagnostic('app_resume_reconnect', {})
        connect()
      }
    }
  })
})

onUnmounted(() => {
  stopLevelMonitoring()
  cancelReconnect()
  
  // Clear audio timeout
  if (audioTimeoutHandle) {
    clearTimeout(audioTimeoutHandle)
    audioTimeoutHandle = null
  }
  
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
  cancelPTTRequest,
  micPermissionGranted,
  micPermissionDenied,
  currentState,
  isReceivingAudio,
  rxLevel,
  txLevel,
  setReceiveGain,
  getDiagnosticLog,
  diagnosticLog,
  activeTalker,
  mediaSessionSupported,
  wakeLockSupported,
  getDataUsage,
  resetDataUsage,
  bytesReceived,
  bytesSent,
  resumeAudioContext,
  maxTransmitDuration,
  transmitStartTime: () => transmitStartTime
})
</script>

<template>
  <!-- This is a headless component - no UI -->
  <div style="display: none;"></div>
</template>
