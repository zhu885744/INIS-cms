<template>
  <div class="card card-pad">
    <header class="overview-head">
      <div>
        <h2 class="block-title">创作中心</h2>
        <p class="block-desc">管理你的文章与动态，随时继续未完成的创作</p>
      </div>
    </header>

    <!-- 影响力：粉丝 / 获赞 / 关注（user-follows/stats，点数字去个人主页） -->
    <div v-if="uid" class="influence-row">
      <RouterLink :to="`/author/${uid}`" class="influence-item" title="查看个人主页">
        <i class="bi bi-people" aria-hidden="true" />
        <strong>{{ loaded ? fmt(stats.fans) : '—' }}</strong>
        <span>粉丝</span>
      </RouterLink>
      <RouterLink :to="`/author/${uid}`" class="influence-item" title="查看个人主页">
        <i class="bi bi-hand-thumbs-up" aria-hidden="true" />
        <strong>{{ loaded ? fmt(stats.likes) : '—' }}</strong>
        <span>获赞</span>
      </RouterLink>
      <RouterLink :to="`/author/${uid}`" class="influence-item" title="查看个人主页">
        <i class="bi bi-person-plus" aria-hidden="true" />
        <strong>{{ loaded ? fmt(stats.follow) : '—' }}</strong>
        <span>关注</span>
      </RouterLink>
      <p class="influence-note">
        <i class="bi bi-info-circle" aria-hidden="true" />
        只统计已发布且审核通过的内容
      </p>
    </div>

    <!-- 内容统计：整卡可点，直达对应列表 -->
    <div class="stat-grid">
      <RouterLink to="/manage/posts" class="stat-card">
        <span class="stat-ico is-primary"><i class="bi bi-file-earmark-text" aria-hidden="true" /></span>
        <span class="stat-num">{{ loaded ? stats.articleTotal : '—' }}</span>
        <span class="stat-label">文章总数</span>
      </RouterLink>
      <RouterLink to="/manage/posts" class="stat-card">
        <span class="stat-ico"><i class="bi bi-file-earmark" aria-hidden="true" /></span>
        <span class="stat-num">{{ loaded ? stats.articleDraft : '—' }}</span>
        <span class="stat-label">文章草稿</span>
      </RouterLink>
      <RouterLink to="/manage/posts" class="stat-card" :class="{ 'is-warn': stats.articlePending > 0 }">
        <span class="stat-ico"><i class="bi bi-hourglass-split" aria-hidden="true" /></span>
        <span class="stat-num">{{ loaded ? stats.articlePending : '—' }}</span>
        <span class="stat-label">待审核文章</span>
      </RouterLink>
      <RouterLink to="/manage/moments" class="stat-card">
        <span class="stat-ico"><i class="bi bi-chat-square-dots" aria-hidden="true" /></span>
        <span class="stat-num">{{ loaded ? stats.momentTotal : '—' }}</span>
        <span class="stat-label">动态总数</span>
      </RouterLink>
      <RouterLink to="/manage/moments" class="stat-card" :class="{ 'is-warn': stats.momentPending > 0 }">
        <span class="stat-ico"><i class="bi bi-hourglass-split" aria-hidden="true" /></span>
        <span class="stat-num">{{ loaded ? stats.momentPending : '—' }}</span>
        <span class="stat-label">待审核动态</span>
      </RouterLink>
    </div>

    <p v-if="pendingTotal > 0" class="audit-tip">
      <i class="bi bi-info-circle" aria-hidden="true" />
      有 {{ pendingTotal }} 条内容正在等待审核，审核通过后才会公开显示。
    </p>

    <!-- 功能卡片 -->
    <div class="entry-grid">
      <router-link to="/manage/posts/write" class="entry-card entry-primary">
        <span class="entry-ico"><i class="bi bi-pencil-square" /></span>
        <span class="entry-body">
          <span class="entry-title">写文章</span>
          <span class="entry-desc">撰写新文章，支持 Markdown 与富文本</span>
        </span>
        <i class="bi bi-chevron-right entry-arrow" />
      </router-link>

      <router-link to="/manage/posts" class="entry-card">
        <span class="entry-ico"><i class="bi bi-file-earmark-text" /></span>
        <span class="entry-body">
          <span class="entry-title">我的文章</span>
          <span class="entry-desc">管理已发布文章与草稿，支持搜索与筛选</span>
        </span>
        <i class="bi bi-chevron-right entry-arrow" />
      </router-link>

      <router-link to="/manage/moments" class="entry-card">
        <span class="entry-ico"><i class="bi bi-chat-square-dots" /></span>
        <span class="entry-body">
          <span class="entry-title">我的动态</span>
          <span class="entry-desc">管理发布的动态，可编辑内容与配图</span>
        </span>
        <i class="bi bi-chevron-right entry-arrow" />
      </router-link>

      <router-link to="/moments" class="entry-card">
        <span class="entry-ico"><i class="bi bi-stars" /></span>
        <span class="entry-body">
          <span class="entry-title">发布动态</span>
          <span class="entry-desc">回到首页，弹窗里随手记录此刻的想法</span>
        </span>
        <i class="bi bi-chevron-right entry-arrow" />
      </router-link>
    </div>

    <!-- 近 7 天发布趋势：文章 / 动态并排小柱图 -->
    <section class="trend">
      <h3 class="recent-title">
        近 7 天发布
        <span class="trend-legend">
          <span><i class="trend-dot is-article" />文章</span>
          <span><i class="trend-dot is-moment" />动态</span>
        </span>
      </h3>
      <div v-if="trend.length" class="trend-chart">
        <div
          v-for="day in trend"
          :key="day.date"
          class="trend-col"
          :title="`${day.date}：文章 ${day.article} · 动态 ${day.moment}`"
        >
          <span v-if="day.article + day.moment" class="trend-total">{{ day.article + day.moment }}</span>
          <div class="trend-bars">
            <span class="trend-bar is-article" :style="{ height: barHeight(day.article) }" />
            <span class="trend-bar is-moment" :style="{ height: barHeight(day.moment) }" />
          </div>
          <span class="trend-day" :class="{ 'is-today': day.isToday }">{{ day.label }}</span>
        </div>
      </div>
      <p v-else class="trend-empty">近 7 天还没有发布内容</p>
    </section>

    <!-- 最近创作：文章 + 动态合并，按时间倒序 -->
    <section class="recent">
      <h3 class="recent-title">
        最近创作
        <router-link v-if="recent.length" to="/manage/posts" class="recent-more">
          全部 <i class="bi bi-chevron-right" />
        </router-link>
      </h3>
      <ul v-if="recent.length" class="recent-list">
        <li v-for="item in recent" :key="`${item.kind}-${item.id}`" class="recent-row">
          <span class="recent-kind" :class="`is-${item.kind}`">
            <i
              class="bi"
              :class="item.kind === 'article' ? 'bi-file-earmark-text' : 'bi-chat-square-dots'"
              :title="item.kind === 'article' ? '文章' : '动态'"
              aria-hidden="true" />
          </span>
          <router-link :to="item.to" class="recent-link">{{ item.title }}</router-link>
          <span class="post-status" :class="item.state.cls">{{ item.state.text }}</span>
          <span class="recent-time">{{ fromNow(item.time) }}</span>
        </li>
      </ul>
      <!-- 空态也保留这块区域：否则新用户看到的概览会突然「短一截」 -->
      <p v-else class="recent-empty">
        <i class="bi bi-feather" aria-hidden="true" />
        还没有内容，
        <router-link to="/manage/posts/write">写一篇文章</router-link>
        或去<router-link to="/moments">发条动态</router-link>。
      </p>
    </section>
  </div>
