<template>
  <div class="log-admin">
    <!-- 概览 + 筛选 -->
    <section class="card card-pad">
      <header class="panel-head">
        <div>
          <h2 class="block-title">日志解析</h2>
          <p class="block-desc">
            解析 <code>runtime/logs/&lt;日期&gt;/{info,warn,error,debug}.log</code> 与
            <code>runtime/sms/{sms,email}.log</code>，只读查看；
            JSON 行按字段展开，解析不出来的行也会原样展示
          </p>
        </div>
        <div class="head-actions">
          <button class="btn btn-sm" :disabled="loading || loadingMeta" @click="refreshAll()">
            <i class="bi bi-arrow-clockwise" /> 刷新
          </button>
        </div>
      </header>

      <!-- 工具栏 -->
      <div class="list-filter">
        <div class="status-tabs">
          <button
            v-for="tab in sourceTabs"
            :key="tab.key"
            type="button"
            class="status-tab"
            :class="{ 'is-active': source === tab.key }"
            @click="switchSource(tab.key)"
          >
            <i :class="tab.icon" /> {{ tab.label }}
          </button>
        </div>

        <div class="filter-right">
          <FormSelect variant="pill"
            v-if="source === 'system'"
            v-model="date"
            :options="dateOptions"
            icon="bi bi-calendar3"
            placeholder="选择日期"
            @change="onDateChange" />
          <FormSelect variant="pill"
            v-model="file"
            :options="fileOptions"
            icon="bi bi-file-earmark-text"
            placeholder="选择文件"
            @change="reloadFromFirstPage" />
          <FormSelect variant="pill"
            v-model="level"
            :options="levelOptions"
            icon="bi bi-funnel"
            placeholder="全部级别"
            @change="reloadFromFirstPage" />
          <div class="search-box">
            <i class="bi bi-search" aria-hidden="true" />
            <FormInput
              v-model="keyword"
              class="search-input"
              type="search"
              placeholder="搜索关键字…"
              aria-label="搜索日志关键字"
              @input="doSearch" />
          </div>
        </div>
      </div>

      <!-- 统计：级别可点击筛选 -->
      <div class="stat-grid" :aria-busy="loading">
        <button
          v-for="item in levelStats"
          :key="item.key"
          type="button"
          class="stat-card"
          :class="{ 'is-link': item.key !== 'other', 'is-active': level === item.key }"
          @click="pickLevel(item.key)"
        >
          <span class="stat-icon" :style="{ color: item.color }"><i :class="item.icon" /></span>
          <span class="stat-body">
            <span class="stat-value">{{ loading ? '···' : item.value }}</span>
            <span class="stat-label">{{ item.label }}</span>
          </span>
        </button>

        <div class="stat-card is-plain">
          <span class="stat-icon"><i class="bi bi-list-ol" /></span>
          <span class="stat-body">
            <span class="stat-value">{{ data.lines || 0 }}</span>
            <span class="stat-label">文件总行数</span>
          </span>
        </div>
      </div>

      <p v-if="data.truncated" class="block-desc warn-text">
        <i class="bi bi-exclamation-triangle" />
        文件过大，仅解析了前 {{ maxLines }} 行（可先按级别或关键字筛选）
      </p>
    </section>

    <!-- 条目列表 -->
    <div class="card card-pad">
      <header class="list-head">
        <div>
          <h2 class="block-title">
            {{ currentFileName || '日志条目' }}
          </h2>
          <p class="block-desc">
            <template v-if="loading">加载中…</template>
            <template v-else-if="!total">暂无匹配的日志</template>
            <template v-else>
              匹配 <strong>{{ total }}</strong> 条 · 第 {{ page }} / {{ data.pages || 1 }} 页 · 默认最新在前
            </template>
          </p>
        </div>
        <!-- 文件基本信息：体积 / 总行数 / 覆盖的时间范围 -->
        <div v-if="currentFileName" class="file-meta">
          <span class="meta-chip" title="文件大小">
            <i class="bi bi-hdd" /> {{ formatSize(data.fileSize) }}
          </span>
          <span class="meta-chip" title="文件总行数">
            <i class="bi bi-list-ol" /> {{ data.lines || 0 }} 行
          </span>
          <span v-if="data.firstTime || data.lastTime" class="meta-chip" title="日志时间范围">
            <i class="bi bi-clock-history" /> {{ data.firstTime || '?' }} ~ {{ data.lastTime || '?' }}
          </span>
          <!-- 字段列折叠：默认只铺前 6 个字段，其余按需展开 -->
          <button
            v-if="hiddenFieldCount || showAllFields"
            type="button"
            class="meta-chip is-btn"
            @click="showAllFields = !showAllFields"
          >
            <i class="bi" :class="showAllFields ? 'bi-arrows-collapse' : 'bi-arrows-expand'" />
            {{ showAllFields ? '收起字段列' : `显示全部字段 +${hiddenFieldCount}` }}
          </button>
        </div>
      </header>

      <div v-if="loading" class="is-loading"><span class="spinner" /> 解析中...</div>

      <EmptyState
        v-else-if="!list.length"
        icon="bi bi-journal-x"
        :text="emptyHint"
      />

      <template v-else>
        <!-- 表格：一行一条日志，基本信息全部铺平；点击行展开完整字段与原始行 -->
        <div class="log-table-wrap">
          <table class="log-table">
            <thead>
              <tr>
                <th class="col-line">行</th>
                <th class="col-time">时间</th>
                <th class="col-level">级别</th>
                <th class="col-msg">消息</th>
                <th class="col-caller">调用方</th>
                <!-- 动态列：日志里的业务字段（cost / ip / method / path / params …） -->
                <th
                  v-for="field in fieldColumns"
                  :key="field.key"
                  class="col-dyn"
                  :style="{ width: fieldWidth(field.key) + 'px' }"
                  :title="`${field.key}（${fieldLabel(field.key)}）· 出现 ${field.count} 次`"
                >{{ fieldLabel(field.key) }}</th>
                <th class="col-act" aria-label="展开详情"></th>
              </tr>
            </thead>
            <tbody>
              <template v-for="(item, index) in list" :key="`${page}-${index}`">
                <tr
                  class="log-row"
                  :class="[`is-${item.level || 'raw'}`, { 'is-expanded': expanded === index }]"
                  @click="toggle(index)"
                >
                  <td class="col-line">{{ item.line || '—' }}</td>
                  <td class="col-time">{{ item.time || '—' }}</td>
                  <td class="col-level">
                    <span class="log-level" :class="`is-${item.level || 'raw'}`">{{ levelLabel(item) }}</span>
                  </td>
                  <td class="col-msg" :title="item.msg || item.raw">{{ item.msg || item.raw }}</td>
                  <td class="col-caller" :title="item.caller || ''">{{ item.caller || '—' }}</td>
                  <td
                    v-for="field in fieldColumns"
                    :key="field.key"
                    class="col-dyn"
                    :title="fieldTitle(item, field.key)"
                  >{{ fieldValue(item, field.key) }}</td>
                  <td class="col-act">
                    <i class="bi log-toggle" :class="expanded === index ? 'bi-chevron-up' : 'bi-chevron-down'" />
                  </td>
                </tr>

                <tr v-if="expanded === index" class="log-detail-row">
                  <td :colspan="6 + fieldColumns.length">
                    <div class="log-detail">
                      <div v-if="item.status" class="detail-row">
                        <span class="detail-label">状态</span>
                        <span class="detail-value">{{ item.status }}</span>
                      </div>
                      <div class="detail-row">
                        <span class="detail-label">消息</span>
                        <span class="detail-value detail-msg">{{ item.msg || item.raw }}</span>
                      </div>
                      <div v-if="hasFields(item)" class="detail-row">
                        <span class="detail-label">字段</span>
                        <pre class="detail-pre">{{ prettyFields(item) }}</pre>
                      </div>
                      <div class="detail-row">
                        <span class="detail-label">原始行</span>
                        <pre class="detail-pre">{{ item.raw }}</pre>
                      </div>
                      <div class="detail-actions">
                        <span class="detail-line">第 {{ item.line || '?' }} 行</span>
                        <button class="btn btn-sm btn-ghost" @click.stop="copy(item.raw)">
                          <i class="bi bi-clipboard" /> 复制原始行
                        </button>
                      </div>
                    </div>
                  </td>
                </tr>
              </template>
            </tbody>
          </table>
        </div>

        <Pagination
          v-if="!loading && total > 0"
          :current="page"
          :total="total"
          :page-size="limit"
          :sizes="PAGE_SIZE_OPTIONS"
          @update:page-size="(size) => { limit = size; page = 1; load() }"
          @update:current="(p) => { page = p; load() }"
        />
      </template>
    </div>
  </div>
