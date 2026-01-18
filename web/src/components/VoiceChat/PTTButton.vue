<script setup lang="ts">
import { computed } from 'vue'
import { Mic, MicOff } from 'lucide-vue-next'

// Props
const props = defineProps<{
  disabled?: boolean
  transmitting?: boolean
}>()

// Emits
const emit = defineEmits<{
  toggle: []
}>()

// Computed
const buttonClass = computed(() => {
  const baseClass = 'flex items-center justify-center w-16 h-16 rounded-full transition-all duration-150 focus:outline-none focus:ring-2 focus:ring-offset-2'
  
  if (props.disabled) {
    return `${baseClass} bg-gray-300 cursor-not-allowed`
  }
  
  if (props.transmitting) {
    return `${baseClass} bg-red-600 hover:bg-red-700 text-white shadow-lg scale-95 focus:ring-red-500`
  }
  
  return `${baseClass} bg-blue-600 hover:bg-blue-700 text-white shadow-md active:scale-95 focus:ring-blue-500`
})

// Handlers
const handleClick = () => {
  if (props.disabled) return
  emit('toggle')
}

// Keyboard support (Space to toggle)
const handleKeyDown = (event: KeyboardEvent) => {
  if (props.disabled) return
  if (event.code === 'Space') {
    event.preventDefault()
    emit('toggle')
  }
}
</script>

<template>
  <button
    :class="buttonClass"
    :disabled="disabled"
    @click="handleClick"
    @keydown="handleKeyDown"
    aria-label="Toggle transmit"
  >
    <Mic v-if="!transmitting" :size="28" />
    <MicOff v-else :size="28" />
  </button>
</template>