</template>

<script setup>
import { reactive, ref, computed, onMounted } from 'vue'
import { getMyArticles, countMyArticles } from '@/api/article'
import { listMoments } from '@/api/moments'
import { getMyTrend } from '@/api/comm'
import { call } from '@/api/request'
import { useUserStore } from '@/stores/user'
import { storeToRefs } from 'pinia'
import { fromNow } from '@/utils/time'

const userStore = useUserStore()
const { user } = storeToRefs(userStore)
const uid = computed(() => user.value?.id || 0)

const stats = reactive({
  articleTotal: 0,
  articleDraft: 0,
  articlePending: 0,
  momentTotal: 0,
  momentPending: 0,
  // 影响力：来自 user-follows/stats（只统计公开内容）
  fans: 0,
  likes: 0,
  follow: 0
})

// 数据就绪前数字显示「—」，避免从 0 跳到真实值的闪烁
const loaded = ref(false)

const pendingTotal = computed(() => stats.articlePending + stats.momentPending)

const recent = ref([])

// 近 7 天发布趋势（comm/my-trend：每天的文章 / 动态数量）
const trend = ref([])

// 柱高归一：以 7 天内单日最大值为分母；0 也留 2px 基线，柱图不会「断掉」
const trendMax = computed(() =>
  Math.max(1, ...trend.value.map((day) => Math.max(day.article, day.moment)))
)

