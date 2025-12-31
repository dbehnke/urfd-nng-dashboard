<script setup lang="ts">
import { useThemeStore } from '../stores/theme'
import { computed, ref, onMounted } from 'vue'

import DesktopPlayer from '../components/AudioPlayer/DesktopPlayer.vue'
import { usePlayerStore } from '../stores/player'

const theme = useThemeStore()
const player = usePlayerStore()

const isMobile = ref(false)

const updateLayout = (e: MediaQueryListEvent | MediaQueryList) => {
  const mobile = e.matches
  isMobile.value = mobile
  
  if (mobile) {
      // Transitioned INTO mobile: Close Sidebar
      theme.sidebarOpen = false
  } else {
      // Transitioned OUT of mobile (to Desktop): Open Sidebar
      theme.sidebarOpen = true
  }
}

onMounted(() => {
  const mq = window.matchMedia('(max-width: 1023px)')
  
  // Initial check
  updateLayout(mq)
  
  // Listen for changes
  mq.addEventListener('change', updateLayout)
})

const sidebarClasses = computed(() => {
  // Base classes
  let c = "bg-white dark:bg-slate-900 border-r border-slate-200 dark:border-slate-800 transition-all duration-300 ease-in-out flex flex-col "

  if (isMobile.value) { // Mobile/Tablet
     c += "fixed inset-y-0 left-0 z-50 w-64 shadow-2xl "
     c += theme.sidebarOpen ? "translate-x-0" : "-translate-x-full"
  } else {
     // Desktop
     c += "h-[calc(100vh-4rem)] sticky top-16 " // Stick below header (4rem height)
     c += theme.sidebarOpen ? "w-64" : "w-0 overflow-hidden border-none"
  }
  return c
})

</script>

<template>
  <div class="min-h-screen bg-slate-50 dark:bg-slate-950 text-slate-900 dark:text-slate-100 flex flex-col">
    
    <!-- Header -->
    <header class="h-16 bg-white dark:bg-slate-900 border-b border-slate-200 dark:border-slate-800 flex items-center px-4 justify-between z-40
                   lg:sticky lg:top-0"> <!-- Sticky on desktop only -->
      
      <div class="flex items-center gap-4">
        <!-- Hamburger (Mobile/Desktop toggle) -->
        <button @click="theme.toggleSidebar" 
                class="p-2 rounded-lg hover:bg-slate-100 dark:hover:bg-slate-800 text-slate-600 dark:text-slate-400 focus:outline-none">
          <svg class="w-6 h-6" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 6h16M4 12h16M4 18h16" />
          </svg>
        </button>

        <!-- Title -->
        <div>
          <h1 class="font-bold text-xl tracking-tight text-blue-600 dark:text-blue-400">
            {{ theme.config.reflector.name }}
          </h1>
          <p class="text-xs text-slate-500 font-medium hidden sm:block">
            {{ theme.config.reflector.description }}
          </p>
        </div>
      </div>

      <!-- Right Side (Slots/Theme Toggle) -->
      <div class="flex items-center gap-3">
         <!-- Desktop Player Toggle -->
         <button v-if="!isMobile" 
                 @click="player.toggleUI()"
                 class="hidden sm:flex items-center gap-2 px-3 py-1.5 rounded-lg text-sm font-bold transition-all border"
                 :class="player.isUIOpen ? 'bg-blue-50 border-blue-200 text-blue-600 dark:bg-blue-900/30 dark:border-blue-800 dark:text-blue-400' : 'bg-white border-slate-200 text-slate-700 hover:bg-slate-50 dark:bg-slate-900 dark:border-slate-800 dark:text-slate-300 dark:hover:bg-slate-800'">
             
             <!-- Icon State -->
             <div class="flex items-center gap-2">
                 <span v-if="player.isRecording && !player.isPlaying" class="relative flex h-3 w-3">
                    <span class="animate-ping absolute inline-flex h-full w-full rounded-full bg-red-400 opacity-75"></span>
                    <span class="relative inline-flex rounded-full h-3 w-3 bg-red-500"></span>
                 </span>
                 <svg v-else-if="player.isPlaying" class="w-4 h-4 fill-current animate-pulse" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                     <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10 9v6m4-6v6m7-3a9 9 0 11-18 0 9 9 0 0118 0z" />
                 </svg>
                 <svg v-else class="w-4 h-4" :class="{'rotate-180': player.isUIOpen}" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                     <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7" />
                 </svg>
             </div>

             <!-- Text State -->
             <span v-if="!player.isUIOpen && (player.isPlaying || player.isRecording)" class="max-w-[100px] truncate">
                {{ player.currentTrack?.callsign || 'Recording...' }}
             </span>
             <span v-else>PLAYER</span>
         </button>

         <slot name="header-actions"></slot>
      </div>
      
      <!-- Desktop Player Dropdown -->
      <DesktopPlayer />
    </header>

    <div class="flex flex-1 relative">
      <!-- Sidebar -->
      <!-- Mobile Overlay -->
      <div v-if="theme.sidebarOpen && isMobile" 
           @click="theme.toggleSidebar"
           class="fixed inset-0 bg-black/50 z-40 lg:hidden backdrop-blur-sm"></div>

      <aside :class="sidebarClasses">
        <div class="p-4 flex-1 overflow-y-auto">
          <slot name="sidebar"></slot>
        </div>
      </aside>

      <!-- Main Content -->
      <main class="flex-1 flex flex-col min-w-0 overflow-hidden relative">
        <div class="flex-1 p-4 lg:p-8 overflow-y-auto w-full">
             <slot></slot>
        </div>

        <!-- Footer -->
        <footer class="bg-white dark:bg-slate-900 border-t border-slate-200 dark:border-slate-800 py-4 px-6 text-center text-xs text-slate-400
                       lg:sticky lg:bottom-0 z-30"> <!-- Sticky on desktop only -->
           <p>
             urfd-nng-dashboard 
             <span class="font-mono text-slate-500">{{ theme.config.version }}</span> 
           </p>
           <p class="mt-1 flex items-center justify-center gap-1">
             Made with <span class="text-red-500">♥</span> in Macomb, MI
           </p>
        </footer>
      </main>
    </div>

  </div>
</template>
