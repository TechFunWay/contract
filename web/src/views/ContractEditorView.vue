<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import PageHeader from '../components/PageHeader.vue'
import Modal from '../components/Modal.vue'
import FieldForm from '../components/contract/FieldForm.vue'
import ContractPreview from '../components/contract/ContractPreview.vue'
import {
  CONTRACT_STATUS_LABELS,
  createContract,
  getContract,
  getTemplates,
  updateContract,
  type ContractDetail,
  type ContractStatus,
  type TemplateField,
  type TemplateSummary,
} from '../api/contract'
import { amountToCnyUppercase } from '../utils/cny-money'

const route = useRoute()
const router = useRouter()

const isNew = computed(() => route.params.id === 'new')
const loading = ref(false)
const saving = ref(false)
const errorMsg = ref('')
const noticeMsg = ref('')

const contract = ref<ContractDetail | null>(null)
const values = ref<Record<string, string>>({})

// 手机端单栏模式下的面板切换（lg 起双栏并排，此状态不生效）
const mobilePane = ref<'form' | 'preview'>('form')
const PANES = [
  { key: 'form', label: '填写' },
  { key: 'preview', label: '预览' },
] as const

// 新建：模板选择弹窗（query.template 直达指定模板）
const pickerVisible = ref(false)
const templates = ref<TemplateSummary[]>([])
const templateKeyword = ref('')
const creating = ref(false)

const missingRequired = computed<TemplateField[]>(() => {
  if (!contract.value) return []
  return contract.value.fields.filter((f) => f.required && !(values.value[f.key] || '').trim())
})

async function loadContract(id: string) {
  loading.value = true
  errorMsg.value = ''
  try {
    const res = await getContract(id)
    if (res.data?.code === 0) {
      contract.value = res.data.data
      resetCnyState()
      values.value = { ...(res.data.data.values || {}) }
      dirtySnapshot.value = editSnapshot()
    } else {
      errorMsg.value = res.data?.message || '加载合同失败'
    }
  } catch (err: any) {
    errorMsg.value = err.response?.data?.message || '加载合同失败'
  } finally {
    loading.value = false
  }
}

async function openPicker() {
  pickerVisible.value = true
  try {
    const res = await getTemplates({ pageSize: 100, keyword: templateKeyword.value || undefined })
    if (res.data?.code === 0) {
      templates.value = res.data.data?.items || []
    }
  } catch {}
}

async function createFrom(template: TemplateSummary) {
  if (creating.value) return
  creating.value = true
  errorMsg.value = ''
  try {
    const res = await createContract({ template_id: template.id })
    if (res.data?.code === 0) {
      const id = res.data.data?.id
      pickerVisible.value = false
      router.replace(`/admin/contract-contracts/${id}`)
    } else {
      errorMsg.value = res.data?.message || '生成合同失败'
    }
  } catch (err: any) {
    errorMsg.value = err.response?.data?.message || '生成合同失败'
  } finally {
    creating.value = false
  }
}

// 未保存检测：与上次加载/保存的快照对比，驱动右下角悬浮保存按钮的出现
const dirtySnapshot = ref('')

function editSnapshot(): string {
  const c = contract.value
  if (!c) return ''
  return JSON.stringify({
    title: c.title,
    status: c.status,
    remark: c.remark,
    sign_date: c.sign_date ? c.sign_date.slice(0, 10) : '',
    values: values.value,
  })
}

const isDirty = computed(() => !!contract.value && editSnapshot() !== dirtySnapshot.value)

async function save() {
  if (!contract.value) return
  if (contract.value.status !== 'draft' && missingRequired.value.length > 0) {
    noticeMsg.value = `还有必填字段未填：${missingRequired.value.map((f) => f.label).join('、')}`
    return
  }
  saving.value = true
  errorMsg.value = ''
  noticeMsg.value = ''
  try {
    const res = await updateContract(contract.value.id, {
      title: contract.value.title,
      values: values.value,
      status: contract.value.status,
      remark: contract.value.remark,
      sign_date: contract.value.sign_date ? contract.value.sign_date.slice(0, 10) : '',
    })
    if (res.data?.code === 0) {
      contract.value = res.data.data
      dirtySnapshot.value = editSnapshot()
      noticeMsg.value = '已保存'
      setTimeout(() => {
        noticeMsg.value = ''
      }, 2000)
    } else {
      errorMsg.value = res.data?.message || '保存失败'
    }
  } catch (err: any) {
    errorMsg.value = err.response?.data?.message || '保存失败'
  } finally {
    saving.value = false
  }
}

