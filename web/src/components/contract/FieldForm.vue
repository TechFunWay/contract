<template>
  <div class="grid grid-cols-1 sm:grid-cols-2 gap-3">
    <div
      v-for="field in fields"
      :key="field.key"
      :class="field.width === 'half' ? '' : 'sm:col-span-2'"
    >
      <label class="block text-sm font-medium text-foreground mb-1.5">
        {{ field.label }}
        <span v-if="field.required" class="text-seal">*</span>
      </label>

      <textarea
        v-if="field.type === 'textarea'"
        :value="modelValue[field.key] || ''"
        class="input-field min-h-[72px]"
        :placeholder="field.placeholder || `请输入${field.label}`"
        @input="set(field.key, ($event.target as HTMLTextAreaElement).value)"
      ></textarea>

      <select
        v-else-if="field.type === 'select'"
        :value="modelValue[field.key] || ''"
        class="input-field"
        @change="set(field.key, ($event.target as HTMLSelectElement).value)"
      >
        <option value="" disabled>请选择{{ field.label }}</option>
        <option v-for="opt in field.options || []" :key="opt" :value="opt">{{ opt }}</option>
      </select>

      <input
        v-else
        :type="inputType(field.type)"
        :value="modelValue[field.key] || ''"
        class="input-field"
        :placeholder="field.placeholder || `请输入${field.label}`"
        @input="set(field.key, ($event.target as HTMLInputElement).value)"
      />
    </div>
  </div>
</template>

<script setup lang="ts">
import type { TemplateField } from '../../api/contract'

const props = defineProps<{
  fields: TemplateField[]
  modelValue: Record<string, string>
}>()

const emit = defineEmits<{
  'update:modelValue': [value: Record<string, string>]
}>()

function set(key: string, value: string) {
  emit('update:modelValue', { ...props.modelValue, [key]: value })
}

function inputType(type: TemplateField['type']): string {
  if (type === 'number') return 'number'
  if (type === 'date') return 'date'
  return 'text'
}
</script>
