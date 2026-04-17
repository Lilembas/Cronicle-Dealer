<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { jobsApi, nodesApi, type Node } from '@/api'
import { showToast } from '@/utils/toast'
import InputText from 'primevue/inputtext'
import Textarea from 'primevue/textarea'
import InputNumber from 'primevue/inputnumber'
import ToggleSwitch from 'primevue/toggleswitch'
import Select from 'primevue/select'
import SelectButton from 'primevue/selectbutton'
import Button from 'primevue/button'
import Card from 'primevue/card'
import { VueCodemirror as Codemirror } from 'codemirror-editor-vue3'
import 'codemirror/addon/display/placeholder.js'
import 'codemirror/mode/shell/shell.js'

const router = useRouter()
const route = useRoute()

const formData = ref({
  name: '',
  description: '',
  category: '',
  cron_expr: '',
  command: '',
  timeout: 3600,
  enabled: true,
  env: [] as Array<{ key: string; value: string }>,
  tags: [] as string[],
  target_type: 'tags',
  target_value: '',
  strict_mode: false,
})

const cmOptions = {
  mode: 'text/x-sh',
  theme: 'default',
  lineNumbers: false,
  lineWrapping: true,
  tabSize: 2,
}

const commonGroups = ['默认分组', '系统任务', '数据同步', '定时清理', '监控告警', '数据备份']
const availableGroups = ref<string[]>([...commonGroups])

const loadGroups = async () => {
  try {
    const result = await jobsApi.list({ page: 1, page_size: 100 }) as unknown as { data: any[] }
    const groups = result?.data?.map((job: any) => job.category).filter(Boolean) || []
    availableGroups.value = Array.from(new Set([...commonGroups, ...groups])).sort()
  } catch {
    availableGroups.value = [...commonGroups]
  }
}

const nodes = ref<Node[]>([])
const loadingNodes = ref(false)
const availableTags = ref<string[]>([])
const loadingTags = ref(false)

const parseEnvString = (value: unknown): Array<{ key: string; value: string }> => {
  if (!value) return []
  if (typeof value !== 'string') return []
  try {
    const parsed = JSON.parse(value) as Record<string, string>
    return Object.entries(parsed).map(([key, v]) => ({ key, value: String(v) }))
  } catch {
    return []
  }
}

const cron = ref({
  minute: '*',
  hour: '*',
  dayOfMonth: '*',
  month: '*',
  dayOfWeek: '*',
})

const cronPresets = [
  { label: '每分钟', value: '* * * * *' },
  { label: '每小时', value: '0 * * * *' },
  { label: '每天0点', value: '0 0 * * *' },
  { label: '每周一0点', value: '0 0 * * 1' },
  { label: '每月1号0点', value: '0 0 1 * *' },
  { label: '每5分钟', value: '*/5 * * * *' },
  { label: '每30分钟', value: '*/30 * * * *' },
]

const isEdit = computed(() => !!route.params.id)
const title = computed(() => isEdit.value ? '编辑任务' : '新建任务')

// Target type options for SelectButton
const targetTypeOptions = [
  { label: '指定节点', value: 'node_id' },
  { label: '匹配标签', value: 'tags' },
]

const loadNodes = async () => {
  try {
    loadingNodes.value = true
    const allNodes = await nodesApi.list({ status: 'online' }) as unknown as Node[]
    nodes.value = (allNodes || []).filter((node: Node) =>
      !String(node.tags || '').includes('master')
    )
  } catch {
    showToast({ severity: 'warn', summary: '加载节点列表失败', life: 3000 })
  } finally {
    loadingNodes.value = false
  }
}

const loadTags = async () => {
  try {
    loadingTags.value = true
    const tags = await nodesApi.listTags() as unknown as string[]
    availableTags.value = tags || []
  } catch {
  } finally {
    loadingTags.value = false
  }
}

