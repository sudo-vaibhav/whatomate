<script setup lang="ts">
import { ref, onMounted, computed, watch } from 'vue'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
import { ScrollArea } from '@/components/ui/scroll-area'
import { Separator } from '@/components/ui/separator'
import { Switch } from '@/components/ui/switch'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { Accordion, AccordionContent, AccordionItem, AccordionTrigger } from '@/components/ui/accordion'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover'
import {
  Command,
  CommandEmpty,
  CommandGroup,
  CommandInput,
  CommandItem,
  CommandList
} from '@/components/ui/command'
import { toast } from 'vue-sonner'
import { Settings, Bell, Loader2, Bot, Brain, Plus, X, Clock, AlertTriangle, UserPlus, MessageSquare, Users } from 'lucide-vue-next'
import { usersService, organizationService, chatbotService } from '@/services/api'

const isSubmitting = ref(false)
const isLoading = ref(true)

// General Settings
const generalSettings = ref({
  organization_name: 'My Organization',
  default_timezone: 'UTC',
  date_format: 'YYYY-MM-DD',
  mask_phone_numbers: false
})

// Notification Settings
const notificationSettings = ref({
  email_notifications: true,
  new_message_alerts: true,
  campaign_updates: true
})

// Chatbot Settings
interface MessageButton {
  id: string
  title: string
}

interface BusinessHour {
  day: number
  enabled: boolean
  start_time: string
  end_time: string
}

const daysOfWeek = ['Sunday', 'Monday', 'Tuesday', 'Wednesday', 'Thursday', 'Friday', 'Saturday']

const defaultBusinessHours: BusinessHour[] = [
  { day: 0, enabled: false, start_time: '09:00', end_time: '17:00' },
  { day: 1, enabled: true, start_time: '09:00', end_time: '17:00' },
  { day: 2, enabled: true, start_time: '09:00', end_time: '17:00' },
  { day: 3, enabled: true, start_time: '09:00', end_time: '17:00' },
  { day: 4, enabled: true, start_time: '09:00', end_time: '17:00' },
  { day: 5, enabled: true, start_time: '09:00', end_time: '17:00' },
  { day: 6, enabled: false, start_time: '09:00', end_time: '17:00' },
]

const chatbotSettings = ref({
  greeting_message: '',
  greeting_buttons: [] as MessageButton[],
  fallback_message: '',
  fallback_buttons: [] as MessageButton[],
  session_timeout_minutes: 30,
  business_hours_enabled: false,
  business_hours: [...defaultBusinessHours] as BusinessHour[],
  out_of_hours_message: '',
  allow_automated_outside_hours: true,
  allow_agent_queue_pickup: true,
  assign_to_same_agent: true,
  agent_current_conversation_only: false,
  transfer_message: ''
})

// Button management functions
const addGreetingButton = () => {
  if (chatbotSettings.value.greeting_buttons.length >= 10) {
    toast.error('Maximum 10 buttons allowed')
    return
  }
  const id = `btn_${Date.now()}`
  chatbotSettings.value.greeting_buttons.push({ id, title: '' })
}

const removeGreetingButton = (index: number) => {
  chatbotSettings.value.greeting_buttons.splice(index, 1)
}

const addFallbackButton = () => {
  if (chatbotSettings.value.fallback_buttons.length >= 10) {
    toast.error('Maximum 10 buttons allowed')
    return
  }
  const id = `btn_${Date.now()}`
  chatbotSettings.value.fallback_buttons.push({ id, title: '' })
}

const removeFallbackButton = (index: number) => {
  chatbotSettings.value.fallback_buttons.splice(index, 1)
}

// AI Settings
const aiSettings = ref({
  ai_enabled: false,
  ai_provider: '',
  ai_api_key: '',
  ai_model: '',
  ai_max_tokens: 500,
  ai_system_prompt: ''
})

const isAIEnabled = ref(false)

const aiProviders = [
  { value: 'openai', label: 'OpenAI', models: ['gpt-4o', 'gpt-4o-mini', 'gpt-4-turbo', 'gpt-3.5-turbo'] },
  { value: 'anthropic', label: 'Anthropic', models: ['claude-3-5-sonnet-latest', 'claude-3-5-haiku-latest', 'claude-3-opus-latest'] },
  { value: 'google', label: 'Google AI', models: ['gemini-2.0-flash', 'gemini-2.0-flash-lite', 'gemini-1.5-flash', 'gemini-1.5-flash-8b'] }
]

const availableModels = computed(() => {
  const provider = aiProviders.find(p => p.value === aiSettings.value.ai_provider)
  return provider?.models || []
})

watch(isAIEnabled, (newValue) => {
  aiSettings.value.ai_enabled = newValue
})

// SLA Settings
const slaSettings = ref({
  sla_enabled: false,
  sla_response_minutes: 15,
  sla_resolution_minutes: 60,
  sla_escalation_minutes: 30,
  sla_auto_close_hours: 24,
  sla_auto_close_message: '',
  sla_warning_message: '',
  sla_escalation_notify_ids: [] as string[]
})