function barHeight(value) {
  const count = Number(value) || 0
  if (count <= 0) return '2px'
  return `${Math.max(8, Math.round((count / trendMax.value) * 100))}%`
}

async function loadTrend() {
  if (!uid.value) return
  try {
    const res = await getMyTrend(7)
    const list = res?.data?.list || []

    // 用本地日期判断「今天」，避免 toISOString 的 UTC 偏移导致标错
    const now = new Date()
    const todayKey = `${now.getFullYear()}-${String(now.getMonth() + 1).padStart(2, '0')}-${String(
      now.getDate()
    ).padStart(2, '0')}`

    trend.value = list.map((item) => {
      const [, month, day] = String(item.date || '').split('-')
      return {
        ...item,
        article: Number(item.article) || 0,
        moment: Number(item.moment) || 0,
        label: month && day ? `${month}-${day}` : item.date,
        isToday: item.date === todayKey
      }
    })
  } catch {
    trend.value = []
  }
}

function fmt(n) {
  return Number(n || 0).toLocaleString('zh-CN')
}

// 安全取数字：count 接口返回 res.data，也可能是 { data: n }
function toNum(res) {
  const d = res?.data
  if (typeof d === 'number') return d
  return Number(d?.data ?? d?.count ?? 0) || 0
}

/** 文章 / 动态共用的状态标签（0 待审核 / 1 已通过 / 2 未通过 + 草稿） */
function stateOf(item) {
  const status = Number(item.status) || 0
  const audit = Number(item.audit) || 0
  if (status === 1 && audit === 0) return { text: '待审核', cls: 'is-audit' }
  if (status === 1 && audit === 2) return { text: '未通过', cls: 'is-reject' }
  if (status === 1) return { text: '已发布', cls: 'is-pub' }
  return { text: '草稿', cls: 'is-draft' }
}

async function loadStats() {
  if (!uid.value) return

  const where = JSON.stringify({ uid: uid.value })

  const tasks = [
    // 文章：总数 / 草稿 / 待审核
    countMyArticles(uid.value).then((r) => { stats.articleTotal = toNum(r) }).catch(() => {}),
    countMyArticles(uid.value, { status: 0 }).then((r) => { stats.articleDraft = toNum(r) }).catch(() => {}),
    countMyArticles(uid.value, { audit: 0 }).then((r) => { stats.articlePending = toNum(r) }).catch(() => {}),
    // 动态：总数 / 待审核（查询自己的动态不受 audit 过滤影响）
    listMoments({ limit: 1, where }).then((r) => { stats.momentTotal = Number(r?.data?.count) || 0 }).catch(() => {}),
    listMoments({ limit: 1, where: JSON.stringify({ uid: uid.value, audit: 0 }) })
      .then((r) => { stats.momentPending = Number(r?.data?.count) || 0 }).catch(() => {}),
    // 影响力：粉丝 / 获赞 / 关注（后端只统计公开内容，获赞有 5 分钟缓存）
    call('user-follows', 'stats', { method: 'GET', params: { uid: uid.value } })
      .then((r) => {
        const d = r?.data || {}
        stats.fans = Number(d.fans) || 0
        stats.likes = Number(d.likes) || 0
        stats.follow = Number(d.follow) || 0
      })
      .catch(() => {})
  ]

  await Promise.all(tasks)
}

