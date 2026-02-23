<script setup lang="ts">
import { ref, computed, onMounted, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useCallingStore } from '@/stores/calling'
import { accountsService, type CallLog } from '@/services/api'
import { Card, CardContent } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogDescription } from '@/components/ui/dialog'
import { Phone, PhoneOff, PhoneMissed, Clock, RefreshCw } from 'lucide-vue-next'

const { t } = useI18n()
const store = useCallingStore()

// Filters
const statusFilter = ref('all')
const accountFilter = ref('all')
const currentPage = ref(1)
const accounts = ref<{ name: string }[]>([])

// Detail dialog
const showDetail = ref(false)
const selectedLog = ref<CallLog | null>(null)

const statusOptions = [
  { value: 'all', label: t('calling.allStatuses') },
  { value: 'completed', label: t('calling.completed') },
  { value: 'missed', label: t('calling.missed') },
  { value: 'ringing', label: t('calling.ringing') },
  { value: 'answered', label: t('calling.answered') },
  { value: 'rejected', label: t('calling.rejected') },
  { value: 'failed', label: t('calling.failed') }
]

const totalPages = computed(() => Math.ceil(store.callLogsTotal / store.callLogsLimit) || 1)

function fetchLogs() {
  store.fetchCallLogs({
    status: statusFilter.value !== 'all' ? statusFilter.value : undefined,
    account: accountFilter.value !== 'all' ? accountFilter.value : undefined,
    page: currentPage.value,
    limit: store.callLogsLimit
  })
}

function viewDetail(log: CallLog) {
  selectedLog.value = log
  showDetail.value = true
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
    case 'completed': return 'default'
    case 'answered': return 'default'
    case 'ringing': return 'secondary'
    case 'missed': return 'outline'
    case 'rejected': return 'destructive'
    case 'failed': return 'destructive'
    default: return 'secondary'
  }
}

function statusIcon(status: string) {
  switch (status) {
    case 'completed':
    case 'answered':
      return Phone
    case 'missed':
      return PhoneMissed
    case 'ringing':
      return Clock
    default:
      return PhoneOff
  }
}

onMounted(async () => {
  fetchLogs()
  try {
    const res = await accountsService.list()
    const data = res.data as any
    accounts.value = data.data?.accounts ?? data.accounts ?? []
  } catch {
    // Ignore
  }
})

watch([statusFilter, accountFilter], () => {
  currentPage.value = 1
  fetchLogs()
})

watch(currentPage, () => fetchLogs())
</script>

