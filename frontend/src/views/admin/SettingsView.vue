<script setup lang="ts">
import { ref, onMounted, reactive } from 'vue'
import { settingsApi } from '@/api'
import { showToast } from '@/utils/toast'
import Button from 'primevue/button'
import InputText from 'primevue/inputtext'
import InputNumber from 'primevue/inputnumber'
import Select from 'primevue/select'
import Panel from 'primevue/panel'

const loading = ref(false)
const saving = ref(false)
const dbOverride = ref(false)

const formData = reactive({
  manager: {
    heartbeat: { timeout: 30, check_interval: 5, pending_timeout: 300 },
    dispatch_retry: { max_retries: 1, base_delay_sec: 2, max_delay_sec: 30 },
    history: { event_retention_days: 30, metric_retention_days: 7 },
  },
  logging: { level: 'info', format: 'json', output: 'stdout', log_dir: './logs', log_retention_days: 30, max_log_size_mb: 100 },
})

const levelOptions = [
  { label: 'Debug', value: 'debug' },
  { label: 'Info', value: 'info' },
  { label: 'Warn', value: 'warn' },
  { label: 'Error', value: 'error' },
]

const formatOptions = [
  { label: 'JSON', value: 'json' },
  { label: 'Console', value: 'console' },
]

const loadSettings = async () => {
  loading.value = true
  try {
    const res = await settingsApi.getSettings() as any
    const cfg = res.config || res
    dbOverride.value = res.db_override || false
    if (cfg.manager?.heartbeat) Object.assign(formData.manager.heartbeat, cfg.manager.heartbeat)
    if (cfg.manager?.dispatch_retry) Object.assign(formData.manager.dispatch_retry, cfg.manager.dispatch_retry)
    if (cfg.manager?.history) Object.assign(formData.manager.history, cfg.manager.history)
    if (cfg.logging) {
      Object.assign(formData.logging, cfg.logging)
    }
  } catch (e: any) {
    showToast({ severity: 'error', summary: '加载配置失败', detail: e.response?.data?.error || '加载失败', life: 3000 })
  } finally {
    loading.value = false
  }
}

const saveSettings = async () => {
  saving.value = true
  try {
    const payload = {
      manager: {
        heartbeat: { ...formData.manager.heartbeat },
        dispatch_retry: { ...formData.manager.dispatch_retry },
        history: { ...formData.manager.history },
      },
      logging: {
        level: formData.logging.level,
        format: formData.logging.format,
        output: formData.logging.output,
        log_retention_days: formData.logging.log_retention_days,
        max_log_size_mb: formData.logging.max_log_size_mb,
      },
    }
    await settingsApi.updateSettings(payload)
    dbOverride.value = true
    showToast({ severity: 'success', summary: '配置已保存', detail: '重启 Manager 后生效', life: 5000 })
  } catch (e: any) {
    showToast({ severity: 'error', summary: '保存失败', detail: e.response?.data?.error || '保存失败', life: 3000 })
  } finally {
    saving.value = false
  }
}

onMounted(() => loadSettings())
</script>

