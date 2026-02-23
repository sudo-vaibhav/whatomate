<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useCallingStore } from '@/stores/calling'
import { callTransfersService, type CallTransfer } from '@/services/api'
import { Card, CardContent } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Tabs, TabsList, TabsTrigger, TabsContent } from '@/components/ui/tabs'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { Phone, PhoneOff, PhoneForwarded, RefreshCw, Clock } from 'lucide-vue-next'
import { toast } from 'vue-sonner'

const { t } = useI18n()
const store = useCallingStore()

const activeTab = ref('waiting')
const historyTransfers = ref<CallTransfer[]>([])
const historyTotal = ref(0)
const historyPage = ref(1)
const historyLoading = ref(false)

const historyPages = computed(() => Math.ceil(historyTotal.value / 20) || 1)

async function fetchHistory() {
  historyLoading.value = true
  try {
    const response = await callTransfersService.list({ page: historyPage.value, limit: 20 })
    const data = response.data as any
    historyTransfers.value = (data.data?.call_transfers ?? data.call_transfers ?? [])
      .filter((t: CallTransfer) => t.status !== 'waiting')
    historyTotal.value = data.data?.total ?? data.total ?? 0
  } catch {
    // Silently handle
  } finally {
    historyLoading.value = false
  }
}

async function handleAccept(id: string) {
  try {
    await store.acceptTransfer(id)
    toast.success(t('callTransfers.callConnected'))
  } catch (err: any) {
    toast.error(t('callTransfers.acceptFailed'), {
      description: err.message || ''
    })
  }
}

function formatDuration(seconds: number): string {
  if (!seconds) return '-'
  const m = Math.floor(seconds / 60)
  const s = seconds % 60
  return m > 0 ? `${m}m ${s}s` : `${s}s`
}

function formatDate(dateStr?: string): string {
  if (!dateStr) return '-'
  return new Date(dateStr).toLocaleString()
}

function statusVariant(status: string): 'default' | 'secondary' | 'destructive' | 'outline' {
  switch (status) {
    case 'waiting': return 'default'
    case 'connected': return 'default'
    case 'completed': return 'secondary'
    case 'abandoned': return 'destructive'
    case 'no_answer': return 'outline'
    default: return 'secondary'
  }
}

onMounted(() => {
  store.fetchWaitingTransfers()
  fetchHistory()
})
</script>