</template>

<script setup>
import FormInput from '@/components/form/FormInput.vue'
import FormSelect from '@/components/form/FormSelect.vue'
import { computed, onMounted, onUnmounted, ref } from 'vue'
import EmptyState from '@/components/EmptyState.vue'
import Pagination from '@/components/Pagination.vue'
import { listLogDates, listLogFiles, readLog } from '@/api/log'
import { PAGE_SIZE_DEFAULT, PAGE_SIZE_OPTIONS } from '@/utils/page-size'
import { toast } from '@/utils/toast'

// 与后端 logMaxLines 保持一致（仅用于提示文案）
const maxLines = 200000

const sourceTabs = [
  { key: 'system', label: '系统日志', icon: 'bi bi-file-earmark-code' },
  { key: 'notify', label: '通知日志', icon: 'bi bi-envelope-paper' }
]

const levelMeta = {
  error: { label: '错误', color: 'var(--danger)', icon: 'bi bi-x-octagon' },
  warn: { label: '警告', color: 'var(--warning, #b8860b)', icon: 'bi bi-exclamation-triangle' },
  info: { label: '信息', color: 'var(--primary, #2f7d6b)', icon: 'bi bi-info-circle' },
  debug: { label: '调试', color: 'var(--text-muted)', icon: 'bi bi-bug' },
  other: { label: '未解析', color: 'var(--text-light, #999)', icon: 'bi bi-question-circle' }
}

