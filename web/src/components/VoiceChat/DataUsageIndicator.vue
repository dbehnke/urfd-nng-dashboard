<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { Activity } from 'lucide-vue-next'

// Props
const props = defineProps<{
  getDataUsage: () => {
    bytesReceived: number
    bytesSent: number
    totalBytes: number
    totalKB: number
    totalMB: number
    duration: number
    rateKbps: number
  }
}>()

// State
const usage = ref({
  bytesReceived: 0,
  bytesSent: 0,
  totalBytes: 0,
  totalKB: 0,
  totalMB: 0,
  duration: 0,
  rateKbps: 0
})

let updateInterval: number | null = null

// Computed
const displaySize = computed(() => {
  const totalMB = usage.value?.totalMB ?? 0
  const totalKB = usage.value?.totalKB ?? 0
  
  if (totalMB >= 1) {
    return `${totalMB.toFixed(2)} MB`
  }
  return `${totalKB.toFixed(2)} KB`
})

const displayRate = computed(() => {
  const rateKbps = usage.value?.rateKbps ?? 0
  return `${rateKbps.toFixed(1)} kbps`
})

const displayDuration = computed(() => {
  const duration = usage.value?.duration ?? 0
  const minutes = Math.floor(duration / 60)
  const seconds = duration % 60
  if (minutes > 0) {
    return `${minutes}m ${seconds}s`
  }
  return `${seconds}s`
})

// Methods
const updateUsage = () => {
  const newUsage = props.getDataUsage()
  if (newUsage) {
    usage.value = {
      bytesReceived: newUsage.bytesReceived ?? 0,
      bytesSent: newUsage.bytesSent ?? 0,
      totalBytes: newUsage.totalBytes ?? 0,
      totalKB: newUsage.totalKB ?? 0,
      totalMB: newUsage.totalMB ?? 0,
      duration: newUsage.duration ?? 0,
      rateKbps: newUsage.rateKbps ?? 0
    }
  }
}

// Lifecycle
onMounted(() => {
  // Update every 2 seconds
  updateInterval = window.setInterval(updateUsage, 2000)
})

onUnmounted(() => {
  if (updateInterval) {
    clearInterval(updateInterval)
  }
})
</script>

<template>
  <div class="flex items-center gap-3 px-3 py-2 bg-gray-50 dark:bg-gray-900/50 rounded border border-gray-200 dark:border-gray-700">
    <Activity :size="16" class="text-gray-600 dark:text-gray-400 flex-shrink-0" />
    
    <div class="flex-1 grid grid-cols-3 gap-2 text-xs">
      <div class="flex flex-col">
        <span class="text-gray-500 dark:text-gray-400">Data</span>
        <span class="font-medium text-gray-900 dark:text-gray-100">{{ displaySize }}</span>
      </div>
      
      <div class="flex flex-col">
        <span class="text-gray-500 dark:text-gray-400">Rate</span>
        <span class="font-medium text-gray-900 dark:text-gray-100">{{ displayRate }}</span>
      </div>
      
      <div class="flex flex-col">
        <span class="text-gray-500 dark:text-gray-400">Time</span>
        <span class="font-medium text-gray-900 dark:text-gray-100">{{ displayDuration }}</span>
      </div>
    </div>
  </div>
</template>
