<template>
  <div class="surface rounded-2xl overflow-hidden">
    <div class="px-5 py-3 border-b border-border bg-muted/30 flex items-center justify-between">
      <span class="text-sm font-bold text-foreground">实时预览</span>
      <span class="text-xs text-muted-foreground">空字段渲染为下划线空位</span>
    </div>
    <div class="contract-preview px-6 py-5 max-h-[70vh] overflow-y-auto" v-html="html"></div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { renderContent } from '../../utils/contract-render'
import type { TemplateField } from '../../api/contract'

const props = defineProps<{
  content: string
  fields: TemplateField[]
  values: Record<string, string>
}>()

const html = computed(() => renderContent(props.content, props.fields, props.values))
</script>

<style scoped>
.contract-preview :deep(.ct-value) {
  @apply text-brand-600 dark:text-brand-300 font-medium;
}
.contract-preview :deep(.ct-blank) {
  @apply inline-block w-24 border-b border-foreground/60;
  /* 与打印页一致：横线低于基线落到行底，横线上方留手写空间 */
  vertical-align: -0.3em;
}
.contract-preview :deep(h1) {
  @apply text-2xl font-bold text-center my-4;
}
.contract-preview :deep(h2) {
  @apply text-xl font-bold my-3;
}
.contract-preview :deep(h3) {
  @apply text-lg font-bold my-2;
}
.contract-preview :deep(p) {
  @apply my-2 leading-relaxed;
}
.contract-preview :deep(table) {
  @apply w-full my-3;
}
.contract-preview :deep(td),
.contract-preview :deep(th) {
  @apply border border-border px-3 py-2;
}
</style>