const source = ref('system')
const dates = ref([])
const notifyFiles = ref([])
const files = ref([])

const date = ref('')
const file = ref('')
const level = ref('')
const keyword = ref('')
const page = ref(1)
const limit = ref(PAGE_SIZE_DEFAULT)

const loading = ref(false)
const loadingMeta = ref(false)
const expanded = ref(-1)

// 空态 / 重置时的统一结构（文件级元信息一起清掉，避免切文件后还显示上一个文件的体积）
function emptyData() {
  return {
    list: [],
    total: 0,
    lines: 0,
    pages: 0,
    levels: {},
    truncated: false,
    empty: false,
    fileSize: 0,
    firstTime: '',
    lastTime: '',
    fieldKeys: []
  }
}

const data = ref(emptyData())

const list = computed(() => data.value.list || [])
const total = computed(() => Number(data.value.total) || 0)

const dateOptions = computed(() =>
  dates.value.map((item) => ({
    value: item.date,
    label: `${item.date}（${item.files} 个文件 · ${formatSize(item.size)}）`
  }))
)

const fileOptions = computed(() =>
  (source.value === 'notify' ? notifyFiles.value : files.value).map((item) => ({
    value: item.name,
    label: `${item.name}（${formatSize(item.size)}）`
  }))
)

const levelOptions = [
  { value: '', label: '全部级别' },
  { value: 'error', label: '错误 error' },
  { value: 'warn', label: '警告 warn' },
  { value: 'info', label: '信息 info' },
  { value: 'debug', label: '调试 debug' }
]

const levelStats = computed(() => {
  const levels = data.value.levels || {}
  return ['error', 'warn', 'info', 'debug', 'other'].map((key) => ({
    key,
    label: levelMeta[key].label,
    color: levelMeta[key].color,
    icon: levelMeta[key].icon,
    value: Number(levels[key]) || 0
  }))
})

const currentFileName = computed(() => file.value || '')

const emptyHint = computed(() => {
  if (data.value.empty) return '该日志文件不存在或已被清理，换个日期或文件看看'
  if (keyword.value || level.value) return '当前筛选条件下没有日志，试试清空关键字或级别'
  if (source.value === 'notify') return '还没有短信 / 邮件发送记录'
  return '这一天还没有日志'
})