const loadJob = async () => {
  if (!isEdit.value) return

  try {
    const job = await jobsApi.get(route.params.id as string) as any
    let tags: string[] = Array.isArray(job.tags) ? job.tags : (typeof job.tags === 'string' ? job.tags.split(',').filter(Boolean) : [])

    let targetValue = job.target_value
    if (job.target_type === 'tags' && targetValue) {
      try {
        const parsed = JSON.parse(targetValue)
        if (Array.isArray(parsed)) {
          tags = parsed
        }
      } catch {}
    }

    formData.value = {
      name: job.name,
      description: job.description || '',
      category: job.category || '默认分组',
      cron_expr: job.cron_expr,
      command: job.command,
      timeout: job.timeout,
      enabled: job.enabled,
      env: parseEnvString(job.env),
      tags: tags,
      target_type: job.target_type === 'node_id' ? 'node_id' : 'tags',
      target_value: String(targetValue || ''),
      strict_mode: job.strict_mode || false,
    }

    const parts = job.cron_expr.split(/\s+/)
    if (parts.length === 5) {
      cron.value = {
        minute: parts[0],
        hour: parts[1],
        dayOfMonth: parts[2],
        month: parts[3],
        dayOfWeek: parts[4],
      }
    }
  } catch {
    showToast({ severity: 'error', summary: '加载任务失败', life: 5000 })
    router.back()
  }
}

const buildCronExpr = () => {
  formData.value.cron_expr = [
    cron.value.minute,
    cron.value.hour,
    cron.value.dayOfMonth,
    cron.value.month,
    cron.value.dayOfWeek,
  ].join(' ')
  validateCronExpr()
}

const updateCronFromExpr = (value: string) => {
  const parts = value.split(/\s+/)
  if (parts.length === 5) {
    cron.value = {
      minute: parts[0],
      hour: parts[1],
      dayOfMonth: parts[2],
      month: parts[3],
      dayOfWeek: parts[4],
    }
  }
}

const selectPreset = (value: string) => {
  formData.value.cron_expr = value
  updateCronFromExpr(value)
  validateCronExpr()
}

const cronError = ref('')

const validateCronField = (value: string, min: number, max: number): boolean => {
  if (value === '*') return true
  if (value.includes(',')) {
    return value.split(',').every(part => validateCronField(part.trim(), min, max))
  }
  if (/^\*\/\d+$/.test(value)) {
    const n = parseInt(value.split('/')[1])
    return n >= 1
  }
  if (/^\d+\/\d+$/.test(value)) {
    const [start, step] = value.split('/').map(Number)
    return start >= min && start <= max && step >= 1
  }
  if (/^\d+-\d+(\/\d+)?$/.test(value)) {
    const [range, step] = value.split('/')
    const [a, b] = range.split('-').map(Number)
    if (a < min || b > max || a > b) return false
    if (step !== undefined && parseInt(step) < 1) return false
    return true
  }
  if (/^\d+$/.test(value)) {
    const n = parseInt(value)
    return n >= min && n <= max
  }
  return false
}

const validateCronExpr = (): boolean => {
  const expr = formData.value.cron_expr.trim()
  if (!expr) {
    cronError.value = '请输入Cron表达式'
    return false
  }
  const parts = expr.split(/\s+/)
  if (parts.length !== 5) {
    cronError.value = 'Cron表达式须包含5个字段（分 时 日 月 周）'
    return false
  }
  const [minute, hour, dom, month, dow] = parts
  if (!validateCronField(minute, 0, 59)) { cronError.value = '「分」字段无效（范围 0-59）'; return false }
  if (!validateCronField(hour, 0, 23)) { cronError.value = '「时」字段无效（范围 0-23）'; return false }
  if (!validateCronField(dom, 1, 31)) { cronError.value = '「日」字段无效（范围 1-31）'; return false }
  if (!validateCronField(month, 1, 12)) { cronError.value = '「月」字段无效（范围 1-12）'; return false }
  if (!validateCronField(dow, 0, 7)) { cronError.value = '「周」字段无效（范围 0-7）'; return false }
  cronError.value = ''
  return true
}

const onTargetTypeChange = () => {
  formData.value.target_value = ''
  formData.value.tags = []
}

const addEnv = () => {
  formData.value.env.push({ key: '', value: '' })
}

const removeEnv = (index: number) => {
  formData.value.env.splice(index, 1)
}

