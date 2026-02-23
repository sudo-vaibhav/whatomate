<script setup lang="ts">
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useCallingStore } from '@/stores/calling'
import { Button } from '@/components/ui/button'
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip'
import { Phone, Loader2 } from 'lucide-vue-next'
import { toast } from 'vue-sonner'

const props = defineProps<{
  contactPhone: string
  contactName: string
  whatsappAccount: string
}>()

const { t } = useI18n()
const store = useCallingStore()
const isInitiating = ref(false)

async function handleCall() {
  if (store.isOnCall || isInitiating.value) return

  isInitiating.value = true
  try {
    await store.makeOutgoingCall(props.contactPhone, props.contactName, props.whatsappAccount)
  } catch (err: any) {
    toast.error(t('outgoingCalls.callFailed'), {
      description: err.message || String(err),
    })
  } finally {
    isInitiating.value = false
  }
}
</script>

<template>
  <Tooltip>
    <TooltipTrigger as-child>
      <Button
        variant="ghost"
        size="icon"
        class="h-8 w-8 text-white/50 hover:text-white hover:bg-white/[0.08] light:text-gray-500 light:hover:text-gray-900 light:hover:bg-gray-100"
        :disabled="store.isOnCall || isInitiating"
        @click="handleCall"
      >
        <Loader2 v-if="isInitiating" class="h-4 w-4 animate-spin" />
        <Phone v-else class="h-4 w-4" />
      </Button>
    </TooltipTrigger>
    <TooltipContent>{{ store.isOnCall ? t('outgoingCalls.callButtonDisabled') : t('outgoingCalls.callButton') }}</TooltipContent>
  </Tooltip>
</template>
