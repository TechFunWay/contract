<template>
  <div class="space-y-3">
    <div class="flex items-center justify-between">
      <h3 class="text-sm font-bold text-foreground">字段设计</h3>
      <button type="button" class="btn-ghost !px-3 !py-1.5 text-xs" @click="addField">＋ 新增字段</button>
    </div>

    <p class="text-xs text-muted-foreground">点「插入正文」把字段写到光标处；生成合同时占位符会被填写值替换。</p>

    <div v-if="fields.length === 0" class="text-xs text-muted-foreground text-center py-6 border border-dashed border-border rounded-xl">
      还没有字段，点上方「新增字段」创建
    </div>

    <div v-for="(field, index) in fields" :key="index" class="border border-border rounded-xl p-3 space-y-2 bg-surface/60">
      <div class="flex items-center gap-1.5">
        <input
          :value="field.label"
          class="input-field !px-2.5 !py-1.5 text-sm flex-1"
          placeholder="字段名称，如：甲方名称"
          @input="update(index, { label: ($event.target as HTMLInputElement).value })"
        />
        <button type="button" class="editor-btn" title="上移" :disabled="index === 0" @click="move(index, -1)">
          <svg fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 15l7-7 7 7"/></svg>
        </button>
        <button type="button" class="editor-btn" title="下移" :disabled="index === fields.length - 1" @click="move(index, 1)">
          <svg fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7"/></svg>
        </button>
        <button type="button" class="editor-btn !text-red-500" title="删除字段" @click="remove(index)">
          <svg fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16"/></svg>
        </button>
      </div>

      <div class="grid grid-cols-2 gap-2">
        <select
          :value="field.type"
          class="input-field !px-2.5 !py-1.5 text-sm"
          @change="update(index, { type: ($event.target as HTMLSelectElement).value as FieldType })"
        >
          <option v-for="(label, type) in FIELD_TYPE_LABELS" :key="type" :value="type">{{ label }}</option>
        </select>
        <select
          :value="field.width || 'full'"
          class="input-field !px-2.5 !py-1.5 text-sm"
          @change="update(index, { width: ($event.target as HTMLSelectElement).value as 'full' | 'half' })"
        >
          <option value="full">整行</option>
          <option value="half">半行</option>
        </select>
      </div>

      <div v-if="field.type === 'select'" class="space-y-1">
        <input
          :value="(field.options || []).join('、')"
          class="input-field !px-2.5 !py-1.5 text-sm"
          placeholder="下拉选项，用顿号或逗号分隔"
          @input="update(index, { options: splitOptions(($event.target as HTMLInputElement).value) })"
        />
      </div>
      <div v-else>
        <input
          :value="field.placeholder"
          class="input-field !px-2.5 !py-1.5 text-sm"
          placeholder="填写提示（可留空）"
          @input="update(index, { placeholder: ($event.target as HTMLInputElement).value })"
        />
      </div>

      <div class="flex items-center justify-between">
        <label class="flex items-center gap-2 text-xs text-muted-foreground cursor-pointer">
          <input type="checkbox" :checked="field.required" class="accent-brand-500" @change="update(index, { required: ($event.target as HTMLInputElement).checked })" />
          必填
        </label>
        <button type="button" class="text-xs font-semibold text-brand-600 dark:text-brand-300 hover:underline" @click="$emit('insert', field)">
          插入正文
        </button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { FIELD_TYPE_LABELS, type FieldType, type TemplateField } from '../../api/contract'

const props = defineProps<{ fields: TemplateField[] }>()

const emit = defineEmits<{
  'update:fields': [value: TemplateField[]]
  insert: [field: TemplateField]
}>()

let seq = 0

function addField() {
  seq = 0
  for (const f of props.fields) {
    const m = /^field_(\d+)$/.exec(f.key)
    if (m) seq = Math.max(seq, Number(m[1]))
  }
  seq += 1
  const field: TemplateField = { key: `field_${seq}`, label: '', type: 'text', required: false, width: 'full' }
  emit('update:fields', [...props.fields, field])
}

function update(index: number, patch: Partial<TemplateField>) {
  const fields = props.fields.map((f, i) => (i === index ? { ...f, ...patch } : f))
  emit('update:fields', fields)
}

function remove(index: number) {
  emit(
    'update:fields',
    props.fields.filter((_, i) => i !== index),
  )
}

function move(index: number, delta: number) {
  const target = index + delta
  if (target < 0 || target >= props.fields.length) return
  const fields = [...props.fields]
  ;[fields[index], fields[target]] = [fields[target], fields[index]]
  emit('update:fields', fields)
}

function splitOptions(raw: string): string[] {
  return raw
    .split(/[、,，]/)
    .map((s) => s.trim())
    .filter(Boolean)
}
</script>

<style scoped>
.editor-btn {
  @apply w-8 h-8 shrink-0 inline-flex items-center justify-center rounded-lg text-muted-foreground hover:text-brand-600 dark:hover:text-brand-300 hover:bg-muted transition-colors disabled:opacity-30 disabled:pointer-events-none;
}
.editor-btn svg {
  @apply w-4 h-4;
}
</style>
