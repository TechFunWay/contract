<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import PageHeader from '../components/PageHeader.vue'
import RichEditor from '../components/contract/RichEditor.vue'
import FieldDesigner from '../components/contract/FieldDesigner.vue'
import ContractPreview from '../components/contract/ContractPreview.vue'
import {
  createTemplate,
  getTemplate,
  updateTemplate,
  type TemplateField,
} from '../api/contract'
import { fieldPlaceholderHTML } from '../utils/contract-render'

const route = useRoute()
const router = useRouter()

const isNew = computed(() => route.params.id === 'new')
const loading = ref(!isNew.value)
const saving = ref(false)
const errorMsg = ref('')

const name = ref('')
const category = ref('')
const description = ref('')
const content = ref('')
const fields = ref<TemplateField[]>([])
const previewing = ref(false)

const editorRef = ref<InstanceType<typeof RichEditor>>()

onMounted(async () => {
  if (isNew.value) return
  try {
    const res = await getTemplate(route.params.id as string)
    if (res.data?.code === 0) {
      const detail = res.data.data
      name.value = detail.name || ''
      category.value = detail.category || ''
      description.value = detail.description || ''
      content.value = detail.content || ''
      fields.value = detail.fields || []
    } else {
      errorMsg.value = res.data?.message || '加载模板失败'
    }
  } catch (err: any) {
    errorMsg.value = err.response?.data?.message || '加载模板失败'
  } finally {
    loading.value = false
  }
})

function insertField(field: TemplateField) {
  editorRef.value?.insertHTML(fieldPlaceholderHTML(field))
}

// 字段改名时同步刷新正文里对应占位符的展示文案。
function onFieldsChange(next: TemplateField[]) {
  const before = new Map(fields.value.map((f) => [f.key, f.label]))
  fields.value = next
  const dirty = next.some((f) => before.get(f.key) !== undefined && before.get(f.key) !== f.label)
  if (dirty && editorRef.value) {
    const el = (editorRef.value.$el as HTMLElement).querySelector('.editor-body')
    if (el) {
      for (const span of el.querySelectorAll<HTMLElement>('.ct-field')) {
        const key = span.dataset.field || ''
        const field = next.find((f) => f.key === key)
        if (field) {
          span.textContent = `【${field.label}】`
          content.value = el.innerHTML
        }
      }
    }
  }
}

async function save() {
  if (!name.value.trim()) {
    errorMsg.value = '请填写模板名称'
    return
  }
  saving.value = true
  errorMsg.value = ''
  try {
    const payload = {
      name: name.value,
      category: category.value,
      description: description.value,
      content: content.value,
      fields: fields.value,
    }
    const res = isNew.value
      ? await createTemplate(payload)
      : await updateTemplate(route.params.id as string, payload)
    if (res.data?.code === 0) {
      router.push('/admin/contract-templates')
    } else {
      errorMsg.value = res.data?.message || '保存失败'
    }
  } catch (err: any) {
    errorMsg.value = err.response?.data?.message || '保存失败'
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <div class="page-container">
    <PageHeader :title="isNew ? '新建模板' : '编辑模板'" description="左侧写正文并插入字段占位符，右侧设计字段">
      <template #actions>
        <button class="btn-ghost" @click="previewing = !previewing">{{ previewing ? '返回编辑' : '预览' }}</button>
        <button class="btn-brand" :disabled="saving" @click="save">{{ saving ? '保存中…' : '保存模板' }}</button>
      </template>
    </PageHeader>

    <div v-if="loading" class="surface rounded-2xl p-10 text-center text-sm text-muted-foreground">加载中…</div>

    <template v-else>
      <div class="surface rounded-2xl p-5 grid grid-cols-1 md:grid-cols-3 gap-4">
        <div>
          <label class="block text-sm font-medium text-foreground mb-1.5">模板名称 <span class="text-red-500">*</span></label>
          <input v-model="name" class="input-field" placeholder="如：房屋租赁合同" />
        </div>
        <div>
          <label class="block text-sm font-medium text-foreground mb-1.5">分类</label>
          <input v-model="category" class="input-field" placeholder="如：租赁合同 / 劳动合同 / 自定义" />
        </div>
        <div>
          <label class="block text-sm font-medium text-foreground mb-1.5">描述</label>
          <input v-model="description" class="input-field" placeholder="模板用途说明（可留空）" />
        </div>
      </div>

      <div v-if="errorMsg" class="text-sm text-red-500">{{ errorMsg }}</div>

      <div v-if="previewing">
        <ContractPreview :content="content" :fields="fields" :values="{}" />
      </div>

      <div v-else class="grid grid-cols-1 lg:grid-cols-3 gap-4 items-start">
        <div class="lg:col-span-2">
          <RichEditor ref="editorRef" v-model:content="content" />
        </div>
        <div class="surface rounded-2xl p-4 lg:sticky lg:top-4">
          <FieldDesigner :fields="fields" @update:fields="onFieldsChange" @insert="insertField" />
        </div>
      </div>
    </template>
  </div>
</template>
