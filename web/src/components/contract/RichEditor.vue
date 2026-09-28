<template>
  <div class="rich-editor surface rounded-2xl overflow-hidden">
    <div class="flex flex-wrap items-center gap-1 px-3 py-2 border-b border-border bg-muted/30">
      <button
        v-for="cmd in commands"
        :key="cmd.cmd"
        type="button"
        class="editor-btn"
        :title="cmd.title"
        @mousedown.prevent
        @click="exec(cmd.cmd, cmd.arg)"
        v-html="cmd.icon"
      ></button>
      <span class="w-px h-5 bg-border mx-1"></span>
      <select
        class="editor-select"
        title="段落格式"
        @change="exec('formatBlock', ($event.target as HTMLSelectElement).value)"
      >
        <option value="p">正文</option>
        <option value="h1">标题 1</option>
        <option value="h2">标题 2</option>
        <option value="h3">标题 3</option>
        <option value="blockquote">引用</option>
      </select>
      <span class="w-px h-5 bg-border mx-1"></span>
      <button type="button" class="editor-btn" title="清除格式" @mousedown.prevent @click="exec('removeFormat')">
        <svg fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 18v-6m0-3h.01M5.6 5.6l12.8 12.8M6.2 21h11.6M7 3h10l-3.5 7H10.5L7 3z"/></svg>
      </button>
    </div>
    <div
      ref="editorEl"
      class="editor-body px-6 py-5 min-h-[420px] max-h-[70vh] overflow-y-auto focus:outline-none"
      contenteditable="true"
      data-placeholder="在此撰写合同正文，用右侧字段面板把字段插入到正文中…"
      @input="onInput"
      @paste="onPaste"
      @blur="onInput"
    ></div>
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref, watch } from 'vue'

const props = defineProps<{ content: string }>()
const emit = defineEmits<{ 'update:content': [value: string] }>()

const editorEl = ref<HTMLDivElement>()

// execCommand 仍是免依赖、跨浏览器可直接用的编辑命令层；合同正文所需的
// 排版能力（标题、列表、对齐、基础内联样式）都在覆盖范围内。
const commands: { cmd: string; title: string; icon: string; arg?: string }[] = [
  { cmd: 'bold', title: '加粗', icon: '<svg fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2.5" d="M7 5h6a3.5 3.5 0 010 7H7zm0 7h7a3.5 3.5 0 010 7H7z"/></svg>' },
  { cmd: 'italic', title: '斜体', icon: '<svg fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 5h4M5 19h4m3-14l-4 14m9-14l-4 14"/></svg>' },
  { cmd: 'underline', title: '下划线', icon: '<svg fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M7 4v6a5 5 0 0010 0V4M5 20h14"/></svg>' },
  { cmd: 'strikeThrough', title: '删除线', icon: '<svg fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 12h14M16 7a4 4 0 00-4-2c-2.2 0-4 1.3-4 3s1.5 2.6 4 3 4 1.3 4 3-1.8 3-4 3a4 4 0 01-4-2"/></svg>' },
  { cmd: 'insertUnorderedList', title: '无序列表', icon: '<svg fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 6h13M8 12h13M8 18h13M3.5 6h.01M3.5 12h.01M3.5 18h.01"/></svg>' },
  { cmd: 'insertOrderedList', title: '有序列表', icon: '<svg fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10 6h11M10 12h11M10 18h11M4 6h1v4M4 10h2M6 18H4v-2l2-2H4"/></svg>' },
  { cmd: 'justifyLeft', title: '左对齐', icon: '<svg fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 6h16M4 12h10M4 18h16"/></svg>' },
  { cmd: 'justifyCenter', title: '居中', icon: '<svg fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 6h16M7 12h10M4 18h16"/></svg>' },
  { cmd: 'justifyRight', title: '右对齐', icon: '<svg fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 6h16M10 12h10M4 18h16"/></svg>' },
  { cmd: 'undo', title: '撤销', icon: '<svg fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 14L4 9l5-5M4 9h10a6 6 0 010 12h-3"/></svg>' },
  { cmd: 'redo', title: '重做', icon: '<svg fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 14l5-5-5-5M20 9H10a6 6 0 000 12h3"/></svg>' },
]

onMounted(() => {
  if (editorEl.value) {
    editorEl.value.innerHTML = props.content || ''
  }
})

// 外部内容变化（如载入模板详情）时同步进编辑器；光标正在编辑内时不回写，
// 避免打字被重置。
watch(
  () => props.content,
  (val) => {
    if (editorEl.value && val !== editorEl.value.innerHTML) {
      const sel = window.getSelection()
      const editing = sel && sel.anchorNode && editorEl.value.contains(sel.anchorNode)
      if (!editing) editorEl.value.innerHTML = val || ''
    }
  },
)

function exec(cmd: string, arg?: string) {
  editorEl.value?.focus()
  document.execCommand(cmd, false, arg)
  onInput()
}

function onInput() {
  if (editorEl.value) {
    emit('update:content', editorEl.value.innerHTML)
  }
}

// 粘贴拦截：优先 text/html 直接插入（保存时服务端会做白名单清洗兜底），
// 否则降级为纯文本。
function onPaste(e: ClipboardEvent) {
  e.preventDefault()
  const html = e.clipboardData?.getData('text/html')
  if (html) {
    document.execCommand('insertHTML', false, html)
  } else {
    const text = e.clipboardData?.getData('text/plain') || ''
    document.execCommand('insertText', false, text)
  }
  onInput()
}

// 在当前光标处插入 HTML（字段占位符由父组件调用）。
function insertHTML(html: string) {
  editorEl.value?.focus()
  document.execCommand('insertHTML', false, html)
  onInput()
}

defineExpose({ insertHTML })
</script>

<style scoped>
.editor-btn {
  @apply w-8 h-8 inline-flex items-center justify-center rounded-lg text-muted-foreground hover:text-brand-600 dark:hover:text-brand-300 hover:bg-muted transition-colors;
}
.editor-btn :deep(svg) {
  @apply w-5 h-5;
}
.editor-select {
  @apply h-8 px-2 rounded-lg border border-input bg-surface/80 dark:bg-surface/10 text-xs text-foreground focus:outline-none;
}
.editor-body:empty::before {
  content: attr(data-placeholder);
  @apply text-muted-foreground;
}
.editor-body :deep(.ct-field) {
  @apply inline-block px-1.5 rounded-md bg-brand-500/10 text-brand-600 dark:text-brand-300 border border-brand-500/30 text-sm font-medium cursor-default select-none;
}
.editor-body :deep(.ct-field) * {
  @apply pointer-events-none;
}
</style>
