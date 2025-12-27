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

    const sidebarOpen = ref(window.innerWidth > 1280) // Default open on XL screens

    // Config State
    interface AppConfig {
        version: string
        commit: string
        date: string
        reflector: {
            name: string
            description: string
            modules?: Record<string, string>
        }
    }

    const config = ref<AppConfig>({
        version: 'dev',
        commit: 'none',
        date: 'unknown',
        reflector: {
            name: 'URFD Dashboard',
            description: 'Universal Reflector'
        }
    })

    const fetchConfig = async () => {
        try {
            const res = await fetch('/api/config')
            if (res.ok) {
                config.value = await res.json()
                document.title = config.value.reflector.name
            }
        } catch (e) {
            console.error("Failed to fetch config", e)
        }
    }

    const toggleSidebar = () => {
        sidebarOpen.value = !sidebarOpen.value
    }

    return { mode, isDark, toggleMode, sidebarOpen, toggleSidebar, config, fetchConfig }
})
