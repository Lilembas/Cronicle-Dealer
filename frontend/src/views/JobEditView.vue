<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { jobsApi, nodesApi, type Node } from '@/api'
import { ArrowLeft, Plus, Delete, QuestionFilled } from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'
import { VueCodemirror as Codemirror } from 'codemirror-editor-vue3'
import 'codemirror/addon/display/placeholder.js'
import 'codemirror/mode/shell/shell.js'

const router = useRouter()
const route = useRoute()

// 表单数据
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
  target_type: '',
  target_value: '',
  strict_mode: false,
})

// CodeMirror 配置
const cmOptions = {
  mode: 'text/x-sh',
  theme: 'default',
  lineNumbers: false,
  lineWrapping: true,
  tabSize: 2,
}

// 常用分组列表
const commonGroups = ['默认分组', '系统任务', '数据同步', '定时清理', '监控告警', '数据备份']

// 获取可用分组（从已有任务中获取）
const availableGroups = ref<string[]>([...commonGroups])

const loadGroups = async () => {
  try {
    const result = await jobsApi.list({ page: 1, page_size: 100 }) as unknown as { data: any[] }
    const groups = new Set<string>()
    if (result?.data) {
      result.data.forEach((job: any) => {
        if (job.category) groups.add(job.category)
      })
    }
    // 合并预置分组和已有分组
    const allGroups = new Set([...commonGroups, ...groups])
    availableGroups.value = Array.from(allGroups).sort()
  } catch {
    // 加载失败使用默认分组
    availableGroups.value = [...commonGroups]
  }
}

// 节点列表
const nodes = ref<Node[]>([])
const loadingNodes = ref(false)

// 可用标签列表（从节点标签中获取）
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

// Cron表达式构建器
const cron = ref({
  minute: '*',
  hour: '*',
  dayOfMonth: '*',
  month: '*',
  dayOfWeek: '*',
})

// 预设Cron表达式
const cronPresets = [
  { label: '每分钟', value: '* * * * *' },
  { label: '每小时', value: '0 * * * *' },
  { label: '每天0点', value: '0 0 * * *' },
  { label: '每周一0点', value: '0 0 * * 1' },
  { label: '每月1号0点', value: '0 0 1 * *' },
  { label: '每5分钟', value: '*/5 * * * *' },
  { label: '每30分钟', value: '*/30 * * * *' },
]

// 是否为编辑模式
const isEdit = computed(() => !!route.params.id)
const title = computed(() => isEdit.value ? '编辑任务' : '新建任务')

// 加载节点列表（过滤掉 master 节点）
const loadNodes = async () => {
  try {
    loadingNodes.value = true
    const allNodes = await nodesApi.list({ status: 'online' }) as unknown as Node[]
    nodes.value = (allNodes || []).filter((node: Node) => {
      // 路由节点不作为执行节点
      const tagsStr = String(node.tags || '')
      return !tagsStr.includes('master')
    })
    console.log('Nodes loaded:', nodes.value.length)
  } catch (error) {
    console.error('加载节点列表失败:', error)
    ElMessage.warning('加载节点列表失败')
  } finally {
    loadingNodes.value = false
  }
}

// 加载所有可用标签
const loadTags = async () => {
  try {
    loadingTags.value = true
    const tags = await nodesApi.listTags() as unknown as string[]
    availableTags.value = tags || []
  } catch (error) {
    console.error('加载标签列表失败:', error)
  } finally {
    loadingTags.value = false
  }
}

// 加载任务数据
const loadJob = async () => {
  if (!isEdit.value) return

  try {
    const job = await jobsApi.get(route.params.id as string) as any
    // 处理后端可能返回的 tags 字段
    let tags: string[] = []
    if (job.tags) {
      if (typeof job.tags === 'string') {
        tags = job.tags.split(',').filter(Boolean)
      } else if (Array.isArray(job.tags)) {
        tags = job.tags
      }
    }

    // 处理 target_value 如果是 JSON 数组
    let targetValue = job.target_value
    if (job.target_type === 'tags' && targetValue) {
      try {
        const parsed = JSON.parse(targetValue)
        if (Array.isArray(parsed)) {
          // 如果后端存储的是 JSON 数组，将其转换为逗号分隔或直接给 tags
          tags = parsed
        }
      } catch (e) {
        // 不是 JSON，按普通字符串处理
      }
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
      target_type: job.target_type || 'any',
      target_value: String(targetValue || ''),
      strict_mode: job.strict_mode || false,
    }

    // 解析Cron表达式
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
  } catch (error) {
    ElMessage.error('加载任务失败')
    router.back()
  }
}

