<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { jobsApi, nodesApi, type Node } from '@/api'
import { ArrowLeft, Plus, Delete } from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'

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
  target_type: 'any',
  target_value: '',
  strict_mode: false,
})

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
  { label: '工作日9点', value: '0 9 * * 1-5' },
]

// 是否为编辑模式
const isEdit = computed(() => !!route.params.id)
const title = computed(() => isEdit.value ? '编辑任务' : '新建任务')

// 加载节点列表（过滤掉 master 节点）
const loadNodes = async () => {
  try {
    loadingNodes.value = true
    const allNodes = await nodesApi.list({ status: 'online' }) as unknown as Node[]
    // 过滤掉 master 节点
    nodes.value = (allNodes || []).filter((node: Node) => node.tags !== 'master' && !node.tags?.includes('master'))
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
    const job = await jobsApi.get(route.params.id as string)

    // 处理标签模式的数据加载
    let tags = job.tags || []
    let targetValue = job.target_value || ''

    if (job.target_type === 'tags' && targetValue) {
      // 如果是标签模式，将逗号分隔的字符串转换为数组
      tags = targetValue.split(',').map(tag => tag.trim()).filter(tag => tag)
      targetValue = ''
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
      target_value: targetValue,
      strict_mode: job.strict_mode || false,
    }

    // 解析Cron表达式
    const parts = job.cron_expr.split(' ')
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

// 选择预设Cron
const selectPreset = (value: string) => {
  formData.value.cron_expr = value
  const parts = value.split(' ')
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
      if (key) {
        env[key] = value
      }
    })

    // 准备提交数据
    const data: any = {
      ...formData.value,
      env: JSON.stringify(env),
    }

    // 处理目标服务器配置
    if (formData.value.target_type === 'tags') {
      // 标签模式：使用逗号分隔的标签字符串
      data.target_value = formData.value.tags.join(',')
    }
    // node_id 模式直接使用 target_value

    if (isEdit.value) {
      await jobsApi.update(route.params.id as string, data)
      ElMessage.success('更新成功')
    } else {
      await jobsApi.create(data)
      ElMessage.success('创建成功')
    }

    router.back()
  } catch (error: any) {
    ElMessage.error(error.response?.data?.error || '保存失败')
  }
}

// 取消
const cancel = () => {
  router.back()
}

