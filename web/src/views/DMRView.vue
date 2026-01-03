<script setup lang="ts">
import { computed } from 'vue'
import { useReflectorStore } from '../stores/reflector'
import { formatTimeSince } from '../utils/time'

const reflector = useReflectorStore()

const dmrClients = computed(() => {
  return reflector.clients.filter(c => 
    (c.Protocol === 'DMR' || c.Protocol === 'DMRMMDVM') && 
    (c.Subscriptions && Array.isArray(c.Subscriptions))
  )
})

const getTimeoutLabel = (sub: any) => {
    if (sub.Type === 'Static') return 'Static'
    if (sub.TimeoutLeft < 0) return 'Infinite'
    return `${sub.TimeoutLeft}s`
}

const getBadgeColor = (sub: any) => {
    if (sub.Type === 'Static') return 'bg-purple-100 text-purple-800 dark:bg-purple-900/30 dark:text-purple-300'
    return 'bg-green-100 text-green-800 dark:bg-green-900/30 dark:text-green-300'
}
</script>

<template>
  <div class="space-y-6">
    <div class="flex justify-between items-center">
      <h2 class="text-2xl font-bold text-slate-800 dark:text-white">DMR Active Subscriptions</h2>
      <div class="text-sm text-slate-500">
        {{ dmrClients.length }} Clients Connected
      </div>
    </div>

    <div class="grid grid-cols-1 gap-6">
        <div v-if="dmrClients.length === 0" class="p-12 text-center text-slate-400 italic bg-white dark:bg-slate-900 rounded-2xl border-2 border-dashed border-slate-200 dark:border-slate-800">
            No DMR clients with active subscriptions found.
        </div>

        <div v-for="client in dmrClients" :key="client.Callsign" class="bg-white dark:bg-slate-900 rounded-xl shadow-sm border border-slate-200 dark:border-slate-800 overflow-hidden">
            <div class="px-6 py-4 border-b border-slate-200 dark:border-slate-800 flex justify-between items-center bg-slate-50/50 dark:bg-slate-800/30">
                <div class="flex items-center gap-4">
                    <div class="font-bold text-lg text-blue-600 dark:text-blue-400">{{ client.Callsign }}</div>
                    <div class="text-xs text-slate-500 px-2 py-0.5 bg-slate-200 dark:bg-slate-700 rounded">{{ client.OnModule ? `Module ${client.OnModule}` : 'No Module' }}</div>
                </div>
                <div class="text-xs text-slate-400 font-mono">{{ formatTimeSince(client.ConnectTime) }}</div>
            </div>

            <div class="p-0">
                <table class="w-full text-left text-sm">
                    <thead class="bg-slate-50 dark:bg-slate-800/50 text-xs uppercase text-slate-500 font-semibold">
                        <tr>
                            <th class="px-6 py-3 w-32">Slot</th>
                            <th class="px-6 py-3">Talkgroup</th>
                            <th class="px-6 py-3 w-32">Type</th>
                            <th class="px-6 py-3 w-32 text-right">Timeout</th>
                        </tr>
                    </thead>
                    <tbody class="divide-y divide-slate-100 dark:divide-slate-800">
                        <tr v-for="(sub, idx) in client.Subscriptions" :key="idx" class="hover:bg-slate-50/50 dark:hover:bg-slate-800/50">
                            <td class="px-6 py-3 font-mono text-slate-600 dark:text-slate-400">TS {{ sub.Slot }}</td>
                            <td class="px-6 py-3 font-bold text-slate-800 dark:text-slate-200">{{ sub.TG }}</td>
                            <td class="px-6 py-3">
                                <span class="px-2 py-0.5 rounded text-xs font-bold uppercase tracking-wider" :class="getBadgeColor(sub)">
                                    {{ sub.Type }}
                                </span>
                            </td>
                            <td class="px-6 py-3 text-right font-mono text-slate-500">
                                {{ getTimeoutLabel(sub) }}
                            </td>
                        </tr>
                        <tr v-if="client.Subscriptions.length === 0">
                            <td colspan="4" class="px-6 py-4 text-center text-slate-400 italic">No Active Subscriptions</td>
                        </tr>
                    </tbody>
                </table>
            </div>
        </div>
    </div>
  </div>
</template>
