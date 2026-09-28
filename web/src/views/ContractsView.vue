<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import PageHeader from '../components/PageHeader.vue'
import ConfirmDialog from '../components/ConfirmDialog.vue'
import {
  CONTRACT_STATUS_LABELS,
  deleteContract,
  duplicateContract,
  getContracts,
  type ContractStatus,
  type ContractSummary,
} from '../api/contract'

const router = useRouter()

const contracts = ref<ContractSummary[]>([])
const total = ref(0)
const page = ref(1)
const pageSize = 20
const keyword = ref('')
const status = ref('')
const loading = ref(false)
const errorMsg = ref('')

const statusTabs: { value: string; label: string }[] = [
  { value: '', label: '全部' },
  { value: 'draft', label: '草稿' },
  { value: 'active', label: '生效' },
  { value: 'archived', label: '归档' },
]

const deleteTarget = ref<ContractSummary | null>(null)
const deleteVisible = ref(false)

const statusBadge: Record<ContractStatus, string> = {
  draft: 'bg-yellow-500/10 text-yellow-600 dark:text-yellow-400',
  active: 'bg-seal/10 text-seal', // 印章红：生效即落章
  archived: 'bg-muted text-muted-foreground',
}

async function load() {
  loading.value = true
  errorMsg.value = ''
  try {
    const res = await getContracts({
      page: page.value,
      pageSize,
      keyword: keyword.value || undefined,
      status: status.value || undefined,
    })
    if (res.data?.code === 0) {
      contracts.value = res.data.data?.items || []
      total.value = res.data.data?.total || 0
    } else {
      errorMsg.value = res.data?.message || '加载合同失败'
    }
  } catch (err: any) {
    errorMsg.value = err.response?.data?.message || '加载合同失败'
  } finally {
    loading.value = false
  }
}

function search() {
  page.value = 1
  load()
}

function switchStatus(value: string) {
  status.value = value
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

async function copyContract(contract: ContractSummary) {
  try {
    await duplicateContract(contract.id)
    await load()
  } catch {}
}

function removeContract() {
  if (!deleteTarget.value) return
  deleteContract(deleteTarget.value.id)
    .then(() => load())
    .catch(() => {})
    .finally(() => {
      deleteVisible.value = false
    })
}

function printContract(contract: ContractSummary) {
  window.open(`/print/contract/${contract.id}`, '_blank')
}

onMounted(load)
</script>

<template>
  <div class="page-container">
    <PageHeader title="我的合同" description="从模板生成的合同：填写、打印导出、状态跟踪">
      <template #actions>
        <button class="btn-brand" @click="router.push('/admin/contract-contracts/new')">＋ 生成合同</button>
      </template>
    </PageHeader>

    <div class="surface rounded-2xl p-5 sm:p-6 space-y-4">
      <div class="flex flex-wrap gap-2">
        <button
          v-for="tab in statusTabs"
          :key="tab.value"
          class="px-4 py-1.5 rounded-full text-sm font-medium transition-colors"
          :class="status === tab.value ? 'bg-brand-500 text-white' : 'bg-muted text-muted-foreground hover:text-foreground'"
          @click="switchStatus(tab.value)"
        >
          {{ tab.label }}
        </button>
      </div>

      <form class="flex gap-3" @submit.prevent="search">
        <input v-model="keyword" class="input-field flex-1" placeholder="搜索标题、模板名或备注…" />
        <button type="submit" class="btn-ghost !px-5 whitespace-nowrap">搜索</button>
      </form>

      <div v-if="loading" class="text-sm text-muted-foreground py-10 text-center">加载中…</div>
      <div v-else-if="errorMsg" class="text-sm text-red-500 py-10 text-center">{{ errorMsg }}</div>
      <div v-else-if="contracts.length === 0" class="text-sm text-muted-foreground py-10 text-center">
        还没有合同，点右上角「生成合同」从模板创建
      </div>

      <ul v-else class="divide-y divide-border">
        <li v-for="contract in contracts" :key="contract.id" class="py-3 flex flex-wrap items-center gap-2">
          <div class="min-w-0 flex-1">
            <div class="flex items-center gap-2 flex-wrap">
              <button
                class="font-medium text-foreground hover:text-brand-600 dark:hover:text-brand-300 truncate"
                @click="router.push(`/admin/contract-contracts/${contract.id}`)"
              >
                {{ contract.title }}
              </button>
              <span class="badge" :class="statusBadge[contract.status]">{{ CONTRACT_STATUS_LABELS[contract.status] }}</span>
            </div>
            <p class="text-xs text-muted-foreground mt-1">
              模板：{{ contract.template_name }}
              <template v-if="contract.sign_date"> · 签订：{{ contract.sign_date.slice(0, 10) }}</template>
              · {{ new Date(contract.updated_at).toLocaleString('zh-CN') }}
            </p>
          </div>
          <div class="flex items-center gap-1.5">
            <button class="btn-ghost !px-3 !py-2 text-xs" @click="router.push(`/admin/contract-contracts/${contract.id}`)">填写</button>
            <button class="btn-ghost !px-3 !py-2 text-xs" @click="printContract(contract)">打印 / PDF</button>
            <button class="btn-ghost !px-3 !py-2 text-xs" @click="copyContract(contract)">复制</button>
            <button class="text-xs text-red-500 hover:underline px-2" @click="deleteTarget = contract; deleteVisible = true">删除</button>
          </div>
        </li>
      </ul>

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
      title="删除合同"
      :message="`确认删除合同「${deleteTarget?.title}」？删除后不可恢复。`"
      confirm-text="删除"
      confirm-type="danger"
      @confirm="removeContract"
    />
  </div>
</template>