<template>
  <div class="p-6 space-y-6">
    <div class="flex items-center justify-between">
      <div>
        <h1 class="text-2xl font-bold">{{ t('calling.callLogs') }}</h1>
        <p class="text-muted-foreground">{{ t('calling.callLogsDesc') }}</p>
      </div>
      <Button variant="outline" size="sm" @click="fetchLogs">
        <RefreshCw class="h-4 w-4 mr-2" />
        {{ t('common.refresh') }}
      </Button>
    </div>

    <!-- Filters -->
    <Card>
      <CardContent class="pt-6">
        <div class="flex gap-4 flex-wrap">
          <Select v-model="statusFilter">
            <SelectTrigger class="w-48">
              <SelectValue :placeholder="t('calling.filterByStatus')" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem v-for="opt in statusOptions" :key="opt.value" :value="opt.value">
                {{ opt.label }}
              </SelectItem>
            </SelectContent>
          </Select>

          <Select v-model="accountFilter">
            <SelectTrigger class="w-48">
              <SelectValue :placeholder="t('calling.filterByAccount')" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="all">{{ t('calling.allAccounts') }}</SelectItem>
              <SelectItem v-for="acc in accounts" :key="acc.name" :value="acc.name">
                {{ acc.name }}
              </SelectItem>
            </SelectContent>
          </Select>
        </div>
      </CardContent>
    </Card>

    <!-- Table -->
    <Card>
      <CardContent class="p-0">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>{{ t('calling.caller') }}</TableHead>
              <TableHead>{{ t('calling.status') }}</TableHead>
              <TableHead>{{ t('calling.duration') }}</TableHead>
              <TableHead>{{ t('calling.ivrFlow') }}</TableHead>
              <TableHead>{{ t('calling.account') }}</TableHead>
              <TableHead>{{ t('calling.time') }}</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            <TableRow
              v-for="log in store.callLogs"
              :key="log.id"
              class="cursor-pointer hover:bg-muted/50"
              @click="viewDetail(log)"
            >
              <TableCell>
                <div>
                  <p class="font-medium">{{ log.contact?.profile_name || log.caller_phone }}</p>
                  <p v-if="log.contact?.profile_name" class="text-sm text-muted-foreground">{{ log.caller_phone }}</p>
                </div>
              </TableCell>
              <TableCell>
                <Badge :variant="statusVariant(log.status)">
                  <component :is="statusIcon(log.status)" class="h-3 w-3 mr-1" />
                  {{ t(`calling.${log.status}`) }}
                </Badge>
              </TableCell>
              <TableCell>{{ formatDuration(log.duration) }}</TableCell>
              <TableCell>{{ log.ivr_flow?.name || '-' }}</TableCell>
              <TableCell>{{ log.whatsapp_account }}</TableCell>
              <TableCell>{{ formatDate(log.started_at || log.created_at) }}</TableCell>
            </TableRow>
            <TableRow v-if="!store.callLogsLoading && store.callLogs.length === 0">
              <TableCell :colspan="6" class="text-center py-8 text-muted-foreground">
                {{ t('calling.noCallLogs') }}
              </TableCell>
            </TableRow>
          </TableBody>
        </Table>

        <!-- Loading -->
        <div v-if="store.callLogsLoading" class="flex justify-center py-8">
          <div class="animate-spin rounded-full h-6 w-6 border-b-2 border-primary" />
        </div>
      </CardContent>
    </Card>

    <!-- Pagination -->
    <div v-if="totalPages > 1" class="flex items-center justify-between">
      <p class="text-sm text-muted-foreground">
        {{ t('calling.showing', { from: (currentPage - 1) * store.callLogsLimit + 1, to: Math.min(currentPage * store.callLogsLimit, store.callLogsTotal), total: store.callLogsTotal }) }}
      </p>
      <div class="flex gap-2">
        <Button variant="outline" size="sm" :disabled="currentPage <= 1" @click="currentPage--">
          {{ t('common.back') }}
        </Button>
        <Button variant="outline" size="sm" :disabled="currentPage >= totalPages" @click="currentPage++">
          {{ t('common.next') }}
        </Button>
      </div>
    </div>

    <!-- Detail Dialog -->
    <Dialog v-model:open="showDetail">
      <DialogContent class="max-w-lg">
        <DialogHeader>
          <DialogTitle>{{ t('calling.callDetail') }}</DialogTitle>
          <DialogDescription>
            {{ selectedLog?.contact?.profile_name || selectedLog?.caller_phone }}
          </DialogDescription>
        </DialogHeader>
        <div v-if="selectedLog" class="space-y-4">
          <div class="grid grid-cols-2 gap-4 text-sm">
            <div>
              <p class="text-muted-foreground">{{ t('calling.caller') }}</p>
              <p class="font-medium">{{ selectedLog.caller_phone }}</p>
            </div>
            <div>
              <p class="text-muted-foreground">{{ t('calling.status') }}</p>
              <Badge :variant="statusVariant(selectedLog.status)">
                {{ t(`calling.${selectedLog.status}`) }}
              </Badge>
            </div>
            <div>
              <p class="text-muted-foreground">{{ t('calling.duration') }}</p>
              <p class="font-medium">{{ formatDuration(selectedLog.duration) }}</p>
            </div>
            <div>
              <p class="text-muted-foreground">{{ t('calling.account') }}</p>
              <p class="font-medium">{{ selectedLog.whatsapp_account }}</p>
            </div>
            <div>
              <p class="text-muted-foreground">{{ t('calling.startedAt') }}</p>
              <p class="font-medium">{{ formatDate(selectedLog.started_at) }}</p>
            </div>
            <div>
              <p class="text-muted-foreground">{{ t('calling.endedAt') }}</p>
              <p class="font-medium">{{ formatDate(selectedLog.ended_at) }}</p>
            </div>
          </div>

          <div v-if="selectedLog.ivr_flow">
            <p class="text-sm text-muted-foreground mb-1">{{ t('calling.ivrFlow') }}</p>
            <p class="font-medium">{{ selectedLog.ivr_flow.name }}</p>
          </div>

          <div v-if="selectedLog.ivr_path?.steps?.length">
            <p class="text-sm text-muted-foreground mb-2">{{ t('calling.ivrPath') }}</p>
            <div class="space-y-1">
              <div
                v-for="(step, idx) in selectedLog.ivr_path.steps"
                :key="idx"
                class="flex items-center gap-2 text-sm"
              >
                <Badge variant="outline" class="font-mono">{{ step.digit }}</Badge>
                <span class="text-muted-foreground">{{ step.menu || t('calling.rootMenu') }}</span>
              </div>
            </div>
          </div>

          <div v-if="selectedLog.error_message">
            <p class="text-sm text-muted-foreground mb-1">{{ t('calling.error') }}</p>
            <p class="text-sm text-destructive">{{ selectedLog.error_message }}</p>
          </div>
        </div>
      </DialogContent>
    </Dialog>
  </div>
</template>