function formatSize(size) {
  const value = Number(size) || 0
  if (value < 1024) return `${value} B`
  if (value < 1024 * 1024) return `${(value / 1024).toFixed(1)} KB`
  return `${(value / 1024 / 1024).toFixed(2)} MB`
}

function levelLabel(item) {
  const key = item.level || 'raw'
  if (key === 'raw') return '未解析'
  return levelMeta[key]?.label || key
}

function hasFields(item) {
  const fields = item?.fields
  return !!fields && typeof fields === 'object' && Object.keys(fields).length > 0
}

function prettyFields(item) {
  try {
    return JSON.stringify(item.fields || {}, null, 2)
  } catch {
    return String(item.fields)
  }
}

/* ---------- 动态字段列 ----------
 * 后端 read 会统计本份日志里出现过的业务字段（cost / ip / method / path / params …），
 * 按出现次数降序返回 field_keys，这里据此生成列：默认只铺 6 列，其余收进「显示全部字段」。
 */
const DEFAULT_FIELD_COLUMNS = 6
const showAllFields = ref(false)

const fieldColumns = computed(() => {
  const keys = data.value.fieldKeys || []
  return showAllFields.value ? keys : keys.slice(0, DEFAULT_FIELD_COLUMNS)
})

// 还有多少列「没铺」：固定按总字段数算，不随展开状态变化
// （之前用「总数 - 当前显示数」，展开后变成 0，翻页时会被下面的复位逻辑强制收起）
const hiddenFieldCount = computed(() =>
  Math.max(0, (data.value.fieldKeys || []).length - DEFAULT_FIELD_COLUMNS)
)

// 表头中文名：常见字段给中文，没登记的字段原样显示（避免瞎翻译业务字段）
const FIELD_LABELS = {
  path: '路径',
  method: '方法',
  params: '参数',
  ip: 'IP',
  'user-agent': 'UA',
  errors: '错误',
  cost: '耗时',
  error: '错误信息',
  stack: '堆栈',
  request: '请求',
  uid: '用户ID',
  id: 'ID',
  url: '链接',
  code: '状态码',
  status: '状态',
  scope: '范围',
  table: '数据表',
  action: '动作',
  count: '数量',
  size: '大小',
  file: '文件',
  body: '请求体',
  query: '查询参数'
}

function fieldLabel(key) {
  return FIELD_LABELS[key] || key
}

// 常用字段给固定列宽：长文本（user-agent / path / url）宽一点，短字段窄一点
const FIELD_WIDTH = {
  ip: 170,
  path: 220,
  url: 220,
  'user-agent': 200,
  cost: 110,
  method: 80,
  errors: 150
}

function fieldWidth(key) {
  return FIELD_WIDTH[key] || 140
}

// 单元格取值：对象压成 k=v（如 params → date=2026-11-21, scope=system），数组显示项数
function fieldValue(item, key) {
  const fields = item?.fields
  if (!fields || typeof fields !== 'object') return '—'
  const value = fields[key]
  if (value === undefined || value === null) return '—'

  if (Array.isArray(value)) {
    return value.length ? `[${value.length} 项]` : '[]'
  }

  if (typeof value === 'object') {
    const entries = Object.entries(value)
    if (!entries.length) return '{}'
    const text = entries
      .map(([subKey, subValue]) => {
        const text_ = subValue === null ? '' : typeof subValue === 'object' ? '[…]' : String(subValue)
        return `${subKey}=${text_}`
      })
      .join(', ')
    return text.length > 60 ? `${text.slice(0, 60)}…` : text
  }

  const text = String(value)
  return text === '' ? '—' : text
}

// 悬浮提示用完整内容（对象给格式化 JSON），单元格里才截断
function fieldTitle(item, key) {
  const value = item?.fields?.[key]
  if (value === undefined || value === null) return ''
  if (typeof value === 'object') {
    try {
      return JSON.stringify(value, null, 2)
    } catch {
      return String(value)
    }
  }
  return String(value)
}

function toggle(index) {
  expanded.value = expanded.value === index ? -1 : index
}

