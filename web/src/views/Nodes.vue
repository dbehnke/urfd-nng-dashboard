<script setup lang="ts">
import { useReflectorStore } from '../stores/reflector'

const reflector = useReflectorStore()
</script>

<template>
  <div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
    <div v-for="node in reflector.clients" :key="node.Callsign" class="bg-white dark:bg-slate-900 p-6 rounded-xl shadow-sm border border-slate-200 dark:border-slate-800 hover:border-blue-500 transition-colors group">
      <div class="flex justify-between items-start mb-4">
        <div>
          <h4 class="text-xl font-bold group-hover:text-blue-600 dark:group-hover:text-blue-400 transition-colors">{{ node.Callsign }}</h4>
          <p class="text-xs text-slate-500 uppercase tracking-widest">{{ node.Protocol }}</p>
        </div>
        <span class="px-2 py-1 rounded bg-blue-50 dark:bg-blue-900/30 text-blue-600 dark:text-blue-400 font-mono text-sm">{{ node.OnModule }}</span>
      </div>
      
      <div class="space-y-2 text-sm">
        <div class="flex justify-between">
          <span class="text-slate-500">Connected for</span>
          <span class="font-medium">2h 14m</span> <!-- We would calculate this from ConnectTime -->
        </div>
      </div>
    </div>
    
    <div v-if="reflector.clients.length === 0" class="col-span-full py-12 text-center text-slate-400 italic bg-white dark:bg-slate-900 rounded-xl border border-dashed border-slate-300 dark:border-slate-800">
      No nodes connected to reflector.
    </div>
  </div>
</template>