async function loadRecent() {
  if (!uid.value) return
  try {
    const [articleRes, momentRes] = await Promise.all([
      getMyArticles(uid.value, { page: 1, limit: 6 }),
      listMoments({ limit: 6, where: JSON.stringify({ uid: uid.value }) })
    ])

    const list = []

    ;(articleRes?.data?.data || []).forEach((item) => {
      list.push({
        kind: 'article',
        id: item.id,
        title: item.title || '无标题文章',
        time: Number(item.create_time) || 0,
        state: stateOf(item),
        to: `/manage/posts/edit/${item.id}`
      })
    })

    ;(momentRes?.data?.data || []).forEach((item) => {
      const text = String(item.content || '').replace(/\s+/g, ' ').trim()
      list.push({
        kind: 'moment',
        id: item.id,
        title: text ? (text.length > 48 ? `${text.slice(0, 48)}…` : text) : '（无文字内容）',
        time: Number(item.create_time) || 0,
        state: stateOf(item),
        // 动态没有独立编辑页，统一落到列表（编辑在弹窗里）
        to: '/manage/moments'
      })
    })

    recent.value = list.sort((a, b) => b.time - a.time).slice(0, 6)
  } catch {
    recent.value = []
  }
}

onMounted(async () => {
  await Promise.all([loadStats(), loadRecent(), loadTrend()])
  loaded.value = true
})
</script>

<style scoped>
.overview-head {
  margin-bottom: 18px;
}

/* ---------- 影响力：粉丝 / 获赞 / 关注 ---------- */
.influence-row {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 8px;
  margin-bottom: 14px;
}
.influence-item {
  display: inline-flex;
  align-items: baseline;
  gap: 6px;
  padding: 7px 12px;
  font-size: 12px;
  color: var(--text-muted);
  background: var(--bg-muted);
  border-radius: 999px;
  text-decoration: none;
  transition: background-color var(--dur) var(--ease), color var(--dur) var(--ease);
}
.influence-item:hover {
  background: var(--accent-wash);
  color: var(--primary-deep);
}
.influence-item i {
  font-size: 12px;
  opacity: 0.7;
}
.influence-item strong {
  font-size: 15px;
  font-weight: 600;
  color: var(--text);
  font-variant-numeric: tabular-nums;
}
.influence-item:hover strong {
  color: var(--primary-deep);
}
.influence-note {
  margin: 0 0 0 auto;
  font-size: 11.5px;
  color: var(--text-light);
}
.influence-note i {
  margin-right: 2px;
}
.block-title {
  font-size: 16px;
  font-weight: 600;
  margin: 0;
}
.block-desc {
  margin: 4px 0 0;
  font-size: 12px;
  color: var(--text-muted);
}