// 金额大写自动转换：模板里同时存在「金额」（number 字段）与「大写」（text/textarea
// 字段）时，填写小写金额自动写入大写。手动改过大写后保留手动值，把大写清空即恢复自动。
const cnyPair = computed<{ src: string; dst: string } | null>(() => {
  const fields = contract.value?.fields
  if (!fields?.length) return null
  const src =
    fields.find((f) => f.type === 'number' && f.label.includes('金额')) ||
    fields.find((f) => f.type === 'number')
  const dst = fields.find(
    (f) => (f.type === 'text' || f.type === 'textarea') && f.label.includes('大写'),
  )
  if (!src || !dst || src.key === dst.key) return null
  return { src: src.key, dst: dst.key }
})

let cnyLastSrc: string | null = null
let cnyLastWritten = ''
let cnyManual = false

function resetCnyState() {
  cnyLastSrc = null
  cnyLastWritten = ''
  cnyManual = false
}

function cnyWrite(value: string) {
  const pair = cnyPair.value
  if (!pair) return
  if ((values.value[pair.dst] || '') === value) {
    cnyLastWritten = value
    return
  }
  cnyManual = false
  cnyLastWritten = value
  values.value = { ...values.value, [pair.dst]: value }
}

watch(values, () => {
  const pair = cnyPair.value
  if (!pair) return
  const src = (values.value[pair.src] || '').trim()
  const dst = values.value[pair.dst] || ''
  const auto = src ? amountToCnyUppercase(src) : ''

  // 大写与上次写入不一致 → 来自服务端数据或用户手改，据此判断是否手动值
  if (dst !== cnyLastWritten) {
    cnyManual = dst !== '' && dst !== auto
    cnyLastWritten = dst
  }
  const srcChanged = cnyLastSrc !== null && src !== cnyLastSrc
  cnyLastSrc = src

  if (!auto) {
    // 金额清空：若大写是自动写入的，同步清掉；手动改过的保留
    if (srcChanged && !cnyManual && dst !== '' && dst === cnyLastWritten) cnyWrite('')
    return
  }
  if (dst === '') {
    cnyWrite(auto)
    return
  }
  if (cnyManual || !srcChanged) return
  cnyWrite(auto)
})

function print() {
  if (contract.value) {
    window.open(`/print/contract/${contract.value.id}`, '_blank')
  }
}

// 同组件实例复用（新建 → 跳转详情只变路由参数）时重新加载。
watch(
  () => route.params.id,
  (id) => {
    if (id && id !== 'new' && (!contract.value || contract.value.id !== Number(id))) {
      loadContract(id as string)
    }
  },
)

onMounted(() => {
  if (isNew.value) {
    const preset = route.query.template
    if (preset) {
      // 从模板列表页带 template 参数跳转而来：直接生成
      createFrom({
        id: Number(preset),
        name: '',
        category: '',
        description: '',
        field_count: 0,
        contract_count: 0,
        created_at: '',
        updated_at: '',
      } as TemplateSummary)
    } else {
      openPicker()
    }
  } else {
    loadContract(route.params.id as string)
  }
})
</script>