const save = async () => {
  if (!formData.value.name.trim()) {
    showToast({ severity: 'warn', summary: '请输入任务名称', life: 3000 })
    return
  }
  if (!validateCronExpr()) {
    showToast({ severity: 'warn', summary: cronError.value, life: 3000 })
    return
  }
  if (!formData.value.command.trim()) {
    showToast({ severity: 'warn', summary: '请输入命令', life: 3000 })
    return
  }

  if (!formData.value.target_type) {
    showToast({ severity: 'warn', summary: '请选择运行节点方式', life: 3000 })
    return
  }
  if (formData.value.target_type === 'node_id' && !formData.value.target_value) {
    showToast({ severity: 'warn', summary: '请选择执行服务器', life: 3000 })
    return
  }
  if (formData.value.target_type === 'tags' && formData.value.tags.length === 0) {
    showToast({ severity: 'warn', summary: '请至少添加一个标签', life: 3000 })
    return
  }

  try {
    const env: Record<string, string> = {}
    formData.value.env.forEach(({ key, value }) => {
      if (key.trim()) {
        env[key.trim()] = value
      }
    })

    const payload = {
      name: formData.value.name,
      description: formData.value.description,
      category: formData.value.category,
      cron_expr: formData.value.cron_expr,
      command: formData.value.command,
      timeout: Number(formData.value.timeout),
      enabled: formData.value.enabled,
      env: JSON.stringify(env),
      target_type: formData.value.target_type,
      target_value: formData.value.target_type === 'tags'
        ? JSON.stringify(formData.value.tags)
        : formData.value.target_value,
      strict_mode: formData.value.strict_mode,
    }

    if (isEdit.value) {
      await jobsApi.update(route.params.id as string, payload)
      showToast({ severity: 'success', summary: '保存成功', life: 3000 })
    } else {
      await jobsApi.create(payload)
      showToast({ severity: 'success', summary: '创建成功', life: 3000 })
    }
    router.back()
  } catch (error: any) {
    showToast({ severity: 'error', summary: '保存失败', detail: error.response?.data?.error || '保存失败', life: 5000 })
  }
}

const cancel = () => {
  router.back()
}

onMounted(() => {
  loadGroups()
  loadNodes()
  loadTags()
  loadJob()

  if (!formData.value.category) {
    formData.value.category = '默认分组'
  }
})
</script>

