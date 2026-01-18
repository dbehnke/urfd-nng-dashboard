<script setup lang="ts">
import { ref, watch } from 'vue'
import { Lock, X } from 'lucide-vue-next'

// Props
const props = defineProps<{
  show: boolean
}>()

// Emits
const emit = defineEmits<{
  submit: [password: string]
  cancel: []
}>()

// State
const password = ref('')
const inputRef = ref<HTMLInputElement | null>(null)

// Focus input when dialog opens
watch(() => props.show, (show) => {
  if (show) {
    password.value = ''
    setTimeout(() => {
      inputRef.value?.focus()
    }, 100)
  }
})

// Methods
const handleSubmit = () => {
  if (password.value.length > 0) {
    emit('submit', password.value)
  }
}

const handleCancel = () => {
  password.value = ''
  emit('cancel')
}

const handleKeydown = (event: KeyboardEvent) => {
  if (event.key === 'Escape') {
    handleCancel()
  } else if (event.key === 'Enter') {
    handleSubmit()
  }
}
</script>

<template>
  <Teleport to="body">
    <Transition name="dialog">
      <div
        v-if="show"
        class="fixed inset-0 z-[9999] flex items-center justify-center bg-black/50 backdrop-blur-sm"
        @click.self="handleCancel"
      >
      <div
        class="relative w-full max-w-md mx-4 bg-white dark:bg-gray-800 rounded-lg shadow-xl"
        @keydown="handleKeydown"
      >
        <!-- Header -->
        <div class="flex items-center justify-between p-4 border-b border-gray-200 dark:border-gray-700">
          <div class="flex items-center gap-2">
            <Lock :size="20" class="text-blue-600" />
            <h3 class="text-lg font-semibold text-gray-900 dark:text-gray-100">
              Password Required
            </h3>
          </div>
          <button
            @click="handleCancel"
            class="p-1 hover:bg-gray-100 dark:hover:bg-gray-700 rounded transition-colors"
            aria-label="Close"
          >
            <X :size="20" class="text-gray-500 dark:text-gray-400" />
          </button>
        </div>

        <!-- Body -->
        <div class="p-4">
          <p class="text-sm text-gray-600 dark:text-gray-400 mb-4">
            Enter the transmit password to enable PTT (Push-to-Talk).
          </p>

          <div class="flex flex-col gap-1">
            <label for="password" class="text-sm font-medium text-gray-700 dark:text-gray-300">
              Password
            </label>
            <input
              id="password"
              ref="inputRef"
              v-model="password"
              type="password"
              placeholder="Enter password..."
              class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 focus:outline-none focus:ring-2 focus:ring-blue-500"
              @keyup.enter="handleSubmit"
              @keyup.escape="handleCancel"
            />
          </div>

          <p class="text-xs text-gray-500 dark:text-gray-400 mt-2">
            Your password will be stored in this session for convenience.
          </p>
        </div>

        <!-- Footer -->
        <div class="flex items-center justify-end gap-2 p-4 border-t border-gray-200 dark:border-gray-700">
          <button
            @click="handleCancel"
            class="px-4 py-2 text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700 rounded-md transition-colors"
          >
            Cancel
          </button>
          <button
            @click="handleSubmit"
            :disabled="password.length === 0"
            class="px-4 py-2 bg-blue-600 hover:bg-blue-700 disabled:bg-gray-300 disabled:cursor-not-allowed text-white rounded-md transition-colors"
          >
            Submit
          </button>
        </div>
      </div>
      </div>
    </Transition>
  </Teleport>
</template>

<style scoped>
.dialog-enter-active,
.dialog-leave-active {
  transition: opacity 0.2s ease;
}

.dialog-enter-from,
.dialog-leave-to {
  opacity: 0;
}

.dialog-enter-active > div,
.dialog-leave-active > div {
  transition: transform 0.2s ease;
}

.dialog-enter-from > div,
.dialog-leave-to > div {
  transform: scale(0.95);
}
</style>