<template>
  <div class="page-container">
    <template v-if="isNew">
      <PageHeader title="生成合同" description="选择一份模板，填写后生成合同" />
      <div class="surface rounded-2xl p-10 text-center text-sm text-muted-foreground">
        正在打开模板选择…
        <button class="btn-ghost !px-4 !py-2 ml-3" @click="openPicker">重新打开</button>
      </div>

      <Modal v-model="pickerVisible" title="选择模板" :closable="true">
        <div class="space-y-3">
          <input v-model="templateKeyword" class="input-field" placeholder="搜索模板…" @keyup.enter="openPicker" />
          <div class="max-h-80 overflow-y-auto divide-y divide-border">
            <div v-if="templates.length === 0" class="text-sm text-muted-foreground py-8 text-center">
              暂无模板，请先到「合同模板」创建
            </div>
            <button
              v-for="template in templates"
              :key="template.id"
              class="w-full text-left px-3 py-3 rounded-xl hover:bg-muted transition-colors"
              @click="createFrom(template)"
            >
              <div class="font-medium text-foreground">{{ template.name }}</div>
              <div class="text-xs text-muted-foreground mt-0.5">
                {{ template.field_count }} 个字段 · 已生成 {{ template.contract_count }} 份
              </div>
            </button>
          </div>
          <p v-if="errorMsg" class="text-sm text-red-500">{{ errorMsg }}</p>
        </div>
      </Modal>
    </template>

    <template v-else-if="loading">
      <div class="surface rounded-2xl p-10 text-center text-sm text-muted-foreground">加载中…</div>
    </template>

    <template v-else-if="contract">
      <PageHeader :title="contract.title" :description="`基于模板「${contract.template_name}」生成`">
        <template #actions>
          <select v-model="contract.status" class="input-field !w-auto !py-2 text-sm">
            <option v-for="(label, key) in CONTRACT_STATUS_LABELS" :key="key" :value="key">{{ label }}</option>
          </select>
          <button class="btn-ghost" @click="print">打印 / PDF</button>
          <button class="btn-brand" :disabled="saving" @click="save">{{ saving ? '保存中…' : '保存' }}</button>
        </template>
      </PageHeader>

      <div class="surface rounded-2xl p-5 grid grid-cols-1 md:grid-cols-3 gap-4">
        <div class="md:col-span-2">
          <label class="block text-sm font-medium text-foreground mb-1.5">合同标题</label>
          <input v-model="contract.title" class="input-field" />
        </div>
        <div>
          <label class="block text-sm font-medium text-foreground mb-1.5">签订日期</label>
          <input
            :value="contract.sign_date ? contract.sign_date.slice(0, 10) : ''"
            type="date"
            class="input-field"
            @input="contract.sign_date = ($event.target as HTMLInputElement).value"
          />
        </div>
      </div>

      <p v-if="noticeMsg" class="text-sm text-green-600 dark:text-green-400">{{ noticeMsg }}</p>
      <p v-if="errorMsg" class="text-sm text-red-500">{{ errorMsg }}</p>

      <!-- 手机端：填写/预览单栏切换；lg 起双栏并排 -->
      <div class="lg:hidden flex gap-1 p-1 rounded-xl bg-muted">
        <button
          v-for="pane in PANES"
          :key="pane.key"
          class="flex-1 py-2 rounded-lg text-sm font-medium transition-colors"
          :class="mobilePane === pane.key ? 'bg-surface text-brand-600 dark:text-brand-300 shadow-sm' : 'text-muted-foreground'"
          @click="mobilePane = pane.key"
        >
          {{ pane.label }}
        </button>
      </div>

      <div class="grid grid-cols-1 lg:grid-cols-2 gap-4 items-start">
        <div class="surface rounded-2xl p-5 space-y-4" :class="mobilePane === 'form' ? '' : 'hidden lg:block'">
          <h3 class="text-sm font-bold text-foreground">填写内容</h3>
          <p class="text-xs text-muted-foreground">生成合同后修改模板不会影响本合同；留空的字段在打印时显示为空位。</p>
          <FieldForm v-model="values" :fields="contract.fields" />
          <div>
            <label class="block text-sm font-medium text-foreground mb-1.5">备注</label>
            <textarea v-model="contract.remark" class="input-field min-h-[64px]" placeholder="内部备注，不进正文（可留空）"></textarea>
          </div>
          <p v-if="contract.status !== 'draft' && missingRequired.length > 0" class="text-xs text-yellow-600 dark:text-yellow-400">
            必填未填：{{ missingRequired.map((f) => f.label).join('、') }}（保存草稿不受影响）
          </p>
        </div>
        <ContractPreview
          :content="contract.content"
          :fields="contract.fields"
          :values="values"
          class="lg:sticky lg:top-4"
          :class="mobilePane === 'preview' ? '' : 'hidden lg:block'"
        />
      </div>

      <!-- 右下角悬浮保存：有未保存修改时出现，免去滑回顶部 -->
      <div
        v-if="isDirty"
        class="fixed right-4 bottom-4 sm:right-6 sm:bottom-6 z-40 flex items-center gap-3 rounded-2xl border border-border bg-surface/95 backdrop-blur px-4 py-3 shadow-lg shadow-black/20"
      >
        <span class="hidden sm:flex items-center gap-1.5 text-xs text-muted-foreground">
          <span class="w-1.5 h-1.5 rounded-full bg-amber-500"></span>
          有未保存的修改
        </span>
        <button class="btn-brand !py-2 !px-5" :disabled="saving" @click="save">
          {{ saving ? '保存中…' : '保存' }}
        </button>
      </div>
    </template>

    <div v-else class="surface rounded-2xl p-10 text-center text-sm text-red-500">{{ errorMsg || '合同不存在' }}</div>
  </div>
</template>
