<script setup lang="ts">
import { useReflectorStore } from '../stores/reflector'

const reflector = useReflectorStore()

const alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZ".split("")
</script>

<template>
  <div class="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-6 gap-4">
    <div v-for="m in alphabet" :key="m" 
         class="p-4 rounded-xl border transition-all duration-200"
         :class="reflector.users.some(u => u.OnModule === m) 
                  ? 'bg-blue-50 dark:bg-blue-900/20 border-blue-200 dark:border-blue-800' 
                  : 'bg-white dark:bg-slate-900 border-slate-200 dark:border-slate-800 opacity-60'">
      <div class="text-xs text-slate-400 font-bold mb-1">MODULE</div>
      <div class="text-3xl font-black text-slate-800 dark:text-slate-100">{{ m }}</div>
      
      <div class="mt-2 flex items-center gap-1.5">
        <div class="w-2 h-2 rounded-full" :class="reflector.users.some(u => u.OnModule === m) ? 'bg-green-500 animate-pulse' : 'bg-slate-300 dark:bg-slate-700'"></div>
        <span class="text-[10px] font-bold uppercase">{{ reflector.users.filter(u => u.OnModule === m).length }} Users</span>
      </div>
    </div>
  </div>
</template>