<template>
  <div class="settings-page">
    <div class="flex items-center justify-between mb-4">
      <div>
        <h3 class="text-lg font-semibold text-gray-800">系统配置</h3>
        <p class="text-sm text-gray-500 mt-1">修改后需重启 Manager 才能生效</p>
      </div>
      <div class="flex items-center gap-3">
        <span v-if="dbOverride" class="text-xs px-2 py-1 rounded bg-blue-50 text-blue-600">
          <i class="pi pi-database mr-1"></i>数据库配置已生效
        </span>
        <Button label="保存配置" icon="pi pi-save" :loading="saving" @click="saveSettings" />
      </div>
    </div>

    <div v-if="loading" class="text-center py-8 text-gray-400">加载中...</div>

    <div v-else class="space-y-4">
      <!-- 心跳检测 -->
      <Panel toggleable>
        <template #header>
          <div>
            <span class="font-semibold">心跳检测</span>
            <p class="text-xs text-gray-400 mt-0.5">Manager 定期检测 Worker 节点存活状态，超时未响应视为离线</p>
          </div>
        </template>
        <div class="grid grid-cols-1 md:grid-cols-3 gap-4">
          <div class="flex flex-col gap-2">
            <label class="text-sm font-medium text-gray-600">节点超时（秒）</label>
            <InputNumber v-model="formData.manager.heartbeat.timeout" :min="5" />
            <span class="text-xs text-gray-400">超过此时间未收到心跳，节点标记为离线</span>
          </div>
          <div class="flex flex-col gap-2">
            <label class="text-sm font-medium text-gray-600">检查间隔（秒）</label>
            <InputNumber v-model="formData.manager.heartbeat.check_interval" :min="1" />
            <span class="text-xs text-gray-400">Manager 每隔多久执行一次健康检查</span>
          </div>
          <div class="flex flex-col gap-2">
            <label class="text-sm font-medium text-gray-600">Pending 超时（秒）</label>
            <InputNumber v-model="formData.manager.heartbeat.pending_timeout" :min="10" />
            <span class="text-xs text-gray-400">任务处于 pending 状态超过此时间未分发则标记为失败</span>
          </div>
        </div>
      </Panel>

      <!-- 分发重试 -->
      <Panel toggleable>
        <template #header>
          <div>
            <span class="font-semibold">分发重试</span>
            <p class="text-xs text-gray-400 mt-0.5">任务分发失败时的重试策略，采用指数退避算法</p>
          </div>
        </template>
        <div class="grid grid-cols-1 md:grid-cols-3 gap-4">
          <div class="flex flex-col gap-2">
            <label class="text-sm font-medium text-gray-600">最大重试次数</label>
            <InputNumber v-model="formData.manager.dispatch_retry.max_retries" :min="0" :max="10" />
            <span class="text-xs text-gray-400">分发失败后的最大重试次数，0 表示不重试</span>
          </div>
          <div class="flex flex-col gap-2">
            <label class="text-sm font-medium text-gray-600">基础退避（秒）</label>
            <InputNumber v-model="formData.manager.dispatch_retry.base_delay_sec" :min="1" />
            <span class="text-xs text-gray-400">首次重试的等待时间</span>
          </div>
          <div class="flex flex-col gap-2">
            <label class="text-sm font-medium text-gray-600">退避上限（秒）</label>
            <InputNumber v-model="formData.manager.dispatch_retry.max_delay_sec" :min="1" />
            <span class="text-xs text-gray-400">实际延迟 = min(基础退避 × 2^重试次数, 退避上限)</span>
          </div>
        </div>
      </Panel>

      <!-- 数据保留 -->
      <Panel toggleable>
        <template #header>
          <div>
            <span class="font-semibold">数据保留</span>
            <p class="text-xs text-gray-400 mt-0.5">历史数据的自动清理策略，每日凌晨 3:00 执行清理</p>
          </div>
        </template>
        <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
          <div class="flex flex-col gap-2">
            <label class="text-sm font-medium text-gray-600">事件保留天数</label>
            <InputNumber v-model="formData.manager.history.event_retention_days" :min="1" />
            <span class="text-xs text-gray-400">任务执行记录的保留天数，超期自动删除</span>
          </div>
          <div class="flex flex-col gap-2">
            <label class="text-sm font-medium text-gray-600">指标保留天数</label>
            <InputNumber v-model="formData.manager.history.metric_retention_days" :min="1" />
            <span class="text-xs text-gray-400">节点负载指标的保留天数，超期自动删除</span>
          </div>
        </div>
      </Panel>

      <!-- 日志配置 -->
      <Panel toggleable>
        <template #header>
          <div>
            <span class="font-semibold">日志配置</span>
            <p class="text-xs text-gray-400 mt-0.5">应用日志与任务执行日志的输出和清理策略</p>
          </div>
        </template>
        <div class="grid grid-cols-1 md:grid-cols-3 gap-4">
          <div class="flex flex-col gap-2">
            <label class="text-sm font-medium text-gray-600">日志级别</label>
            <Select v-model="formData.logging.level" :options="levelOptions" optionLabel="label" optionValue="value" />
            <span class="text-xs text-gray-400">仅输出该级别及以上的日志</span>
          </div>
          <div class="flex flex-col gap-2">
            <label class="text-sm font-medium text-gray-600">日志格式</label>
            <Select v-model="formData.logging.format" :options="formatOptions" optionLabel="label" optionValue="value" />
            <span class="text-xs text-gray-400">JSON 适合日志采集，Console 适合终端查看</span>
          </div>
          <div class="flex flex-col gap-2">
            <label class="text-sm font-medium text-gray-600">输出目标</label>
            <InputText v-model="formData.logging.output" disabled />
            <span class="text-xs text-gray-400">应用日志输出目标（stdout 或文件路径）</span>
          </div>
          <div class="flex flex-col gap-2">
            <label class="text-sm font-medium text-gray-600">日志目录</label>
            <InputText v-model="formData.logging.log_dir" disabled />
            <span class="text-xs text-gray-400">任务执行日志的存储目录</span>
          </div>
          <div class="flex flex-col gap-2">
            <label class="text-sm font-medium text-gray-600">日志保留天数</label>
            <InputNumber v-model="formData.logging.log_retention_days" :min="1" />
            <span class="text-xs text-gray-400">任务执行日志文件的保留天数，超期自动删除</span>
          </div>
          <div class="flex flex-col gap-2">
            <label class="text-sm font-medium text-gray-600">日志文件上限（MB）</label>
            <InputNumber v-model="formData.logging.max_log_size_mb" :min="1" />
            <span class="text-xs text-gray-400">单个日志文件超过此大小会被截断</span>
          </div>
        </div>
      </Panel>
    </div>
  </div>
</template>

<style scoped>
.settings-page {
  max-width: 960px;
}

:deep(.p-panel-header) {
  padding: 0.75rem 1rem !important;
}

:deep(.p-panel-content) {
  padding: 1rem !important;
}
</style>
