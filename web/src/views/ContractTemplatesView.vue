<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import PageHeader from '../components/PageHeader.vue'
import ConfirmDialog from '../components/ConfirmDialog.vue'
import {
  deleteTemplate,
  duplicateTemplate,
  getTemplates,
  type TemplateSummary,
} from '../api/contract'

const router = useRouter()

const templates = ref<TemplateSummary[]>([])
const total = ref(0)
const page = ref(1)
const pageSize = 12
const keyword = ref('')
const loading = ref(false)
const errorMsg = ref('')

const deleteTarget = ref<TemplateSummary | null>(null)
const deleteVisible = ref(false)
const deleteMsg = ref('')
const deleting = ref(false)

async function load() {
  loading.value = true
  errorMsg.value = ''
  try {
    const res = await getTemplates({ page: page.value, pageSize, keyword: keyword.value || undefined })
    if (res.data?.code === 0) {
      templates.value = res.data.data?.items || []
      total.value = res.data.data?.total || 0
    } else {
      errorMsg.value = res.data?.message || '加载模板失败'
    }
  } catch (err: any) {
    errorMsg.value = err.response?.data?.message || '加载模板失败'
  } finally {
    loading.value = false
  }
}

function search() {
  page.value = 1
  load()
}

function prevPage() {
  if (page.value > 1) {
    page.value--
    load()
  }
}

function nextPage() {
  if (page.value * pageSize < total.value) {
    page.value++
    load()
  }
}

async function copyTemplate(template: TemplateSummary) {
  try {
    const res = await duplicateTemplate(template.id)
    if (res.data?.code === 0) {
      await load()
    }
  } catch {}
}

function confirmDelete() {
  if (!deleteTarget.value) return
  deleting.value = true
  deleteTemplate(deleteTarget.value.id)
    .then(() => {
      deleteVisible.value = false
      return load()
    })
    .catch((err: any) => {
      deleteMsg.value = err.response?.data?.message || '删除失败'
    })
    .finally(() => {
      deleting.value = false
    })
}

function openDelete(template: TemplateSummary) {
  deleteTarget.value = template
  deleteMsg.value =
    template.contract_count > 0
      ? `已有 ${template.contract_count} 份合同基于此模板生成。删除模板不影响这些合同（合同保留自己的快照），确认删除「${template.name}」？`
      : `确认删除模板「${template.name}」？`
  deleteVisible.value = true
}

function newContractFrom(template: TemplateSummary) {
  router.push({ path: '/admin/contract-contracts/new', query: { template: String(template.id) } })
}

onMounted(load)
</script>

<template>
  <div class="page-container">
    <PageHeader title="合同模板" description="沉淀常用合同：富文本正文 + 自定义字段，随用随生成">
      <template #actions>
        <button class="btn-brand" @click="router.push('/admin/contract-templates/new')">＋ 新建模板</button>
      </template>
    </PageHeader>

    <div class="surface rounded-2xl p-5 sm:p-6 space-y-4">
      <form class="flex gap-3" @submit.prevent="search">
        <input v-model="keyword" class="input-field flex-1" placeholder="搜索模板名称、分类或描述…" />
        <button type="submit" class="btn-ghost !px-5 whitespace-nowrap">搜索</button>
      </form>

      <div v-if="loading" class="text-sm text-muted-foreground py-10 text-center">加载中…</div>
      <div v-else-if="errorMsg" class="text-sm text-red-500 py-10 text-center">{{ errorMsg }}</div>
      <div v-else-if="templates.length === 0" class="text-sm text-muted-foreground py-10 text-center">
        没有模板，点右上角「新建模板」创建第一份
      </div>

      <div v-else class="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-4">
        <div
          v-for="template in templates"
          :key="template.id"
          class="border border-border rounded-2xl p-4 flex flex-col gap-3 bg-surface/60 hover:border-brand-500/50 transition-colors"
        >
          <div class="min-w-0">
            <div class="flex items-center gap-2 flex-wrap">
              <h3 class="font-bold text-foreground truncate">{{ template.name }}</h3>
              <span v-if="template.category" class="badge bg-brand-500/10 text-brand-600 dark:text-brand-300">{{ template.category }}</span>
            </div>
            <p class="text-xs text-muted-foreground mt-1 line-clamp-2 min-h-[2em]">{{ template.description || '（无描述）' }}</p>
          </div>

          <div class="flex items-center gap-3 text-xs text-muted-foreground">
            <span>{{ template.field_count }} 个字段</span>
            <span>{{ template.contract_count }} 份合同</span>
            <span class="ml-auto">{{ new Date(template.updated_at).toLocaleDateString('zh-CN') }}</span>
          </div>

          <div class="flex items-center gap-2 pt-1 border-t border-border">
            <button class="btn-brand !px-3 !py-2 text-xs" @click="newContractFrom(template)">生成合同</button>
            <button class="btn-ghost !px-3 !py-2 text-xs" @click="router.push(`/admin/contract-templates/${template.id}`)">编辑</button>
            <button class="btn-ghost !px-3 !py-2 text-xs" @click="copyTemplate(template)">复制</button>
            <button class="ml-auto text-xs text-red-500 hover:underline" @click="openDelete(template)">删除</button>
          </div>
        </div>
      </div>

      <div v-if="total > pageSize" class="flex items-center justify-between text-sm text-muted-foreground">
        <span>共 {{ total }} 份</span>
        <div class="flex gap-2 items-center">
          <button class="btn-secondary !px-3 !py-1" :disabled="page <= 1" @click="prevPage">上一页</button>
          <span class="px-2 py-1">第 {{ page }} 页</span>
          <button class="btn-secondary !px-3 !py-1" :disabled="page * pageSize >= total" @click="nextPage">下一页</button>
        </div>
      </div>
    </div>

    <ConfirmDialog
      v-model="deleteVisible"
      title="删除模板"
      :message="deleteMsg"
      confirm-text="删除"
      confirm-type="danger"
      @confirm="confirmDelete"
    />
  </div>
</template>
