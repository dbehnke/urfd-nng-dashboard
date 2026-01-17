<script setup lang="ts">
import { ref, computed } from 'vue'
import { Mic, MicOff } from 'lucide-vue-next'

// Props
const props = defineProps<{
  disabled?: boolean
  transmitting?: boolean
}>()

// Emits
const emit = defineEmits<{
  pttDown: []
  pttUp: []
}>()

// State
const isPressed = ref(false)

// Computed
const buttonClass = computed(() => {
  const baseClass = 'flex items-center justify-center w-16 h-16 rounded-full transition-all duration-150 focus:outline-none focus:ring-2 focus:ring-offset-2'
  
  if (props.disabled) {
    return `${baseClass} bg-gray-300 cursor-not-allowed`
  }
  
  if (isPressed.value || props.transmitting) {
    return `${baseClass} bg-red-600 hover:bg-red-700 text-white shadow-lg scale-95 focus:ring-red-500`
  }
  
  return `${baseClass} bg-blue-600 hover:bg-blue-700 text-white shadow-md active:scale-95 focus:ring-blue-500`
})

// Handlers
const handleMouseDown = () => {
  if (props.disabled) return
  isPressed.value = true
  emit('pttDown')
}

const handleMouseUp = () => {
  if (props.disabled) return
  isPressed.value = false
  emit('pttUp')
}

const handleMouseLeave = () => {
  // Release PTT if mouse leaves button while pressed
  if (isPressed.value) {
    handleMouseUp()
  }
}

// Keyboard support
const handleKeyDown = (event: KeyboardEvent) => {
  if (props.disabled) return
  if (event.code === 'Space' && !isPressed.value) {
    event.preventDefault()
    isPressed.value = true
    emit('pttDown')
  }
}

const handleKeyUp = (event: KeyboardEvent) => {
  if (props.disabled) return
  if (event.code === 'Space' && isPressed.value) {
    event.preventDefault()
    isPressed.value = false
    emit('pttUp')
  }
}
</script>

<template>
  <button
    :class="buttonClass"
    :disabled="disabled"
    @mousedown="handleMouseDown"
    @mouseup="handleMouseUp"
    @mouseleave="handleMouseLeave"
    @touchstart.prevent="handleMouseDown"
    @touchend.prevent="handleMouseUp"
    @keydown="handleKeyDown"
    @keyup="handleKeyUp"
    aria-label="Push to talk"
  >
    <Mic v-if="!isPressed && !transmitting" :size="28" />
    <MicOff v-else :size="28" />
  </button>
</template>
