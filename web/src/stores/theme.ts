import { defineStore } from 'pinia'
import { ref, watchEffect } from 'vue'

export const useThemeStore = defineStore('theme', () => {
    const mode = ref<'light' | 'dark' | 'system'>(
        (localStorage.getItem('theme-mode') as any) || 'system'
    )

    const isDark = ref(false)

    const updateTheme = () => {
        const root = window.document.documentElement
        let dark = false

        if (mode.value === 'system') {
            dark = window.matchMedia('(prefers-color-scheme: dark)').matches
        } else {
            dark = mode.value === 'dark'
        }

        isDark.value = dark
        if (dark) {
            root.classList.add('dark')
        } else {
            root.classList.remove('dark')
        }

        localStorage.setItem('theme-mode', mode.value)
    }

    watchEffect(() => {
        updateTheme()
    })

    // Listen for system changes
    window.matchMedia('(prefers-color-scheme: dark)').addEventListener('change', () => {
        if (mode.value === 'system') updateTheme()
    })

    const toggleMode = () => {
        if (mode.value === 'light') mode.value = 'dark'
        else if (mode.value === 'dark') mode.value = 'system'
        else mode.value = 'light'
    }

    return { mode, isDark, toggleMode }
})