.stat-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(140px, 1fr));
  gap: 10px;
  margin-bottom: 16px;
}
/* 整卡可点：直达对应的内容列表 */
.stat-card {
  display: block;
  padding: 14px 14px 12px;
  border: 1px solid var(--border-soft);
  border-radius: var(--radius);
  background: var(--bg-card);
  text-decoration: none;
  transition: border-color var(--dur) var(--ease), box-shadow var(--dur) var(--ease),
    transform var(--dur) var(--ease);
}
.stat-card:hover {
  border-color: var(--accent-glow);
  box-shadow: var(--shadow-sm);
  transform: translateY(-1px);
  text-decoration: none;
}
/* 待审核是「需要你知道」的数字，用暖色底把它从其余统计里拎出来 */
.stat-card.is-warn {
  border-color: transparent;
  background: var(--gold-soft);
}
.stat-ico {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 26px;
  height: 26px;
  margin-bottom: 8px;
  font-size: 13px;
  color: var(--text-muted);
  background: var(--bg-muted);
  border-radius: 8px;
}
.stat-ico.is-primary {
  color: var(--primary-deep);
  background: var(--accent-wash);
}
.stat-card.is-warn .stat-ico {
  color: var(--warning);
  background: transparent;
}
.stat-num {
  display: block;
  font-size: 24px;
  font-weight: 600;
  font-variant-numeric: tabular-nums;
  color: var(--text);
  line-height: 1.15;
}
.stat-label {
  display: block;
  margin-top: 4px;
  font-size: 12px;
  color: var(--text-muted);
}
.stat-card.is-warn .stat-num {
  color: var(--warning);
}

.audit-tip {
  margin: 0 0 16px;
  padding: 8px 12px;
  font-size: 12px;
  color: var(--warning);
  background: var(--gold-wash);
  border-radius: var(--radius-sm);
}

.entry-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(300px, 1fr));
  gap: 12px;
}
.entry-card {
  display: flex;
  align-items: center;
  gap: 14px;
  padding: 16px 18px;
  border: 1px solid var(--border);
  border-radius: var(--radius);
  background: var(--bg-card);
  transition: border-color var(--dur) var(--ease), background-color var(--dur) var(--ease),
    transform var(--dur) var(--ease), box-shadow var(--dur) var(--ease);
}
.entry-card:hover {
  border-color: var(--primary);
  background: var(--accent-wash);
  transform: translateY(-2px);
  box-shadow: var(--shadow-sm);
}
.entry-primary {
  border-color: var(--accent-glow);
  background: var(--accent-wash);
}
.entry-ico {
  width: 40px;
  height: 40px;
  flex-shrink: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: var(--radius);
  background: var(--bg-muted);
  color: var(--primary-deep);
  font-size: 18px;
}
.entry-primary .entry-ico {
  background: var(--primary);
  color: #fff;
}
.entry-body {
  flex: 1;
  min-width: 0;
}
.entry-title {
  display: block;
  font-size: 14px;
  font-weight: 600;
  color: var(--text);
}
.entry-desc {
  display: block;
  margin-top: 2px;
  font-size: 12px;
  color: var(--text-muted);
  line-height: 1.5;
}
.entry-arrow {
  flex-shrink: 0;
  color: var(--text-light);
  font-size: 13px;
  transition: color var(--dur) var(--ease), transform var(--dur) var(--ease);
}
.entry-card:hover .entry-arrow {
  color: var(--primary);
  transform: translateX(2px);
}

/* ---------- 近 7 天发布趋势 ---------- */
.trend {
  margin-top: 22px;
  padding-top: 18px;
  border-top: 1px solid var(--border-soft);
}
.trend-legend {
  display: inline-flex;
  align-items: center;
  gap: 12px;
  font-size: 11.5px;
  font-weight: 400;
  color: var(--text-muted);
}
.trend-legend span {
  display: inline-flex;
  align-items: center;
  gap: 4px;
}
.trend-dot {
  display: inline-block;
  width: 8px;
  height: 8px;
  border-radius: 2px;
}
.trend-dot.is-article {
  background: var(--primary);
}
.trend-dot.is-moment {
  background: var(--primary-soft);
}

