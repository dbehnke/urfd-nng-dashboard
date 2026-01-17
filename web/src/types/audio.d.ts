declare module 'libopus.js' {
  export default function (): Promise<any>
}

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
