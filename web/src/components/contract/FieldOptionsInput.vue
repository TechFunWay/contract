<template>
  <div class="space-y-1">
    <input
      :value="raw"
      class="input-field !px-2.5 !py-1.5 text-sm"
      placeholder="下拉选项，用顿号、逗号或空格分隔"
      @input="onInput"
    />
    <p class="text-[11px] leading-snug text-muted-foreground">
      多个选项用顿号、逗号或空格隔开（如 <code>现金 转账</code>）；选项本身要含这些字符时用引号包住，如
      <code>“北京市, 上海市”</code>。
    </p>
  </div>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue'
import { joinOptions, splitOptions } from '../../utils/field-options'

const props = defineProps<{ options?: string[] }>()

const emit = defineEmits<{
  'update:options': [value: string[]]
}>()

// 本地原始文本：输入过程中绝不回填。旧实现拿 options.join('、') 覆盖输入框，
// 逗号/空格一敲进去就被吃掉（粉丝反馈「没法空格和填逗号」的根因）。这里只在
// 选项数组被外部真正改动（字段排序、类型切换、数据加载）时才同步文本，
// 自己敲出来的文本解析结果与外部一致时保持原样，避免刚敲的分隔符又被吃。
const raw = ref(joinOptions(props.options))

watch(
  () => joinOptions(props.options),
  (join) => {
    if (join !== joinOptions(splitOptions(raw.value))) {
      raw.value = join
    }
  },
)

function onInput(e: Event) {
  raw.value = (e.target as HTMLInputElement).value
  emit('update:options', splitOptions(raw.value))
}
</script>
