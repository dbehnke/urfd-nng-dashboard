// Type definitions are now provided by the opus-decoder package itself
// The opus-decoder package includes its own types.d.ts file

declare module 'opus-recorder' {
  export interface RecorderConfig {
    encoderPath?: string
    encoderSampleRate?: number
    encoderApplication?: number
    streamPages?: boolean
    numberOfChannels?: number
    encoderComplexity?: number
    encoderBitRate?: number
    encoderFrameSize?: number
    sourceNode?: MediaStream
  }
  
  export default class Recorder {
    constructor(config: RecorderConfig)
    ondataavailable: (data: Uint8Array) => void
    start(): void
    stop(): void
  }
}