onMounted(() => {
  loadJob()
  loadNodes()
  loadTags()
  loadGroups()
  // 新建模式设置默认分组
  if (!isEdit.value) {
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
      <el-form label-width="120px" label-position="left">
        <!-- 基本信息 -->
        <div class="form-section">
          <h3 class="section-title">基本信息</h3>

          <el-form-item label="任务名称" required>
            <el-input
              v-model="formData.name"
              placeholder="输入任务名称"
              maxlength="100"
              show-word-limit
            />
          </el-form-item>

          <el-form-item label="任务描述">
            <el-input
              v-model="formData.description"
              type="textarea"
              placeholder="输入任务描述"
              :rows="3"
              maxlength="500"
              show-word-limit
            />
          </el-form-item>

          <el-form-item label="任务分组">
            <el-select
              v-model="formData.category"
              filterable
              allow-create
              default-first-option
              placeholder="选择或输入分组名称"
              style="width: 100%"
            >
              <el-option
                v-for="group in availableGroups"
                :key="group"
                :label="group"
                :value="group"
              />
            </el-select>
            <div class="field-hint" style="margin-top: 4px;">用于任务列表中的分组筛选和展示</div>
          </el-form-item>

          <el-form-item label="标签">
            <el-select
              v-model="formData.tags"
              multiple
              filterable
              allow-create
              placeholder="添加标签（按回车确认）"
              style="width: 100%"
            >
            </el-select>
          </el-form-item>
        </div>

        <!-- Cron表达式 -->
        <div class="form-section">
          <h3 class="section-title">调度规则</h3>

          <el-form-item label="预设" label-width="80px">
            <el-select
              placeholder="选择预设Cron表达式"
              @change="selectPreset"
              style="width: 300px"
            >
              <el-option
                v-for="preset in cronPresets"
                :key="preset.value"
                :label="preset.label"
                :value="preset.value"
              >
                <span style="float: left">{{ preset.label }}</span>
                <span style="float: right; color: #8492a6; font-size: 12px">
                  {{ preset.value }}
                </span>
              </el-option>
            </el-select>
          </el-form-item>

          <el-form-item label="Cron表达式" required label-width="120px">
            <el-input
              v-model="formData.cron_expr"
              placeholder="* * * * *"
              style="width: 300px"
              @input="buildCronExpr"
            />
            <span class="cron-hint">格式：分 时 日 月 周</span>
          </el-form-item>

          <!-- Cron表达式构建器 -->
          <el-form-item label="分钟">
            <el-input v-model="cron.minute" @input="buildCronExpr" placeholder="* 或 0-59" />
            <span class="field-hint">0-59</span>
          </el-form-item>

          <el-form-item label="小时">
            <el-input v-model="cron.hour" @input="buildCronExpr" placeholder="* 或 0-23" />
            <span class="field-hint">0-23</span>
          </el-form-item>

          <el-form-item label="日期">
            <el-input v-model="cron.dayOfMonth" @input="buildCronExpr" placeholder="* 或 1-31" />
            <span class="field-hint">1-31</span>
          </el-form-item>

          <el-form-item label="月份">
            <el-input v-model="cron.month" @input="buildCronExpr" placeholder="* 或 1-12" />
            <span class="field-hint">1-12</span>
          </el-form-item>

          <el-form-item label="星期">
            <el-input v-model="cron.dayOfWeek" @input="buildCronExpr" placeholder="* 或 0-6" />
            <span class="field-hint">0-6 (0=周日)</span>
          </el-form-item>

          <el-form-item label="启用任务">
            <el-switch v-model="formData.enabled" />
          </el-form-item>
        </div>

        <!-- 执行配置 -->
        <div class="form-section">
          <h3 class="section-title">执行配置</h3>

          <el-form-item label="目标服务器" required>
            <el-radio-group v-model="formData.target_type">
              <el-radio value="node_id">指定服务器</el-radio>
              <el-radio value="tags">按标签选择</el-radio>
            </el-radio-group>
          </el-form-item>

          <el-form-item v-if="formData.target_type === 'node_id'" label="选择服务器" required>
            <el-select
              v-model="formData.target_value"
              placeholder="选择执行任务的服务器"
              :loading="loadingNodes"
              style="width: 100%"
            >
              <el-option
                v-for="node in nodes"
                :key="node.id"
                :label="`${node.hostname} (${node.ip})`"
                :value="node.id"
              >
                <div style="display: flex; justify-content: space-between; align-items: center">
                  <span>{{ node.hostname }}</span>
                  <span style="color: #8492a6; font-size: 12px">{{ node.ip }}</span>
                </div>
              </el-option>
            </el-select>
            <span class="field-hint">任务将在选定的服务器上执行</span>
          </el-form-item>

          <el-form-item v-if="formData.target_type === 'tags'" label="服务器标签" required>
            <el-select
              v-model="formData.tags"
              multiple
              filterable
              placeholder="选择服务器标签"
              style="width: 100%"
              :loading="loadingTags"
            >
              <el-option
                v-for="tag in availableTags"
                :key="tag"
                :label="tag"
                :value="tag"
              />
            </el-select>
            <span class="field-hint">任务将在包含任一标签的服务器上执行</span>
          </el-form-item>

          <el-form-item label="执行命令" required>
            <el-input
              v-model="formData.command"
              type="textarea"
              placeholder="输入要执行的Shell命令"
              :rows="4"
              spellcheck="false"
            />
          </el-form-item>

          <el-form-item label="超时时间（秒）">
            <el-input-number
              v-model="formData.timeout"
              :min="1"
              :max="86400"
              :step="60"
            />
            <span class="field-hint">默认3600秒（1小时）</span>
          </el-form-item>

          <el-form-item label="严格模式">
            <el-switch v-model="formData.strict_mode" />
            <div class="field-hint" style="margin-top: 8px; line-height: 1.6;">
              <div style="margin-bottom: 4px;">✓ 启用：任何命令失败立即退出脚本（推荐多行脚本）</div>
              <div>✗ 禁用：使用 bash 默认行为（单个命令失败不会终止脚本）</div>
            </div>
          </el-form-item>

          <el-form-item label="环境变量">
            <div v-for="(envVar, index) in formData.env" :key="index" class="env-item">
              <el-input
                v-model="envVar.key"
                placeholder="变量名"
                style="width: 200px"
              />
              <span class="env-separator">=</span>
              <el-input
                v-model="envVar.value"
                placeholder="变量值"
                style="flex: 1"
              />
              <el-button
                :icon="Delete"
                type="danger"
                size="small"
                @click="removeEnv(index)"
              />
            </div>
            <el-button :icon="Plus" size="small" @click="addEnv">添加环境变量</el-button>
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
  padding: 24px 0;
  border-bottom: 1px solid #e2e8f0;
}

.form-section:last-of-type {
  border-bottom: none;
}

.section-title {
  font-size: 16px;
  font-weight: 600;
  color: #1e293b;
  margin: 0 0 20px 0;
}

.cron-hint {
  margin-left: 12px;
  font-size: 13px;
  color: #64748b;
}

.field-hint {
  margin-left: 12px;
  font-size: 12px;
  color: #94a3b8;
}

.env-item {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 8px;
}

.env-separator {
  color: #64748b;
}

.form-actions {
  display: flex;
  justify-content: flex-end;
  gap: 12px;
  margin-top: 24px;
}

/* 响应式设计 */
@media (max-width: 768px) {
  .job-edit {
    padding: 16px;
  }

  .page-title {
    font-size: 20px;
  }

  .form-section {
    padding: 16px 0;
  }

  .env-item {
    flex-wrap: wrap;
  }

  .env-item .el-input {
    width: 100% !important;
  }
}
</style>