const isSLAEnabled = ref(false)
const availableUsers = ref<{ id: string; full_name: string }[]>([])
const escalationComboboxOpen = ref(false)

const selectedEscalationUsers = computed(() => {
  return availableUsers.value.filter(u =>
    slaSettings.value.sla_escalation_notify_ids.includes(u.id)
  )
})

const unselectedUsers = computed(() => {
  return availableUsers.value.filter(u =>
    !slaSettings.value.sla_escalation_notify_ids.includes(u.id)
  )
})

watch(isSLAEnabled, (newValue) => {
  slaSettings.value.sla_enabled = newValue
})

onMounted(async () => {
  // Load all settings in parallel
  try {
    const [orgResponse, userResponse, chatbotResponse, usersResponse] = await Promise.all([
      organizationService.getSettings(),
      usersService.me(),
      chatbotService.getSettings(),
      usersService.list()
    ])

    // Organization settings
    const orgData = orgResponse.data.data || orgResponse.data
    if (orgData) {
      generalSettings.value = {
        organization_name: orgData.name || 'My Organization',
        default_timezone: orgData.settings?.timezone || 'UTC',
        date_format: orgData.settings?.date_format || 'YYYY-MM-DD',
        mask_phone_numbers: orgData.settings?.mask_phone_numbers || false
      }
    }

    // User notification settings
    const user = userResponse.data.data || userResponse.data
    if (user.settings) {
      notificationSettings.value = {
        email_notifications: user.settings.email_notifications ?? true,
        new_message_alerts: user.settings.new_message_alerts ?? true,
        campaign_updates: user.settings.campaign_updates ?? true
      }
    }

    // Users for escalation notify
    const usersData = usersResponse.data.data || usersResponse.data
    const usersList = usersData.users || usersData || []
    availableUsers.value = usersList.filter((u: any) => u.is_active !== false).map((u: any) => ({
      id: u.id,
      full_name: u.full_name
    }))

    // Chatbot settings
    const chatbotData = chatbotResponse.data.data || chatbotResponse.data
    if (chatbotData.settings) {
      const loadedHours = chatbotData.settings.business_hours || []
      const mergedHours = defaultBusinessHours.map(defaultDay => {
        const loaded = loadedHours.find((h: BusinessHour) => h.day === defaultDay.day)
        return loaded || defaultDay
      })

      chatbotSettings.value = {
        greeting_message: chatbotData.settings.greeting_message || '',
        greeting_buttons: chatbotData.settings.greeting_buttons || [],
        fallback_message: chatbotData.settings.fallback_message || '',
        fallback_buttons: chatbotData.settings.fallback_buttons || [],
        session_timeout_minutes: chatbotData.settings.session_timeout_minutes || 30,
        business_hours_enabled: chatbotData.settings.business_hours_enabled || false,
        business_hours: mergedHours,
        out_of_hours_message: chatbotData.settings.out_of_hours_message || '',
        allow_automated_outside_hours: chatbotData.settings.allow_automated_outside_hours !== false,
        allow_agent_queue_pickup: chatbotData.settings.allow_agent_queue_pickup !== false,
        assign_to_same_agent: chatbotData.settings.assign_to_same_agent !== false,
        agent_current_conversation_only: chatbotData.settings.agent_current_conversation_only === true,
        transfer_message: ''
      }

      const aiEnabledValue = chatbotData.settings.ai_enabled === true
      isAIEnabled.value = aiEnabledValue
      aiSettings.value = {
        ai_enabled: aiEnabledValue,
        ai_provider: chatbotData.settings.ai_provider || '',
        ai_api_key: '',
        ai_model: chatbotData.settings.ai_model || '',
        ai_max_tokens: chatbotData.settings.ai_max_tokens || 500,
        ai_system_prompt: chatbotData.settings.ai_system_prompt || ''
      }

      const slaEnabledValue = chatbotData.settings.sla_enabled === true
      isSLAEnabled.value = slaEnabledValue
      slaSettings.value = {
        sla_enabled: slaEnabledValue,
        sla_response_minutes: chatbotData.settings.sla_response_minutes || 15,
        sla_resolution_minutes: chatbotData.settings.sla_resolution_minutes || 60,
        sla_escalation_minutes: chatbotData.settings.sla_escalation_minutes || 30,
        sla_auto_close_hours: chatbotData.settings.sla_auto_close_hours || 24,
        sla_auto_close_message: chatbotData.settings.sla_auto_close_message || '',
        sla_warning_message: chatbotData.settings.sla_warning_message || '',
        sla_escalation_notify_ids: chatbotData.settings.sla_escalation_notify_ids || []
      }
    }
  } catch (error) {
    console.error('Failed to load settings:', error)
  } finally {
    isLoading.value = false
  }
})

