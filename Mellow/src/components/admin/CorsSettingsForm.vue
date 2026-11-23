<template>
  <section class="cors-panel">
    <header class="cors-head">
      <div>
        <h3 class="cors-title">跨域（CORS）</h3>
        <p class="cors-desc">
          控制哪些外部站点可以带凭证调用本站接口。后端中间件每次请求都会读取这份配置，
          <strong>保存后即时生效</strong>，不需要重启。
        </p>
      </div>
      <button type="button" class="btn btn-sm" :disabled="loading" @click="load">
        <i class="bi bi-arrow-clockwise" /> 重新加载
      </button>
    </header>

    <div v-if="loading" class="is-loading"><span class="spinner" /> 加载中…</div>

    <form v-else class="cors-form" @submit.prevent="save">
      <div class="form-item">
        <FormCheckbox v-model="form.enabled" variant="switch" label="启用 CORS" />
        <p class="form-hint">
          关闭后所有响应都不再带 CORS 头（同源访问不受影响；外部站点将无法跨域读取接口）
        </p>
      </div>

      <div class="form-item">
        <label class="form-label">允许的来源</label>
        <FormTextarea
          v-model="form.allowed_origins"
          :rows="4"
          placeholder="https://example.com&#10;http://localhost:5173"
        />
        <p class="form-hint">
          每行一个，或用英文逗号分隔。必须带协议（http / https）和端口；
          不支持 <code>*</code>（携带凭证时浏览器会拒绝通配符）
        </p>
      </div>

      <div class="cors-grid">
        <div class="form-item">
          <label class="form-label">允许的方法</label>
          <FormInput v-model="form.allowed_methods" placeholder="GET, POST, PUT, DELETE, OPTIONS" />
        </div>
        <div class="form-item">
          <label class="form-label">预检缓存（秒）</label>
          <FormInput v-model.number="form.max_age" type="number" min="1" />
          <p class="form-hint">浏览器缓存 OPTIONS 预检结果的时间</p>
        </div>
      </div>

      <div class="form-item">
        <label class="form-label">允许的请求头</label>
        <FormTextarea v-model="form.allowed_headers" :rows="3" placeholder="Content-Type, Authorization" />
      </div>

      <div class="form-item">
        <label class="form-label">暴露的响应头</label>
        <FormTextarea v-model="form.exposed_headers" :rows="3" />
        <p class="form-hint">跨域时前端能读到的响应头（其余头会被浏览器隐藏）</p>
      </div>

      <div class="form-item">
        <FormCheckbox v-model="form.allow_credentials" variant="switch" label="允许携带凭证" />
        <p class="form-hint">
          对应 <code>Access-Control-Allow-Credentials</code>；开启后「允许的来源」不能使用通配符
        </p>
      </div>

      <div class="form-item">
        <label class="form-label">默认来源</label>
        <FormInput v-model="form.default_origin" placeholder="http://localhost:8642" />
        <p class="form-hint">请求没有 Origin 头（同源 / 直接访问）时回显的来源</p>
      </div>

      <div class="cors-actions">
        <button type="submit" class="btn" :disabled="saving">
          <span v-if="saving" class="spinner spinner-sm" />
          {{ saving ? '保存中…' : '保存配置' }}
        </button>
        <span class="cors-tip">保存后立即生效，无需重启服务</span>
      </div>
    </form>
  </section>
</template>

<script setup>
/**
 * 跨域（CORS）配置表单
 *
 * 对应 config/app.toml 的 [system.cors] 段（中间件见 app/middleware/cors.go）。
 * 后端 putCors 只重写该段，不会碰 app.toml 里的其它配置和注释。
 */
import { onMounted, reactive, ref } from 'vue'
import FormCheckbox from '@/components/form/FormCheckbox.vue'
import FormInput from '@/components/form/FormInput.vue'
import FormTextarea from '@/components/form/FormTextarea.vue'
import { getTomlCors, saveTomlCors } from '@/api/toml'
import { toast } from '@/utils/toast'

const loading = ref(false)
const saving = ref(false)

const form = reactive({
  enabled: true,
  allowed_origins: '',
  allowed_methods: '',
  allowed_headers: '',
  exposed_headers: '',
  allow_credentials: true,
  max_age: 1800,
  default_origin: ''
})

async function load() {
  loading.value = true
  try {
    const res = await getTomlCors()
    const data = res?.data || {}

    // 后端返回原始值；bool 用 !== false 判断，避免缺字段时误关
    form.enabled = data.enabled !== false
    form.allowed_origins = String(data.allowed_origins || '')
    form.allowed_methods = String(data.allowed_methods || '')
    form.allowed_headers = String(data.allowed_headers || '')
    form.exposed_headers = String(data.exposed_headers || '')
    form.allow_credentials = data.allow_credentials !== false
    form.max_age = Number(data.max_age) || 1800
    form.default_origin = String(data.default_origin || '')
  } catch {
    /* 请求失败已由 request.js 提示 */
  } finally {
    loading.value = false
  }
}

async function save() {
  if (saving.value) return
  saving.value = true
  try {
    const res = await saveTomlCors({
      enabled: form.enabled,
      allowed_origins: form.allowed_origins,
      allowed_methods: form.allowed_methods,
      allowed_headers: form.allowed_headers,
      exposed_headers: form.exposed_headers,
      allow_credentials: form.allow_credentials,
      max_age: form.max_age,
      default_origin: form.default_origin
    })
    if (res?.code !== 200) return
    toast.success(res.msg || '跨域配置已保存')
    await load()
  } finally {
    saving.value = false
  }
}

onMounted(load)
</script>

<style scoped>
.cors-panel {
  padding: 18px 20px 20px;
  background: var(--bg-card);
  border: 1px solid var(--border);
  border-radius: var(--radius);
}
.cors-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 14px;
  margin-bottom: 16px;
}
.cors-title {
  margin: 0 0 4px;
  font-size: 15px;
  font-weight: 600;
  color: var(--text);
}
.cors-desc {
  margin: 0;
  max-width: 720px;
  font-size: 12.5px;
  line-height: 1.7;
  color: var(--text-muted);
}
.cors-form {
  display: flex;
  flex-direction: column;
}
.cors-grid {
  display: grid;
  grid-template-columns: 1fr 180px;
  gap: 14px;
}
.cors-actions {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-top: 4px;
  padding-top: 14px;
  border-top: 1px solid var(--border-soft);
}
.cors-tip {
  font-size: 12px;
  color: var(--text-light);
}
.cors-panel .form-hint code {
  padding: 1px 4px;
  font-size: 11.5px;
  background: var(--bg-muted);
  border-radius: 4px;
}
@media (max-width: 640px) {
  .cors-grid {
    grid-template-columns: 1fr;
  }
  .cors-head {
    flex-direction: column;
  }
}
</style>