async function copy(text) {
  const value = String(text || '')
  if (!value) return
  try {
    await navigator.clipboard.writeText(value)
    toast.success('已复制')
  } catch {
    toast.warning('当前环境不支持自动复制，请手动选择文本')
  }
}

// ---------- 数据加载 ----------

async function loadMeta() {
  loadingMeta.value = true
  try {
    const res = await listLogDates()
    const payload = res?.data || {}
    dates.value = Array.isArray(payload.dates) ? payload.dates : []
    notifyFiles.value = Array.isArray(payload.notify) ? payload.notify : []

    if (!date.value) {
      // 默认看最近一天（没有历史日期时用今天，交给后端回空态）
      date.value = dates.value[0]?.date || payload.today || ''
    }
  } catch {
    dates.value = []
    notifyFiles.value = []
  } finally {
    loadingMeta.value = false
  }
}

async function loadFiles() {
  try {
    const res = await listLogFiles(source.value === 'notify' ? { scope: 'notify' } : { date: date.value })
    const list_ = res?.data?.list || []
    files.value = source.value === 'notify' ? [] : list_
    if (source.value === 'notify') notifyFiles.value = list_

    const options = source.value === 'notify' ? notifyFiles.value : files.value
    // 当前文件已不在列表里（换日期/换来源）→ 自动选中第一个
    if (!options.some((item) => item.name === file.value)) {
      file.value = options[0]?.name || ''
    }
  } catch {
    files.value = []
    file.value = ''
  }
}

// 请求序号：日志文件可能很大（解析要几秒），快速切换日期 / 文件 / 来源时
// 旧请求可能后回来把新结果覆盖掉，用序号丢弃过期响应
let loadSeq = 0

async function load() {
  if (!file.value) {
    data.value = emptyData()
    return
  }

  const seq = ++loadSeq
  loading.value = true
  expanded.value = -1
  try {
    const res = await readLog({
      scope: source.value,
      date: date.value,
      file: file.value,
      level: level.value,
      keyword: keyword.value.trim(),
      page: page.value,
      limit: limit.value,
      order: 'desc'
    })
    if (seq !== loadSeq) return
    const payload = res?.data || {}
    data.value = {
      list: payload.list || [],
      total: payload.total || 0,
      lines: payload.lines || 0,
      pages: payload.pages || 0,
      levels: payload.levels || {},
      truncated: !!payload.truncated,
      empty: !!payload.empty,
      fileSize: Number(payload.file_size) || 0,
      firstTime: payload.first_time || '',
      lastTime: payload.last_time || '',
      fieldKeys: Array.isArray(payload.field_keys) ? payload.field_keys : []
    }
    // 字段本来就不到 6 个时，「展开/收起」没有意义，切文件后复位
    if (showAllFields.value && !hiddenFieldCount.value) showAllFields.value = false
    if (payload.empty && page.value === 1) toast.info('该日志文件不存在或已被清理')
  } catch (e) {
    if (seq !== loadSeq) return
    // silent 请求：这里自己提示，避免与拦截器重复
    toast.error(e?.msg || '读取日志失败')
    data.value = emptyData()
  } finally {
    // 只有最后一次请求能收起 loading，否则旧请求先返回会把转圈提前关掉
    if (seq === loadSeq) loading.value = false
  }
}

function reloadFromFirstPage() {
  // 切换筛选时如果防抖里还压着一次搜索，先取消，避免多打一次请求
  if (searchTimer) clearTimeout(searchTimer)
  page.value = 1
  load()
}

async function onDateChange() {
  await loadFiles()
  reloadFromFirstPage()
}

async function switchSource(key) {
  if (source.value === key) return
  source.value = key
  level.value = ''
  keyword.value = ''
  await loadFiles()
  reloadFromFirstPage()
}

function pickLevel(key) {
  // 「未解析」不是可筛选的级别（解析失败的行没有 level 字段）
  if (key === 'other') return
  level.value = level.value === key ? '' : key
  reloadFromFirstPage()
}

// 关键字输入防抖，避免每敲一个字都扫一遍文件
let searchTimer = null
function doSearch() {
  if (searchTimer) clearTimeout(searchTimer)
  searchTimer = setTimeout(() => {
    page.value = 1
    load()
  }, 350)
}