<template>
  <div class="p-6 space-y-6">
    <div class="flex items-center justify-between">
      <div>
        <h1 class="text-2xl font-bold">{{ t('callTransfers.title') }}</h1>
      </div>
      <Button variant="outline" size="sm" @click="store.fetchWaitingTransfers(); fetchHistory()">
        <RefreshCw class="h-4 w-4 mr-2" />
        {{ t('common.refresh') }}
      </Button>
    </div>

    <Tabs v-model="activeTab">
      <TabsList>
        <TabsTrigger value="waiting">
          {{ t('callTransfers.waiting') }}
          <Badge v-if="store.waitingTransfers.length > 0" variant="destructive" class="ml-2 h-5 min-w-[20px]">
            {{ store.waitingTransfers.length }}
          </Badge>
        </TabsTrigger>
        <TabsTrigger value="history">{{ t('callTransfers.history') }}</TabsTrigger>
      </TabsList>

      <TabsContent value="waiting" class="mt-4">
        <Card>
          <CardContent class="p-0">
            <div v-if="store.waitingTransfers.length === 0" class="flex flex-col items-center justify-center py-12 text-zinc-400">
              <PhoneForwarded class="h-12 w-12 mb-3 opacity-50" />
              <p>{{ t('callTransfers.noWaiting') }}</p>
            </div>

            <Table v-else>
              <TableHeader>
                <TableRow>
                  <TableHead>{{ t('callTransfers.callerPhone') }}</TableHead>
                  <TableHead>{{ t('common.status') }}</TableHead>
                  <TableHead>{{ t('callTransfers.transferredAt') }}</TableHead>
                  <TableHead>{{ t('common.actions') }}</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                <TableRow v-for="transfer in store.waitingTransfers" :key="transfer.id">
                  <TableCell>
                    <div class="flex items-center gap-2">
                      <Phone class="h-4 w-4 text-green-400" />
                      <span>{{ transfer.contact?.profile_name || transfer.caller_phone }}</span>
                    </div>
                  </TableCell>
                  <TableCell>
                    <Badge variant="default" class="bg-yellow-600/20 text-yellow-400 border-yellow-600/30">
                      {{ t('callTransfers.waiting') }}
                    </Badge>
                  </TableCell>
                  <TableCell>{{ formatDate(transfer.transferred_at) }}</TableCell>
                  <TableCell>
                    <Button
                      size="sm"
                      class="bg-green-600 hover:bg-green-500 text-white"
                      @click="handleAccept(transfer.id)"
                    >
                      <Phone class="h-3.5 w-3.5 mr-1" />
                      {{ t('callTransfers.accept') }}
                    </Button>
                  </TableCell>
                </TableRow>
              </TableBody>
            </Table>
          </CardContent>
        </Card>
      </TabsContent>

      <TabsContent value="history" class="mt-4">
        <Card>
          <CardContent class="p-0">
            <div v-if="historyLoading" class="flex items-center justify-center py-12 text-zinc-400">
              <RefreshCw class="h-5 w-5 animate-spin mr-2" />
              {{ t('common.loading') }}
            </div>

            <div v-else-if="historyTransfers.length === 0" class="flex flex-col items-center justify-center py-12 text-zinc-400">
              <Clock class="h-12 w-12 mb-3 opacity-50" />
              <p>{{ t('common.noResults') }}</p>
            </div>

            <Table v-else>
              <TableHeader>
                <TableRow>
                  <TableHead>{{ t('callTransfers.callerPhone') }}</TableHead>
                  <TableHead>{{ t('common.status') }}</TableHead>
                  <TableHead>{{ t('callTransfers.holdDuration') }}</TableHead>
                  <TableHead>{{ t('callTransfers.talkDuration') }}</TableHead>
                  <TableHead>{{ t('callTransfers.transferredAt') }}</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                <TableRow v-for="transfer in historyTransfers" :key="transfer.id">
                  <TableCell>
                    <div class="flex items-center gap-2">
                      <component :is="transfer.status === 'completed' ? Phone : PhoneOff"
                        class="h-4 w-4"
                        :class="transfer.status === 'completed' ? 'text-green-400' : 'text-red-400'"
                      />
                      <span>{{ transfer.contact?.profile_name || transfer.caller_phone }}</span>
                    </div>
                  </TableCell>
                  <TableCell>
                    <Badge :variant="statusVariant(transfer.status)">
                      {{ transfer.status }}
                    </Badge>
                  </TableCell>
                  <TableCell>{{ formatDuration(transfer.hold_duration) }}</TableCell>
                  <TableCell>{{ formatDuration(transfer.talk_duration) }}</TableCell>
                  <TableCell>{{ formatDate(transfer.transferred_at) }}</TableCell>
                </TableRow>
              </TableBody>
            </Table>

            <div v-if="historyPages > 1" class="flex items-center justify-between p-4 border-t border-zinc-800">
              <Button
                variant="outline"
                size="sm"
                :disabled="historyPage <= 1"
                @click="historyPage--; fetchHistory()"
              >
                {{ t('common.back') }}
              </Button>
              <span class="text-sm text-zinc-400">{{ historyPage }} / {{ historyPages }}</span>
              <Button
                variant="outline"
                size="sm"
                :disabled="historyPage >= historyPages"
                @click="historyPage++; fetchHistory()"
              >
                {{ t('common.next') }}
              </Button>
            </div>
          </CardContent>
        </Card>
      </TabsContent>
    </Tabs>
  </div>
</template>
