<template>
  <div class="page-container animate-fade-in">
    <!-- Hero -->
    <div class="relative overflow-hidden rounded-2xl bg-brand-gradient p-8 sm:p-10 shadow-glow">
      <div class="absolute -top-16 -right-16 w-64 h-64 rounded-full bg-white/10 blur-2xl"></div>
      <div class="absolute -bottom-20 -left-10 w-56 h-56 rounded-full bg-black/10 blur-2xl"></div>
      <div class="relative">
        <p class="text-white/80 text-sm font-medium">{{ greeting }}，欢迎回来 👋</p>
        <h1 class="text-2xl sm:text-3xl font-extrabold text-white mt-2">{{ authStore.user?.username || '用户' }}</h1>
        <p class="text-white/80 mt-3 max-w-xl">{{ authStore.siteTitle }} —— 把常用合同沉淀为模板，填表即得，一键打印导出 PDF。</p>
        <div class="flex flex-wrap gap-3 mt-6">
          <RouterLink to="/admin/contract-templates" class="inline-flex items-center gap-2 px-4 py-2.5 rounded-xl bg-white text-brand-700 text-sm font-semibold hover:bg-white/90 active:scale-[0.98] transition-all">
            管理模板
          </RouterLink>
          <RouterLink to="/admin/contract-contracts/new" class="inline-flex items-center gap-2 px-4 py-2.5 rounded-xl bg-white/15 text-white text-sm font-semibold backdrop-blur hover:bg-white/25 active:scale-[0.98] transition-all">
            生成合同
          </RouterLink>
        </div>
      </div>
    </div>

    <!-- Feature cards -->
    <div class="grid grid-cols-1 sm:grid-cols-2 xl:grid-cols-4 gap-4">
      <div v-for="t in features" :key="t.name" class="surface rounded-2xl p-6 hover:-translate-y-0.5 transition-transform cursor-pointer" @click="t.to && router.push(t.to)">
        <div class="w-11 h-11 rounded-xl flex items-center justify-center mb-4" :class="t.bg">
          <span v-html="t.icon" class="w-6 h-6 block" :class="t.fg"></span>
        </div>
        <div class="text-lg font-bold text-foreground">{{ t.name }}</div>
        <div class="text-sm text-muted-foreground mt-1">{{ t.desc }}</div>
      </div>
    </div>

    <!-- Workflow -->
    <div class="surface rounded-2xl p-6 sm:p-8">
      <h3 class="text-lg font-bold text-foreground mb-5">使用流程</h3>
      <div class="grid grid-cols-1 sm:grid-cols-2 xl:grid-cols-4 gap-x-8 gap-y-3">
        <div v-for="(step, i) in steps" :key="step" class="flex items-center gap-3 text-sm text-foreground/80">
          <span class="w-6 h-6 rounded-full bg-brand-gradient text-white text-xs font-bold flex items-center justify-center shrink-0">{{ i + 1 }}</span>
          {{ step }}
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { RouterLink, useRouter } from 'vue-router'
import { useAuthStore } from '../stores/auth'

const authStore = useAuthStore()
const router = useRouter()

const greeting = computed(() => {
  const h = new Date().getHours()
  if (h < 6) return '凌晨好'
  if (h < 12) return '早上好'
  if (h < 14) return '中午好'
  if (h < 18) return '下午好'
  return '晚上好'
})

const features = [
  {
    name: '合同模板',
    desc: '富文本正文 + 字段占位符，保存为模板反复使用',
    to: '/admin/contract-templates',
    bg: 'bg-brand-50 dark:bg-brand-500/15',
    fg: 'text-brand-600 dark:text-brand-300',
    icon: '<svg fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z"/></svg>',
  },
  {
    name: '自定义字段',
    desc: '文本、数字、日期、下拉，必填与布局随心配',
    to: '/admin/contract-templates',
    bg: 'bg-emerald-50 dark:bg-emerald-500/15',
    fg: 'text-emerald-600 dark:text-emerald-300',
    icon: '<svg fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z"/></svg>',
  },
  {
    name: '生成合同',
    desc: '按模板填表即得，留空字段自动变成手写空位',
    to: '/admin/contract-contracts',
    bg: 'bg-amber-50 dark:bg-amber-500/15',
    fg: 'text-amber-600 dark:text-amber-300',
    icon: '<svg fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2"/></svg>',
  },
  {
    name: '导出 PDF',
    desc: 'A4 版式打印视图，浏览器一键打印或另存 PDF',
    to: '/admin/contract-contracts',
    bg: 'bg-seal/10',
    fg: 'text-seal',
    icon: '<svg fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M17 9V7a2 2 0 00-2-2H5a2 2 0 00-2 2v6a2 2 0 002 2h2m2 4h10a2 2 0 002-2v-6a2 2 0 00-2-2H9a2 2 0 00-2 2v6a2 2 0 002 2zm7-5v5"/></svg>',
  },
]

const steps = [
  '设计正文并插入字段占位符',
  '保存为合同模板',
  '从模板生成合同并填写',
  '打印导出 PDF 或留档跟踪',
]
</script>