async function saveGeneralSettings() {
  isSubmitting.value = true
  try {
    await organizationService.updateSettings({
      name: generalSettings.value.organization_name,
      timezone: generalSettings.value.default_timezone,
      date_format: generalSettings.value.date_format,
      mask_phone_numbers: generalSettings.value.mask_phone_numbers
    })
    toast.success('General settings saved')
  } catch (error) {
    toast.error('Failed to save settings')
  } finally {
    isSubmitting.value = false
  }
}

async function saveNotificationSettings() {
  isSubmitting.value = true
  try {
    await usersService.updateSettings({
      email_notifications: notificationSettings.value.email_notifications,
      new_message_alerts: notificationSettings.value.new_message_alerts,
      campaign_updates: notificationSettings.value.campaign_updates
    })
    toast.success('Notification settings saved')
  } catch (error) {
    toast.error('Failed to save notification settings')
  } finally {
    isSubmitting.value = false
  }
}

async function saveChatbotSettings() {
  const invalidGreetingBtn = chatbotSettings.value.greeting_buttons.find(btn => !btn.title.trim())
  if (invalidGreetingBtn) {
    toast.error('All greeting buttons must have a title')
    return
  }
  const invalidFallbackBtn = chatbotSettings.value.fallback_buttons.find(btn => !btn.title.trim())
  if (invalidFallbackBtn) {
    toast.error('All fallback buttons must have a title')
    return
  }

  isSubmitting.value = true
  try {
    await chatbotService.updateSettings({
      greeting_message: chatbotSettings.value.greeting_message,
      greeting_buttons: chatbotSettings.value.greeting_buttons.filter(btn => btn.title.trim()),
      fallback_message: chatbotSettings.value.fallback_message,
      fallback_buttons: chatbotSettings.value.fallback_buttons.filter(btn => btn.title.trim()),
      session_timeout_minutes: chatbotSettings.value.session_timeout_minutes,
      allow_agent_queue_pickup: chatbotSettings.value.allow_agent_queue_pickup,
      assign_to_same_agent: chatbotSettings.value.assign_to_same_agent,
      agent_current_conversation_only: chatbotSettings.value.agent_current_conversation_only
    })
    toast.success('Chatbot settings saved')
  } catch (error) {
    toast.error('Failed to save settings')
  } finally {
    isSubmitting.value = false
  }
}

async function saveBusinessHoursSettings() {
  isSubmitting.value = true
  try {
    await chatbotService.updateSettings({
      business_hours_enabled: chatbotSettings.value.business_hours_enabled,
      business_hours: chatbotSettings.value.business_hours,
      out_of_hours_message: chatbotSettings.value.out_of_hours_message,
      allow_automated_outside_hours: chatbotSettings.value.allow_automated_outside_hours
    })
    toast.success('Business hours settings saved')
  } catch (error) {
    toast.error('Failed to save settings')
  } finally {
    isSubmitting.value = false
  }
}

async function saveAISettings() {
  isSubmitting.value = true
  try {
    const payload: any = {
      ai_enabled: aiSettings.value.ai_enabled,
      ai_provider: aiSettings.value.ai_provider,
      ai_model: aiSettings.value.ai_model,
      ai_max_tokens: aiSettings.value.ai_max_tokens,
      ai_system_prompt: aiSettings.value.ai_system_prompt
    }
    if (aiSettings.value.ai_api_key) {
      payload.ai_api_key = aiSettings.value.ai_api_key
    }
    await chatbotService.updateSettings(payload)
    toast.success('AI settings saved')
    aiSettings.value.ai_api_key = ''
  } catch (error) {
    toast.error('Failed to save AI settings')
  } finally {
    isSubmitting.value = false
  }
}

async function saveSLASettings() {
  isSubmitting.value = true
  try {
    await chatbotService.updateSettings({
      sla_enabled: slaSettings.value.sla_enabled,
      sla_response_minutes: slaSettings.value.sla_response_minutes,
      sla_resolution_minutes: slaSettings.value.sla_resolution_minutes,
      sla_escalation_minutes: slaSettings.value.sla_escalation_minutes,
      sla_auto_close_hours: slaSettings.value.sla_auto_close_hours,
      sla_auto_close_message: slaSettings.value.sla_auto_close_message,
      sla_warning_message: slaSettings.value.sla_warning_message,
      sla_escalation_notify_ids: slaSettings.value.sla_escalation_notify_ids
    })
    toast.success('SLA settings saved')
  } catch (error) {
    toast.error('Failed to save SLA settings')
  } finally {
    isSubmitting.value = false
  }
}

function addEscalationUser(userId: string) {
  if (!slaSettings.value.sla_escalation_notify_ids.includes(userId)) {
    slaSettings.value.sla_escalation_notify_ids.push(userId)
  }
  escalationComboboxOpen.value = false
}