.trend-chart {
  display: flex;
  align-items: flex-end;
  gap: 8px;
  height: 104px;
  padding-top: 6px;
}
.trend-col {
  flex: 1;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 4px;
  height: 100%;
}
/* 合计数字平时隐藏，悬停才出现，避免柱图上叮叮当当一堆数字 */
.trend-total {
  font-size: 10.5px;
  color: var(--text-light);
  font-variant-numeric: tabular-nums;
  opacity: 0;
  transition: opacity var(--dur) var(--ease);
}
.trend-col:hover .trend-total {
  opacity: 1;
}
.trend-bars {
  flex: 1;
  display: flex;
  align-items: flex-end;
  justify-content: center;
  gap: 3px;
  width: 100%;
  min-height: 6px;
}
.trend-bar {
  width: 10px;
  min-height: 2px;
  border-radius: 3px 3px 1px 1px;
  transition: height var(--dur) var(--ease), opacity var(--dur) var(--ease);
}
.trend-bar.is-article {
  background: var(--primary);
}
.trend-bar.is-moment {
  background: var(--primary-soft);
}
.trend-col:hover .trend-bar {
  opacity: 0.7;
}
.trend-day {
  font-size: 10.5px;
  color: var(--text-light);
  font-variant-numeric: tabular-nums;
}
.trend-day.is-today {
  color: var(--primary-deep);
  font-weight: 600;
}
.trend-empty {
  margin: 10px 0 0;
  font-size: 11.5px;
  color: var(--text-light);
  text-align: center;
}

.recent {
  margin-top: 24px;
  padding-top: 18px;
  border-top: 1px solid var(--border-soft);
}
.recent-title {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin: 0 0 6px;
  font-size: 14px;
  font-weight: 600;
}
.recent-more {
  display: inline-flex;
  align-items: center;
  gap: 2px;
  font-size: 12px;
  font-weight: 400;
  color: var(--text-muted);
  transition: color var(--dur) var(--ease);
}
.recent-more:hover {
  color: var(--primary);
}
.recent-list {
  display: flex;
  flex-direction: column;
}
.recent-row {
  display: flex;
  align-items: center;
  gap: 10px;
  /* 负 margin 让 hover 底色略微出血，文字仍与其他内容左对齐 */
  margin: 0 -8px;
  padding: 9px 8px;
  border-bottom: 1px dashed var(--border-soft);
  border-radius: var(--radius-sm);
  font-size: 13px;
  transition: background-color var(--dur) var(--ease);
}
.recent-row:hover {
  background: var(--bg-muted);
}
.recent-row:last-child {
  border-bottom: none;
}
.recent-empty {
  display: flex;
  align-items: center;
  gap: 6px;
  margin: 0;
  padding: 16px 0;
  font-size: 13px;
  color: var(--text-muted);
}
.recent-empty a {
  color: var(--primary);
}
.recent-link {
  flex: 1;
  min-width: 0;
  color: var(--text);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.recent-link:hover {
  color: var(--primary);
}
/* 最近创作左侧的类型图标：文章用主色、动态用中性色 */
.recent-kind {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 22px;
  height: 22px;
  flex-shrink: 0;
  font-size: 11px;
  color: var(--text-muted);
  background: var(--bg-muted);
  border-radius: 6px;
}
.recent-kind.is-article {
  color: var(--primary-deep);
  background: var(--accent-wash);
}
.post-status {
  padding: 1px 7px;
  font-size: 11px;
  border-radius: 3px;
  white-space: nowrap;
}
.post-status.is-pub {
  background: rgba(108, 154, 77, 0.12);
  color: var(--success);
}
.post-status.is-draft {
  background: var(--bg-muted);
  color: var(--text-muted);
}
.post-status.is-audit {
  background: var(--gold-wash);
  color: var(--warning);
}
.post-status.is-reject {
  background: var(--accent-soft);
  color: var(--danger);
}
.recent-time {
  flex-shrink: 0;
  font-size: 12px;
  color: var(--text-muted);
}

@media (max-width: 640px) {
  .stat-grid {
    grid-template-columns: repeat(2, 1fr);
  }
  .entry-grid {
    grid-template-columns: 1fr;
  }
  .recent-time {
    display: none;
  }
  /* 窄屏放不下右对齐的说明，改成独占一行 */
  .influence-note {
    flex-basis: 100%;
    margin-left: 0;
  }
}
</style>