<template>
  <div class="job-edit">
    <div class="page-header">
      <Button icon="pi pi-arrow-left" text @click="cancel" class="back-btn" label="返回" />
      <h2 class="page-title">{{ title }}</h2>
    </div>

    <Card class="form-card">
      <template #content>
        <div class="form-content">
          <!-- 基本信息 -->
          <div class="form-section">
            <h3 class="section-title">基本信息</h3>

            <div class="grid grid-cols-24 gap-4 mb-4">
              <div class="col-span-14">
                <div class="flex flex-col gap-1">
                  <label class="font-medium text-sm">任务名称 <span class="text-red-500">*</span></label>
                  <InputText v-model="formData.name" placeholder="输入任务名称" class="w-full" />
                </div>
              </div>
              <div class="col-span-10">
                <div class="flex flex-col gap-1">
                  <label class="font-medium text-sm">任务分组</label>
                  <Select v-model="formData.category" :options="availableGroups" filterable editable placeholder="选择或输入分组" class="w-full" />
                </div>
              </div>
            </div>

            <div class="flex flex-col gap-1 mb-4">
              <label class="font-medium text-sm">任务描述</label>
              <Textarea v-model="formData.description" placeholder="简要说明此任务的功能和用途..." rows="2" autoResize class="w-full" />
            </div>
          </div>

          <!-- Cron表达式 -->
          <div class="form-section">
            <div class="section-header">
              <h3 class="section-title">调度规则</h3>
              <div class="flex items-center gap-2">
                <ToggleSwitch v-model="formData.enabled" />
                <span class="text-sm">启用任务</span>
              </div>
            </div>

            <div class="flex flex-col gap-1 mb-4">
              <label class="font-medium text-sm">快速预设</label>
              <SelectButton
                v-model="formData.cron_expr"
                :options="cronPresets"
                optionLabel="label"
                optionValue="value"
                @change="(e: any) => selectPreset(e.value)"
              />
            </div>

            <div class="flex flex-col gap-1 mb-2">
              <label class="font-medium text-sm">自定义规则</label>
              <div class="cron-builder">
                <div class="cron-grid">
                  <div class="cron-part">
                    <span class="part-label">分</span>
                    <InputText v-model="cron.minute" @input="buildCronExpr" placeholder="*" size="small" class="w-full text-center" />
                  </div>
                  <div class="cron-part">
                    <span class="part-label">时</span>
                    <InputText v-model="cron.hour" @input="buildCronExpr" placeholder="*" size="small" class="w-full text-center" />
                  </div>
                  <div class="cron-part">
                    <span class="part-label">日</span>
                    <InputText v-model="cron.dayOfMonth" @input="buildCronExpr" placeholder="*" size="small" class="w-full text-center" />
                  </div>
                  <div class="cron-part">
                    <span class="part-label">月</span>
                    <InputText v-model="cron.month" @input="buildCronExpr" placeholder="*" size="small" class="w-full text-center" />
                  </div>
                  <div class="cron-part">
                    <span class="part-label">周</span>
                    <InputText v-model="cron.dayOfWeek" @input="buildCronExpr" placeholder="*" size="small" class="w-full text-center" />
                  </div>
                </div>
                <div class="cron-raw">
                  <span class="raw-label">表达式:</span>
                  <InputText :modelValue="formData.cron_expr" placeholder="* * * * *" size="small" class="raw-input" readonly />
                </div>
              </div>
              <div v-if="cronError" class="cron-error mt-2">
                {{ cronError }}
              </div>
              <div v-else class="field-hint mt-2">
                格式：分 时 日 月 周 (5位标准版)。使用 * 表示通配，*/N 表示频率。
              </div>
            </div>
          </div>

          <!-- 执行配置 -->
          <div class="form-section">
            <h3 class="section-title">执行配置</h3>

            <div class="flex flex-col gap-1 mb-4">
              <label class="font-medium text-sm">运行节点 <span class="text-red-500">*</span></label>
              <div class="target-row">
                <SelectButton
                  v-model="formData.target_type"
                  :options="targetTypeOptions"
                  optionLabel="label"
                  optionValue="value"
                  @change="onTargetTypeChange"
                  class="target-radio"
                />

                <Select
                  v-if="formData.target_type === 'node_id'"
                  v-model="formData.target_value"
                  :options="nodes"
                  optionLabel="hostname"
                  optionValue="id"
                  placeholder="选择执行节点"
                  :loading="loadingNodes"
                  class="target-select flex-1"
                >
                  <template #value="{ value }">
                    <span v-if="value">{{ nodes.find(n => n.id === value)?.hostname }} ({{ nodes.find(n => n.id === value)?.ip }})</span>
                  </template>
                  <template #option="{ option }">
                    <div class="flex justify-between items-center w-full">
                      <span class="font-medium">{{ option.hostname }}</span>
                      <span class="text-gray-400 text-xs ml-2">{{ option.ip }}</span>
                    </div>
                  </template>
                </Select>

                <Select
                  v-else
                  v-model="formData.tags"
                  :options="availableTags"
                  multiple
                  filterable
                  placeholder="选择匹配标签"
                  :loading="loadingTags"
                  class="target-select flex-1"
                />
              </div>
            </div>

            <div class="grid grid-cols-24 gap-6 mb-4">
              <div class="col-span-10">
                <div class="flex flex-col gap-1">
                  <label class="font-medium text-sm">超时限制 (s)</label>
                  <InputNumber v-model="formData.timeout" :min="1" :max="86400" :step="60" showButtons buttonLayout="horizontal" incrementButtonIcon="pi pi-plus" decrementButtonIcon="pi pi-minus" class="w-full" />
                </div>
              </div>
              <div class="col-span-14">
                <div class="flex flex-col gap-1">
                  <label class="font-medium text-sm">严格模式</label>
                  <div class="strict-wrap">
                    <ToggleSwitch v-model="formData.strict_mode" />
                    <i class="pi pi-question-circle help-mini cursor-help" v-tooltip.top="'启用：命令失败立即终止\n禁用：继续执行后续命令'"></i>
                    <span class="field-hint">开启后脚本遇错即停</span>
                  </div>
                </div>
              </div>
            </div>

            <div class="flex flex-col gap-1 mb-4">
              <label class="font-medium text-sm">执行脚本 <span class="text-red-500">*</span></label>
              <div class="command-editor-wrapper">
                <Codemirror
                  v-model:value="formData.command"
                  :options="cmOptions"
                  :placeholder="'输入 Shell 脚本...'"
                  :height="'240px'"
                />
              </div>
            </div>

            <div class="flex flex-col gap-1 mb-4">
              <label class="font-medium text-sm">环境变量</label>
              <div class="env-container">
                <div v-for="(envVar, index) in formData.env" :key="index" class="env-row">
                  <InputText v-model="envVar.key" placeholder="Key" class="w-44" />
                  <span class="env-eq">=</span>
                  <InputText v-model="envVar.value" placeholder="Value" class="flex-1" />
                  <Button icon="pi pi-trash" severity="danger" text rounded size="small" @click="removeEnv(index)" />
                </div>
                <Button icon="pi pi-plus" severity="info" text size="small" @click="addEnv" label="添加变量" />
              </div>
            </div>
          </div>

          <!-- 操作按钮 -->
          <div class="form-actions">
            <Button severity="secondary" @click="cancel" label="取消" />
            <Button severity="info" @click="save" label="保存" />
          </div>
        </div>
      </template>
    </Card>
  </div>