function removeEscalationUser(userId: string) {
  const index = slaSettings.value.sla_escalation_notify_ids.indexOf(userId)
  if (index !== -1) {
    slaSettings.value.sla_escalation_notify_ids.splice(index, 1)
  }
}
</script>

<template>
  <div class="flex flex-col h-full">
    <!-- Header -->
    <header class="border-b bg-background/95 backdrop-blur supports-[backdrop-filter]:bg-background/60">
      <div class="flex h-16 items-center px-6">
        <Settings class="h-5 w-5 mr-3" />
        <div class="flex-1">
          <h1 class="text-xl font-semibold">Settings</h1>
          <p class="text-sm text-muted-foreground">Manage your account and application settings</p>
        </div>
      </div>
    </header>

    <!-- Content -->
    <ScrollArea class="flex-1">
      <div class="p-6 space-y-4 max-w-4xl mx-auto">
        <Tabs default-value="general" class="w-full">
          <TabsList class="grid w-full grid-cols-3 mb-6">
            <TabsTrigger value="general">
              <Settings class="h-4 w-4 mr-2" />
              General
            </TabsTrigger>
            <TabsTrigger value="notifications">
              <Bell class="h-4 w-4 mr-2" />
              Notifications
            </TabsTrigger>
            <TabsTrigger value="chatbot">
              <Bot class="h-4 w-4 mr-2" />
              Chatbot
            </TabsTrigger>
          </TabsList>

          <!-- General Settings Tab -->
          <TabsContent value="general">
            <Card>
              <CardHeader>
                <CardTitle>General Settings</CardTitle>
                <CardDescription>Basic organization and display settings</CardDescription>
              </CardHeader>
              <CardContent class="space-y-4">
                <div class="space-y-2">
                  <Label for="org_name">Organization Name</Label>
                  <Input
                    id="org_name"
                    v-model="generalSettings.organization_name"
                    placeholder="Your Organization"
                  />
                </div>
                <div class="grid grid-cols-2 gap-4">
                  <div class="space-y-2">
                    <Label for="timezone">Default Timezone</Label>
                    <Select v-model="generalSettings.default_timezone">
                      <SelectTrigger>
                        <SelectValue placeholder="Select timezone" />
                      </SelectTrigger>
                      <SelectContent>
                        <SelectItem value="UTC">UTC</SelectItem>
                        <SelectItem value="America/New_York">Eastern Time</SelectItem>
                        <SelectItem value="America/Los_Angeles">Pacific Time</SelectItem>
                        <SelectItem value="Europe/London">London</SelectItem>
                        <SelectItem value="Asia/Tokyo">Tokyo</SelectItem>
                      </SelectContent>
                    </Select>
                  </div>
                  <div class="space-y-2">
                    <Label for="date_format">Date Format</Label>
                    <Select v-model="generalSettings.date_format">
                      <SelectTrigger>
                        <SelectValue placeholder="Select format" />
                      </SelectTrigger>
                      <SelectContent>
                        <SelectItem value="YYYY-MM-DD">YYYY-MM-DD</SelectItem>
                        <SelectItem value="DD/MM/YYYY">DD/MM/YYYY</SelectItem>
                        <SelectItem value="MM/DD/YYYY">MM/DD/YYYY</SelectItem>
                      </SelectContent>
                    </Select>
                  </div>
                </div>
                <Separator />
                <div class="flex items-center justify-between">
                  <div>
                    <p class="font-medium">Mask Phone Numbers</p>
                    <p class="text-sm text-muted-foreground">Hide phone numbers showing only last 4 digits</p>
                  </div>
                  <Switch
                    :checked="generalSettings.mask_phone_numbers"
                    @update:checked="generalSettings.mask_phone_numbers = $event"
                  />
                </div>
                <div class="flex justify-end">
                  <Button variant="outline" size="sm" @click="saveGeneralSettings" :disabled="isSubmitting">
                    <Loader2 v-if="isSubmitting" class="mr-2 h-4 w-4 animate-spin" />
                    Save Changes
                  </Button>
                </div>
              </CardContent>
            </Card>
          </TabsContent>

          <!-- Notification Settings Tab -->
          <TabsContent value="notifications">
            <Card>
              <CardHeader>
                <CardTitle>Notifications</CardTitle>
                <CardDescription>Manage how you receive notifications</CardDescription>
              </CardHeader>
              <CardContent class="space-y-4">
                <div class="flex items-center justify-between">
                  <div>
                    <p class="font-medium">Email Notifications</p>
                    <p class="text-sm text-muted-foreground">Receive important updates via email</p>
                  </div>
                  <Switch
                    :checked="notificationSettings.email_notifications"
                    @update:checked="notificationSettings.email_notifications = $event"
                  />
                </div>
                <Separator />
                <div class="flex items-center justify-between">
                  <div>
                    <p class="font-medium">New Message Alerts</p>
                    <p class="text-sm text-muted-foreground">Get notified when new messages arrive</p>
                  </div>
                  <Switch
                    :checked="notificationSettings.new_message_alerts"
                    @update:checked="notificationSettings.new_message_alerts = $event"
                  />
                </div>
                <Separator />
                <div class="flex items-center justify-between">
                  <div>
                    <p class="font-medium">Campaign Updates</p>
                    <p class="text-sm text-muted-foreground">Receive campaign status notifications</p>
                  </div>
                  <Switch
                    :checked="notificationSettings.campaign_updates"
                    @update:checked="notificationSettings.campaign_updates = $event"
                  />
                </div>
                <div class="flex justify-end pt-4">
                  <Button variant="outline" size="sm" @click="saveNotificationSettings" :disabled="isSubmitting">
                    <Loader2 v-if="isSubmitting" class="mr-2 h-4 w-4 animate-spin" />
                    Save Changes
                  </Button>
                </div>
              </CardContent>
            </Card>
          </TabsContent>

          <!-- Chatbot Settings Tab -->
          <TabsContent value="chatbot">
            <Accordion type="multiple" :default-value="['messages']" class="space-y-4">
              <!-- Messages Section -->
              <AccordionItem value="messages" class="border rounded-lg px-4">
                <AccordionTrigger class="hover:no-underline">
                  <div class="flex items-center gap-3">
                    <MessageSquare class="h-5 w-5 text-muted-foreground" />
                    <div class="text-left">
                      <p class="font-medium">Messages</p>
                      <p class="text-sm text-muted-foreground font-normal">Greeting, fallback messages and session timeout</p>
                    </div>
                  </div>
                </AccordionTrigger>
                <AccordionContent class="pt-4 pb-6">
                  <div class="space-y-4">
                    <div class="space-y-2">
                      <Label for="greeting">Greeting Message</Label>
                      <Textarea
                        id="greeting"
                        v-model="chatbotSettings.greeting_message"
                        placeholder="Hello! How can I help you?"
                        :rows="2"
                      />
                      <!-- Greeting Buttons -->
                      <div class="mt-2">
                        <div class="flex items-center justify-between mb-2">
                          <Label class="text-sm text-muted-foreground">Quick Reply Buttons (optional)</Label>
                          <Button
                            variant="outline"
                            size="sm"
                            @click="addGreetingButton"
                            :disabled="chatbotSettings.greeting_buttons.length >= 10"
                          >
                            <Plus class="h-4 w-4 mr-1" />
                            Add Button
                          </Button>
                        </div>
                        <div v-if="chatbotSettings.greeting_buttons.length > 0" class="space-y-2">
                          <div
                            v-for="(button, index) in chatbotSettings.greeting_buttons"
                            :key="button.id"
                            class="flex items-center gap-2"
                          >
                            <Input
                              v-model="button.title"
                              placeholder="Button text (max 20 chars)"
                              maxlength="20"
                              class="flex-1"
                            />
                            <Button variant="ghost" size="icon" @click="removeGreetingButton(index)">
                              <X class="h-4 w-4" />
                            </Button>
                          </div>
                          <p class="text-xs text-muted-foreground">1-3 buttons show as reply buttons, 4-10 show as a list menu</p>
                        </div>
                      </div>
                    </div>

                    <div class="space-y-2">
                      <Label for="fallback">Fallback Message</Label>
                      <Textarea
                        id="fallback"
                        v-model="chatbotSettings.fallback_message"
                        placeholder="Sorry, I didn't understand that."
                        :rows="2"
                      />
                      <!-- Fallback Buttons -->
                      <div class="mt-2">
                        <div class="flex items-center justify-between mb-2">
                          <Label class="text-sm text-muted-foreground">Quick Reply Buttons (optional)</Label>
                          <Button
                            variant="outline"
                            size="sm"
                            @click="addFallbackButton"
                            :disabled="chatbotSettings.fallback_buttons.length >= 10"
                          >
                            <Plus class="h-4 w-4 mr-1" />
                            Add Button
                          </Button>
                        </div>
                        <div v-if="chatbotSettings.fallback_buttons.length > 0" class="space-y-2">
                          <div
                            v-for="(button, index) in chatbotSettings.fallback_buttons"
                            :key="button.id"
                            class="flex items-center gap-2"
                          >
                            <Input
                              v-model="button.title"
                              placeholder="Button text (max 20 chars)"
                              maxlength="20"
                              class="flex-1"
                            />
                            <Button variant="ghost" size="icon" @click="removeFallbackButton(index)">
                              <X class="h-4 w-4" />
                            </Button>
                          </div>
                          <p class="text-xs text-muted-foreground">1-3 buttons show as reply buttons, 4-10 show as a list menu</p>
                        </div>
                      </div>
                    </div>

                    <div class="space-y-2">
                      <Label for="timeout">Session Timeout (minutes)</Label>
                      <Input
                        id="timeout"
                        v-model.number="chatbotSettings.session_timeout_minutes"
                        type="number"
                        min="5"
                        max="120"
                        class="w-32"
                      />
                    </div>

                    <div class="flex justify-end pt-2">
                      <Button variant="outline" size="sm" @click="saveChatbotSettings" :disabled="isSubmitting">
                        <Loader2 v-if="isSubmitting" class="mr-2 h-4 w-4 animate-spin" />
                        Save Changes
                      </Button>
                    </div>
                  </div>
                </AccordionContent>
              </AccordionItem>

              <!-- Agent Settings Section -->
              <AccordionItem value="agents" class="border rounded-lg px-4">
                <AccordionTrigger class="hover:no-underline">
                  <div class="flex items-center gap-3">
                    <Users class="h-5 w-5 text-muted-foreground" />
                    <div class="text-left">
                      <p class="font-medium">Agent Settings</p>
                      <p class="text-sm text-muted-foreground font-normal">Transfer queue and agent assignment options</p>
                    </div>
                  </div>
                </AccordionTrigger>
                <AccordionContent class="pt-4 pb-6">
                  <div class="space-y-4">
                    <div class="flex items-center justify-between py-2">
                      <div>
                        <p class="font-medium text-sm">Allow Agents to Pick from Queue</p>
                        <p class="text-xs text-muted-foreground">When enabled, agents can self-assign transfers from the queue</p>
                      </div>
                      <Switch
                        :checked="chatbotSettings.allow_agent_queue_pickup"
                        @update:checked="chatbotSettings.allow_agent_queue_pickup = $event"
                      />
                    </div>

                    <Separator />

                    <div class="flex items-center justify-between py-2">
                      <div>
                        <p class="font-medium text-sm">Assign to Same Agent</p>
                        <p class="text-xs text-muted-foreground">When enabled, transfers are auto-assigned to the contact's existing agent</p>
                      </div>
                      <Switch
                        :checked="chatbotSettings.assign_to_same_agent"
                        @update:checked="chatbotSettings.assign_to_same_agent = $event"
                      />
                    </div>

                    <Separator />

                    <div class="flex items-center justify-between py-2">
                      <div>
                        <p class="font-medium text-sm">Agents See Current Conversation Only</p>
                        <p class="text-xs text-muted-foreground">When enabled, agents only see messages from the current session</p>
                      </div>
                      <Switch
                        :checked="chatbotSettings.agent_current_conversation_only"
                        @update:checked="chatbotSettings.agent_current_conversation_only = $event"
                      />
                    </div>

                    <div class="flex justify-end pt-2">
                      <Button variant="outline" size="sm" @click="saveChatbotSettings" :disabled="isSubmitting">
                        <Loader2 v-if="isSubmitting" class="mr-2 h-4 w-4 animate-spin" />
                        Save Changes
                      </Button>
                    </div>
                  </div>
                </AccordionContent>
              </AccordionItem>

              <!-- Business Hours Section -->
              <AccordionItem value="business-hours" class="border rounded-lg px-4">
                <AccordionTrigger class="hover:no-underline">
                  <div class="flex items-center gap-3">
                    <Clock class="h-5 w-5 text-muted-foreground" />
                    <div class="text-left">
                      <p class="font-medium">Business Hours</p>
                      <p class="text-sm text-muted-foreground font-normal">Set when the chatbot is active</p>
                    </div>
                  </div>
                </AccordionTrigger>
                <AccordionContent class="pt-4 pb-6">
                  <div class="space-y-4">
                    <div class="flex items-center justify-between">
                      <div>
                        <p class="font-medium">Enable Business Hours</p>
                        <p class="text-sm text-muted-foreground">Restrict chatbot activity to specific hours</p>
                      </div>
                      <Switch
                        :checked="chatbotSettings.business_hours_enabled"
                        @update:checked="chatbotSettings.business_hours_enabled = $event"
                      />
                    </div>

                    <div v-if="chatbotSettings.business_hours_enabled" class="space-y-4 pt-2">
                      <Separator />

                      <div class="border rounded-lg p-4 space-y-3">
                        <div
                          v-for="hour in chatbotSettings.business_hours"
                          :key="hour.day"
                          class="flex items-center gap-4"
                        >
                          <div class="w-20">
                            <Switch
                              :checked="hour.enabled"
                              @update:checked="hour.enabled = $event"
                            />
                          </div>
                          <span class="w-24 font-medium" :class="{ 'text-muted-foreground': !hour.enabled }">
                            {{ daysOfWeek[hour.day] }}
                          </span>
                          <div class="flex items-center gap-2" :class="{ 'opacity-50': !hour.enabled }">
                            <Input
                              v-model="hour.start_time"
                              type="time"
                              class="w-28"
                              :disabled="!hour.enabled"
                            />
                            <span class="text-muted-foreground">to</span>
                            <Input
                              v-model="hour.end_time"
                              type="time"
                              class="w-28"
                              :disabled="!hour.enabled"
                            />
                          </div>
                        </div>
                      </div>

                      <Separator />

                      <div class="space-y-2">
                        <Label>Out of Hours Message</Label>
                        <Textarea
                          v-model="chatbotSettings.out_of_hours_message"
                          placeholder="Sorry, we're currently closed. We'll get back to you soon!"
                          :rows="2"
                        />
                      </div>

                      <div class="flex items-center justify-between py-2">
                        <div>
                          <p class="font-medium text-sm">Allow Automated Responses Outside Hours</p>
                          <p class="text-xs text-muted-foreground">Continue processing flows, keywords, and AI outside business hours</p>
                        </div>
                        <Switch
                          :checked="chatbotSettings.allow_automated_outside_hours"
                          @update:checked="chatbotSettings.allow_automated_outside_hours = $event"
                        />
                      </div>
                    </div>

                    <div class="flex justify-end pt-2">
                      <Button variant="outline" size="sm" @click="saveBusinessHoursSettings" :disabled="isSubmitting">
                        <Loader2 v-if="isSubmitting" class="mr-2 h-4 w-4 animate-spin" />
                        Save Changes
                      </Button>
                    </div>
                  </div>
                </AccordionContent>
              </AccordionItem>

              <!-- SLA Section -->
              <AccordionItem value="sla" class="border rounded-lg px-4">
                <AccordionTrigger class="hover:no-underline">
                  <div class="flex items-center gap-3">
                    <AlertTriangle class="h-5 w-5 text-muted-foreground" />
                    <div class="text-left">
                      <p class="font-medium">SLA Settings</p>
                      <p class="text-sm text-muted-foreground font-normal">Service level agreements and escalation</p>
                    </div>
                  </div>
                </AccordionTrigger>
                <AccordionContent class="pt-4 pb-6">
                  <div class="space-y-4">
                    <div class="flex items-center justify-between">
                      <div>
                        <p class="font-medium">Enable SLA Tracking</p>
                        <p class="text-sm text-muted-foreground">Track response times and escalate overdue transfers</p>
                      </div>
                      <Switch
                        :checked="isSLAEnabled"
                        @update:checked="(val: boolean) => isSLAEnabled = val"
                      />
                    </div>

                    <div v-if="isSLAEnabled" class="space-y-4 pt-2">
                      <Separator />

                      <div class="grid grid-cols-2 gap-4">
                        <div class="space-y-2">
                          <Label>Response Time (minutes)</Label>
                          <Input v-model.number="slaSettings.sla_response_minutes" type="number" min="1" max="1440" />
                          <p class="text-xs text-muted-foreground">Time for agent to pick up</p>
                        </div>
                        <div class="space-y-2">
                          <Label>Escalation Time (minutes)</Label>
                          <Input v-model.number="slaSettings.sla_escalation_minutes" type="number" min="1" max="1440" />
                          <p class="text-xs text-muted-foreground">Time before escalating</p>
                        </div>
                      </div>

                      <div class="grid grid-cols-2 gap-4">
                        <div class="space-y-2">
                          <Label>Resolution Time (minutes)</Label>
                          <Input v-model.number="slaSettings.sla_resolution_minutes" type="number" min="1" max="10080" />
                          <p class="text-xs text-muted-foreground">Expected resolution time</p>
                        </div>
                        <div class="space-y-2">
                          <Label>Auto-Close (hours)</Label>
                          <Input v-model.number="slaSettings.sla_auto_close_hours" type="number" min="1" max="168" />
                          <p class="text-xs text-muted-foreground">Auto-close inactive chats</p>
                        </div>
                      </div>

                      <div class="space-y-2">
                        <Label>Auto-Close Message</Label>
                        <Textarea
                          v-model="slaSettings.sla_auto_close_message"
                          placeholder="Your chat has been closed due to inactivity."
                          :rows="2"
                        />
                      </div>

                      <div class="space-y-2">
                        <Label>Customer Warning Message</Label>
                        <Textarea
                          v-model="slaSettings.sla_warning_message"
                          placeholder="We're experiencing higher than usual wait times."
                          :rows="2"
                        />
                      </div>

                      <Separator />

                      <div class="space-y-3">
                        <div class="flex items-center justify-between">
                          <div>
                            <Label>Escalation Notify Contacts</Label>
                            <p class="text-xs text-muted-foreground">Users to notify on escalation</p>
                          </div>
                          <Popover v-model:open="escalationComboboxOpen">
                            <PopoverTrigger as-child>
                              <Button variant="outline" size="sm" class="gap-2" :disabled="unselectedUsers.length === 0">
                                <UserPlus class="h-4 w-4" />
                                Add User
                              </Button>
                            </PopoverTrigger>
                            <PopoverContent class="w-[250px] p-0" align="end">
                              <Command>
                                <CommandInput placeholder="Search users..." />
                                <CommandList>
                                  <CommandEmpty>No users found.</CommandEmpty>
                                  <CommandGroup>
                                    <CommandItem
                                      v-for="user in unselectedUsers"
                                      :key="user.id"
                                      :value="user.full_name"
                                      @select="addEscalationUser(user.id)"
                                      class="cursor-pointer"
                                    >
                                      {{ user.full_name }}
                                    </CommandItem>
                                  </CommandGroup>
                                </CommandList>
                              </Command>
                            </PopoverContent>
                          </Popover>
                        </div>

                        <div v-if="selectedEscalationUsers.length > 0" class="flex flex-wrap gap-2">
                          <div
                            v-for="user in selectedEscalationUsers"
                            :key="user.id"
                            class="flex items-center gap-2 px-3 py-1.5 bg-muted rounded-full text-sm"
                          >
                            <span>{{ user.full_name }}</span>
                            <button type="button" @click="removeEscalationUser(user.id)" class="text-muted-foreground hover:text-foreground">
                              <X class="h-3.5 w-3.5" />
                            </button>
                          </div>
                        </div>
                        <p v-else class="text-sm text-muted-foreground italic">No users selected</p>
                      </div>
                    </div>

                    <div class="flex justify-end pt-2">
                      <Button variant="outline" size="sm" @click="saveSLASettings" :disabled="isSubmitting">
                        <Loader2 v-if="isSubmitting" class="mr-2 h-4 w-4 animate-spin" />
                        Save Changes
                      </Button>
                    </div>
                  </div>
                </AccordionContent>
              </AccordionItem>

              <!-- AI Section -->
              <AccordionItem value="ai" class="border rounded-lg px-4">
                <AccordionTrigger class="hover:no-underline">
                  <div class="flex items-center gap-3">
                    <Brain class="h-5 w-5 text-muted-foreground" />
                    <div class="text-left">
                      <p class="font-medium">AI Settings</p>
                      <p class="text-sm text-muted-foreground font-normal">Configure AI-powered responses</p>
                    </div>
                  </div>
                </AccordionTrigger>
                <AccordionContent class="pt-4 pb-6">
                  <div class="space-y-4">
                    <div class="flex items-center justify-between">
                      <div>
                        <p class="font-medium">Enable AI Responses</p>
                        <p class="text-sm text-muted-foreground">Use AI to generate responses when no flow matches</p>
                      </div>
                      <Switch
                        :checked="isAIEnabled"
                        @update:checked="(val: boolean) => isAIEnabled = val"
                      />
                    </div>

                    <div v-if="isAIEnabled" class="space-y-4 pt-2">
                      <Separator />

                      <div class="grid grid-cols-2 gap-4">
                        <div class="space-y-2">
                          <Label>AI Provider</Label>
                          <Select v-model="aiSettings.ai_provider">
                            <SelectTrigger>
                              <SelectValue placeholder="Select provider..." />
                            </SelectTrigger>
                            <SelectContent>
                              <SelectItem v-for="provider in aiProviders" :key="provider.value" :value="provider.value">
                                {{ provider.label }}
                              </SelectItem>
                            </SelectContent>
                          </Select>
                        </div>
                        <div class="space-y-2">
                          <Label>Model</Label>
                          <Select v-model="aiSettings.ai_model" :disabled="!aiSettings.ai_provider">
                            <SelectTrigger>
                              <SelectValue placeholder="Select model..." />
                            </SelectTrigger>
                            <SelectContent>
                              <SelectItem v-for="model in availableModels" :key="model" :value="model">
                                {{ model }}
                              </SelectItem>
                            </SelectContent>
                          </Select>
                        </div>
                      </div>

                      <div class="space-y-2">
                        <Label>API Key</Label>
                        <Input
                          v-model="aiSettings.ai_api_key"
                          type="password"
                          placeholder="Enter API key (leave empty to keep existing)"
                        />
                        <p class="text-xs text-muted-foreground">Your API key is encrypted and stored securely</p>
                      </div>

                      <div class="space-y-2">
                        <Label>Max Tokens</Label>
                        <Input v-model.number="aiSettings.ai_max_tokens" type="number" min="100" max="4000" class="w-32" />
                      </div>

                      <div class="space-y-2">
                        <Label>System Prompt (optional)</Label>
                        <Textarea
                          v-model="aiSettings.ai_system_prompt"
                          placeholder="You are a helpful customer service assistant..."
                          :rows="3"
                        />
                      </div>
                    </div>

                    <div class="flex justify-end pt-2">
                      <Button variant="outline" size="sm" @click="saveAISettings" :disabled="isSubmitting">
                        <Loader2 v-if="isSubmitting" class="mr-2 h-4 w-4 animate-spin" />
                        Save Changes
                      </Button>
                    </div>
                  </div>
                </AccordionContent>
              </AccordionItem>
            </Accordion>
          </TabsContent>
        </Tabs>
      </div>
    </ScrollArea>
  </div>
</template>