async function refreshAll() {
  await loadMeta()
  await loadFiles()
  await load()
}

onMounted(async () => {
  await loadMeta()
  await loadFiles()
  await load()
})

// 离开页面时清掉待触发的搜索防抖，避免卸载后还发一次请求
onUnmounted(() => {
  if (searchTimer) clearTimeout(searchTimer)
})
</script>

<style scoped>
/* 概览卡与条目列表卡是上下两块，需要竖向间距（与其它后台页统一 16px） */
.log-admin {
  display: flex;
  flex-direction: column;
  gap: 16px;
}
.panel-head,
.list-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 14px;
}
.block-title {
  margin: 0;
  font-size: 14px;
  font-weight: 600;
}
.block-desc {
  margin: 4px 0 0;
  font-size: 12px;
  line-height: 1.7;
  color: var(--text-muted);
}
.block-desc code {
  padding: 1px 5px;
  font-size: 11px;
  border-radius: 3px;
  background: var(--bg-muted);
}
/* 截断提示紧跟在统计卡下面，单独给一点上间距，否则会贴着卡片 */
.warn-text {
  margin-top: 12px;
  color: var(--warning, #b8860b);
}
.head-actions {
  display: flex;
  gap: 8px;
}

/* 筛选栏 */
.list-filter {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  margin-bottom: 14px;
}
.status-tabs {
  display: flex;
  gap: 6px;
}
.status-tab {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  padding: 5px 12px;
  font-size: 13px;
  border: 1px solid var(--border);
  border-radius: 999px;
  background: transparent;
  color: var(--text-soft);
  cursor: pointer;
  transition: all 0.15s;
}
.status-tab:hover {
  border-color: var(--primary);
  color: var(--text);
}
.status-tab.is-active {
  border-color: var(--primary);
  color: var(--primary-deep);
  background: var(--accent-soft);
}
.filter-right {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px;
}
.search-box {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 0 10px;
  height: 32px;
  border: 1px solid var(--border);
  border-radius: var(--radius-sm);
  background: var(--bg-card);
  color: var(--text-muted);
}
.search-input {
  width: 170px;
  border: none;
  outline: none;
  background: transparent;
  font-size: 13px;
  color: var(--text);
}

/* 统计卡 */
.stat-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(120px, 1fr));
  gap: 10px;
}
.stat-card {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 10px 12px;
  border: 1px solid var(--border);
  border-radius: var(--radius-sm);
  background: var(--bg-card);
  text-align: left;
  cursor: default;
}
.stat-card.is-link {
  cursor: pointer;
}
.stat-card.is-link:hover {
  border-color: var(--primary);
}
.stat-card.is-active {
  border-color: var(--primary);
  background: var(--accent-soft);
}
.stat-icon {
  font-size: 16px;
  color: var(--text-light);
}
.stat-body {
  display: flex;
  flex-direction: column;
  min-width: 0;
}
.stat-value {
  font-size: 16px;
  font-weight: 600;
  line-height: 1.2;
}
.stat-label {
  font-size: 11px;
  color: var(--text-muted);
}

/* 文件元信息（列表卡右上角） */
.file-meta {
  display: flex;
  flex-wrap: wrap;
  justify-content: flex-end;
  gap: 6px;
}
.meta-chip {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  padding: 3px 9px;
  font-size: 11px;
  line-height: 1.6;
  color: var(--text-muted);
  background: var(--bg-muted);
  border-radius: 999px;
  white-space: nowrap;
  font-variant-numeric: tabular-nums;
}
.meta-chip i {
  font-size: 11px;
}
/* 字段列开关：胶囊形状与信息胶囊一致，但可点 */
.meta-chip.is-btn {
  border: 1px solid var(--border);
  background: var(--bg-card);
  font-family: inherit;
  cursor: pointer;
  transition: color var(--dur) var(--ease), border-color var(--dur) var(--ease);
}
.meta-chip.is-btn:hover {
  color: var(--primary);
  border-color: var(--primary-soft);
}