</template>

<style scoped>
.job-edit {
  padding: 24px;
  max-width: 1000px;
  margin: 0 auto;
}

.page-header {
  display: flex;
  align-items: center;
  gap: 16px;
  margin-bottom: 24px;
}

.page-title {
  font-size: 24px;
  font-weight: 600;
  color: #1e293b;
  margin: 0;
}

.form-card {
  border-radius: 16px;
  border: 1px solid #e2e8f0;
}

.form-content {
  padding: 0;
}

.form-section {
  padding: 16px 0;
  border-bottom: 1px solid #f1f5f9;
}

.form-section:last-of-type {
  border-bottom: none;
}

.section-title {
  font-size: 16px;
  font-weight: 600;
  color: #1e293b;
  margin: 0 0 12px 0;
}

.field-hint {
  margin-left: 12px;
  font-size: 12px;
  color: #94a3b8;
}

.cron-error {
  margin-left: 12px;
  font-size: 12px;
  color: #f56c6c;
}

.env-row {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 8px;
}

.env-eq {
  color: #94a3b8;
  font-weight: bold;
}

.target-row {
  display: flex;
  gap: 12px;
  width: 100%;
}

.target-radio {
  flex-shrink: 0;
}

.target-select {
  flex: 1;
}

.strict-wrap {
  display: flex;
  align-items: center;
  gap: 8px;
  height: 32px;
}

.help-mini {
  color: #94a3b8;
  cursor: help;
  font-size: 14px;
}

.section-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 20px;
}

.section-header .section-title {
  margin-bottom: 0;
}

.cron-builder {
  background: #f8fafc;
  padding: 16px;
  border-radius: 8px;
  border: 1px solid #e2e8f0;
  width: 100%;
}

.cron-grid {
  display: flex;
  gap: 12px;
  margin-bottom: 12px;
}

.cron-part {
  flex: 1;
  display: flex;
  flex-direction: column;
  gap: 4px;
  align-items: center;
}

.part-label {
  font-size: 12px;
  color: #64748b;
  font-weight: 500;
}

.cron-raw {
  display: flex;
  align-items: center;
  gap: 8px;
  padding-top: 12px;
  border-top: 1px dashed #e2e8f0;
}

.raw-label {
  font-size: 12px;
  font-weight: 600;
  color: #475569;
}

.raw-input {
  width: 200px !important;
}

.form-actions {
  display: flex;
  justify-content: flex-end;
  gap: 12px;
  margin-top: 24px;
}

.command-editor-wrapper {
  width: 100%;
  display: block;
}

.command-editor-wrapper :deep(.CodeMirror) {
  border: 1px solid #c0c4cc !important;
  border-radius: 8px;
  background-color: #fff !important;
  width: 100% !important;
  height: 240px !important;
  font-family: 'Monaco', 'Menlo', 'Ubuntu Mono', 'Consolas', monospace;
  font-size: 14px;
  padding: 4px;
}
</style>
