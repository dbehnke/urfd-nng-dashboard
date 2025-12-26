<script setup lang="ts">
import { useReflectorStore } from '../stores/reflector'
import { formatTimeSince } from '../utils/time'

const reflector = useReflectorStore()
</script>

<template>
  <div class="bg-white dark:bg-slate-900 rounded-xl shadow-sm border border-slate-200 dark:border-slate-800 overflow-hidden">
    <div class="p-6 border-b border-slate-200 dark:border-slate-800 flex justify-between items-center">
      <h3 class="text-lg font-semibold">Linked Peers</h3>
      <span class="text-xs font-medium px-2.5 py-0.5 rounded-full bg-blue-100 text-blue-800 dark:bg-blue-900/30 dark:text-blue-400">
        {{ reflector.peers.length }} Linked
      </span>
    </div>
    
    <div class="overflow-x-auto">
      <table class="w-full text-left border-collapse">
        <thead class="bg-slate-50 dark:bg-slate-800/50">
          <tr>
            <th class="px-6 py-4 font-medium text-slate-500 text-sm">Reflector</th>
            <th class="px-6 py-4 font-medium text-slate-500 text-sm">Protocol</th>
            <th class="px-6 py-4 font-medium text-slate-500 text-sm">Uptime</th>
          </tr>
        </thead>
        <tbody class="divide-y divide-slate-100 dark:divide-slate-800">
          <tr v-for="peer in reflector.peers" :key="peer.Callsign" class="hover:bg-slate-50 dark:hover:bg-slate-800/30 transition-colors">
            <td class="px-6 py-4 font-bold text-blue-600 dark:text-blue-400">{{ peer.Callsign }}</td>
            <td class="px-6 py-4 text-sm">{{ peer.Protocol }}</td>
            <td class="px-6 py-4 text-sm text-slate-500">{{ formatTimeSince(peer.ConnectTime) }} ago</td>
          </tr>
          <tr v-if="reflector.peers.length === 0">
            <td colspan="3" class="px-6 py-12 text-center text-slate-400 italic">No peer reflectors linked.</td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>