// 构建Cron表达式
const buildCronExpr = () => {
  formData.value.cron_expr = [
    cron.value.minute,
    cron.value.hour,
    cron.value.dayOfMonth,
    cron.value.month,
    cron.value.dayOfWeek,
  ].join(' ')
}

// 从表达式更新Cron组件
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

// 选择预设Cron
const selectPreset = (value: string) => {
  formData.value.cron_expr = value
  updateCronFromExpr(value)
}

// 切换运行方式时清空已选值
const onTargetTypeChange = () => {
  formData.value.target_value = ''
  formData.value.tags = []
}

// 添加环境变量
const addEnv = () => {
  formData.value.env.push({ key: '', value: '' })
}

// 删除环境变量
const removeEnv = (index: number) => {
  formData.value.env.splice(index, 1)
}

// 保存任务
const save = async () => {
  // 验证表单
  if (!formData.value.name.trim()) {
    ElMessage.warning('请输入任务名称')
    return
  }
  if (!formData.value.cron_expr.trim()) {
    ElMessage.warning('请输入Cron表达式')
    return
  }
  if (!formData.value.command.trim()) {
    ElMessage.warning('请输入命令')
    return
  }

  // 验证目标服务器配置
  if (formData.value.target_type === 'node_id' && !formData.value.target_value) {
    ElMessage.warning('请选择执行服务器')
    return
  }
  if (formData.value.target_type === 'tags' && formData.value.tags.length === 0) {
    ElMessage.warning('请至少添加一个标签')
    return
  }

  try {
    // 转换环境变量格式
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
      // 如果按标签选择，target_value 存储标签的 JSON 数组
      target_value: formData.value.target_type === 'tags' 
        ? JSON.stringify(formData.value.tags) 
        : formData.value.target_value,
      strict_mode: formData.value.strict_mode,
    }

    if (isEdit.value) {
      await jobsApi.update(route.params.id as string, payload)
      ElMessage.success('保存成功')
    } else {
      await jobsApi.create(payload)
      ElMessage.success('创建成功')
    }
    router.back()
  } catch (error: any) {
    console.error('保存任务失败:', error)
    ElMessage.error(error.response?.data?.error || '保存失败')
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

  // 如果没有分组，设置默认值
  if (!formData.value.category) {
    formData.value.category = '默认分组'
  }
})
</script>

