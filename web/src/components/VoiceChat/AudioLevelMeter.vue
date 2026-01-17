<script setup lang="ts">
import { computed } from 'vue'

// Props
const props = defineProps<{
  level: number  // 0-100
  label: string  // "RX" or "TX"
  color?: string // Optional color override
}>()

// Computed
const fillWidth = computed(() => `${Math.min(100, Math.max(0, props.level))}%`)
const colorClass = computed(() => {
  if (props.color) return props.color
  
  // Default color based on level
  if (props.level > 85) return 'bg-red-500'
  if (props.level > 70) return 'bg-yellow-500'
  return 'bg-green-500'
})
</script>

<template>
  <div class="flex items-center gap-2">
    <span class="text-xs font-medium text-gray-700 dark:text-gray-300 w-6">
      {{ label }}
    </span>
    <div class="flex-1 h-3 bg-gray-200 dark:bg-gray-700 rounded-full overflow-hidden">
      <div 
        :class="['h-full transition-all duration-75', colorClass]"
        :style="{ width: fillWidth }"
      />
    </div>
    <span class="text-xs text-gray-500 dark:text-gray-400 w-8 text-right tabular-nums">
      {{ Math.round(level) }}
    </span>
  </div>
</template>