/* 日志表格 */
.log-table-wrap {
  overflow-x: auto;
  border: 1px solid var(--border);
  border-radius: var(--radius-sm);
}
.log-table {
  /* 列宽固定 + 表格按内容撑宽、容器横向滚动：字段列再多也不会把消息列挤没 */
  width: max-content;
  min-width: 100%;
  border-collapse: collapse;
  table-layout: fixed;
  font-size: 12px;
}
.log-table thead th {
  padding: 8px 10px;
  font-size: 11px;
  font-weight: 600;
  text-align: left;
  white-space: nowrap;
  /* 未登记的字段名可能很长（如 x-request-id），固定列宽下要截断而不是撑破表格 */
  overflow: hidden;
  text-overflow: ellipsis;
  color: var(--text-muted);
  background: var(--bg-muted);
  border-bottom: 1px solid var(--border);
}
.log-table td {
  padding: 7px 10px;
  vertical-align: middle;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  border-bottom: 1px solid var(--border-soft);
}
.log-table tbody tr:last-child td {
  border-bottom: none;
}
/* 列宽：消息列吃掉剩余宽度，其余固定，便于快速扫读 */
.col-line {
  width: 64px;
  color: var(--text-light);
  font-variant-numeric: tabular-nums;
}
.col-time {
  width: 148px;
  color: var(--text-muted);
  font-variant-numeric: tabular-nums;
}
.col-level {
  width: 76px;
}
.col-msg {
  width: 380px;
  color: var(--text);
}
.col-caller {
  width: 180px;
  color: var(--text-light);
}
/* 动态业务字段列：宽度由行内 style 给出（见 fieldWidth），这里只管文本表现 */
.col-dyn {
  color: var(--text-soft);
  font-variant-numeric: tabular-nums;
}
.col-act {
  width: 36px;
  text-align: center;
  color: var(--text-light);
}
.log-row {
  cursor: pointer;
}
.log-row:hover td,
.log-row.is-expanded td {
  background: var(--bg-muted);
}
.log-row.is-error .col-msg {
  color: var(--danger);
}
.log-level {
  display: inline-block;
  min-width: 44px;
  padding: 1px 6px;
  font-size: 11px;
  text-align: center;
  border-radius: 3px;
  color: #fff;
  background: var(--text-light);
}
.log-level.is-error {
  background: var(--danger);
}
.log-level.is-warn {
  background: var(--warning, #b8860b);
}
.log-level.is-info {
  background: var(--primary, #2f7d6b);
}
.log-level.is-debug {
  background: var(--text-muted);
}
.log-level.is-raw {
  background: var(--text-light);
}
.log-toggle {
  color: var(--text-light);
}

/* 展开行：表格里用一整行承载详情 */
.log-detail-row > td {
  padding: 0;
  white-space: normal;
  background: var(--bg-muted);
}

/* 展开明细 */
.log-detail {
  padding: 4px 12px 12px;
}
.detail-row {
  display: flex;
  gap: 10px;
  margin-bottom: 8px;
}
.detail-label {
  flex-shrink: 0;
  width: 56px;
  padding-top: 6px;
  font-size: 11px;
  color: var(--text-muted);
}
.detail-value {
  font-size: 12px;
}
.detail-pre {
  flex: 1;
  min-width: 0;
  margin: 0;
  padding: 8px 10px;
  font-size: 12px;
  line-height: 1.6;
  white-space: pre-wrap;
  word-break: break-all;
  border: 1px solid var(--border);
  border-radius: var(--radius-sm);
  background: var(--bg-card);
  color: var(--text-soft);
}
.detail-actions {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
}
/* 展开行左侧的行号提示，方便回原文件对照 */
.detail-line {
  font-size: 11px;
  color: var(--text-muted);
  font-variant-numeric: tabular-nums;
}
/* 表格里消息被截断，展开后完整显示（保留换行） */
.detail-msg {
  word-break: break-all;
  white-space: pre-wrap;
}

/* 窄屏：动态字段列 / 调用方列依次让位，消息列始终保留 */
@media (max-width: 900px) {
  .col-dyn {
    display: none;
  }
}
@media (max-width: 640px) {
  .col-caller {
    display: none;
  }
  .col-time {
    width: 132px;
  }
  .col-msg {
    width: 260px;
  }
  .file-meta {
    justify-content: flex-start;
  }
}
</style>