<template>
  <div class="job-edit">
    <div class="page-header">
      <el-button :icon="ArrowLeft" @click="cancel" class="back-btn">返回</el-button>
      <h2 class="page-title">{{ title }}</h2>
    </div>

    <el-card class="form-card">
      <el-form label-width="100px" label-position="right" class="compact-form">
        <!-- 基本信息 -->
        <div class="form-section">
          <h3 class="section-title">基本信息</h3>

          <el-row :gutter="16">
            <el-col :span="14">
              <el-form-item label="任务名称" required>
                <el-input
                  v-model="formData.name"
                  placeholder="输入任务名称"
                  maxlength="100"
                  show-word-limit
                />
              </el-form-item>
            </el-col>
            <el-col :span="10">
              <el-form-item label="任务分组">
                <el-select
                  v-model="formData.category"
                  filterable
                  allow-create
                  default-first-option
                  placeholder="选择或输入分组"
                  style="width: 100%"
                >
                  <el-option
                    v-for="group in availableGroups"
                    :key="group"
                    :label="group"
                    :value="group"
                  />
                </el-select>
              </el-form-item>
            </el-col>
          </el-row>

          <el-form-item label="任务描述">
            <el-input
              v-model="formData.description"
              type="textarea"
              placeholder="简要说明此任务的功能和用途..."
              :rows="2"
              maxlength="500"
              show-word-limit
            />
          </el-form-item>

        </div>

        <!-- Cron表达式 -->
        <div class="form-section">
          <div class="section-header">
            <h3 class="section-title">调度规则</h3>
            <el-switch v-model="formData.enabled" active-text="启用任务" />
          </div>

          <el-form-item label="快速预设">
            <el-radio-group v-model="formData.cron_expr" @change="selectPreset" size="small">
              <el-radio-button v-for="preset in cronPresets" :key="preset.value" :value="preset.value">
                {{ preset.label }}
              </el-radio-button>
            </el-radio-group>
          </el-form-item>

          <el-form-item label="自定义规则">
            <div class="cron-builder">
              <div class="cron-grid">
                <div class="cron-part">
                  <span class="part-label">分</span>
                  <el-input v-model="cron.minute" @input="buildCronExpr" placeholder="*" size="small" />
                </div>
                <div class="cron-part">
                  <span class="part-label">时</span>
                  <el-input v-model="cron.hour" @input="buildCronExpr" placeholder="*" size="small" />
                </div>
                <div class="cron-part">
                  <span class="part-label">日</span>
                  <el-input v-model="cron.dayOfMonth" @input="buildCronExpr" placeholder="*" size="small" />
                </div>
                <div class="cron-part">
                  <span class="part-label">月</span>
                  <el-input v-model="cron.month" @input="buildCronExpr" placeholder="*" size="small" />
                </div>
                <div class="cron-part">
                  <span class="part-label">周</span>
                  <el-input v-model="cron.dayOfWeek" @input="buildCronExpr" placeholder="*" size="small" />
                </div>
              </div>
              <div class="cron-raw">
                <span class="raw-label">表达式:</span>
                <el-input v-model="formData.cron_expr" placeholder="* * * * *" @input="updateCronFromExpr" size="small" class="raw-input" />
              </div>
            </div>
            <div class="field-hint" style="margin-top: 8px">
              格式：分 时 日 月 周 (5位标准版)。使用 * 表示通配，*/N 表示频率。
            </div>
          </el-form-item>
        </div>

        <!-- 执行配置 -->
        <div class="form-section">
          <h3 class="section-title">执行配置</h3>

          <el-form-item label="运行节点" required>
            <div class="target-row">
              <el-radio-group v-model="formData.target_type" size="small" class="target-radio" @change="onTargetTypeChange">
                <el-radio-button value="node_id">指定节点</el-radio-button>
                <el-radio-button value="tags">匹配标签</el-radio-button>
              </el-radio-group>
              
              <el-select
                v-if="formData.target_type === 'node_id'"
                v-model="formData.target_value"
                placeholder="选择执行节点"
                :loading="loadingNodes"
                class="target-select"
              >
                <el-option
                  v-for="node in nodes"
                  :key="node.id"
                  :label="`${node.hostname} (${node.ip})`"
                  :value="node.id"
                />
              </el-select>

              <el-select
                v-else
                v-model="formData.tags"
                multiple
                filterable
                placeholder="选择匹配标签"
                :loading="loadingTags"
                class="target-select"
              >
                <el-option
                  v-for="tag in availableTags"
                  :key="tag"
                  :label="tag"
                  :value="tag"
                />
              </el-select>
            </div>
          </el-form-item>

          <el-row :gutter="24">
            <el-col :span="10">
              <el-form-item label="超时限制 (s)">
                <el-input-number
                  v-model="formData.timeout"
                  :min="1"
                  :max="86400"
                  :step="60"
                  controls-position="right"
                  style="width: 100%"
                />
              </el-form-item>
            </el-col>
            <el-col :span="14">
              <el-form-item label="严格模式">
                <div class="strict-wrap">
                  <el-switch v-model="formData.strict_mode" />
                  <el-tooltip placement="top">
                    <template #content>
                      ✓ 启用：命令失败立即终止<br/>
                      ✗ 禁用：继续执行后续命令
                    </template>
                    <el-icon class="help-mini"><QuestionFilled /></el-icon>
                  </el-tooltip>
                  <span class="field-hint">开启后脚本遇错即停</span>
                </div>
              </el-form-item>
            </el-col>
          </el-row>

          <el-form-item label="执行脚本" required>
            <div class="command-editor-wrapper">
              <Codemirror
                v-model:value="formData.command"
                :options="cmOptions"
                :placeholder="'输入 Shell 脚本...'"
                :height="'240px'"
              />
            </div>
          </el-form-item>

          <el-form-item label="环境变量">
            <div class="env-container">
              <div v-for="(envVar, index) in formData.env" :key="index" class="env-row">
                <el-input v-model="envVar.key" placeholder="Key" style="width: 180px" />
                <span class="env-eq">=</span>
                <el-input v-model="envVar.value" placeholder="Value" style="flex: 1" />
                <el-button :icon="Delete" type="danger" link @click="removeEnv(index)" />
              </div>
              <el-button :icon="Plus" text type="primary" size="small" @click="addEnv">添加变量</el-button>
            </div>
          </el-form-item>
        </div>

        <!-- 操作按钮 -->
        <div class="form-actions">
          <el-button @click="cancel">取消</el-button>
          <el-button type="primary" @click="save">保存</el-button>
        </div>
      </el-form>
    </el-card>
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

.form-section {
  padding: 16px 0;
  border-bottom: 1px solid #f1f5f9;
}

.form-section :deep(.el-form-item) {
  margin-bottom: 16px;
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

.node-option {
  display: flex;
  justify-content: space-between;
  width: 100%;
}

.node-ip {
  color: #94a3b8;
  font-size: 11px;
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

.help-icon {
  margin-left: 8px;
  vertical-align: middle;
  color: #94a3b8;
  cursor: help;
  font-size: 16px;
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
