<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRoute } from 'vue-router'
import { getContract, type ContractDetail } from '../api/contract'

// 独立打印页：A4 版式只呈现合同正文（服务端渲染快照 rendered），打印或
// "另存为 PDF" 由浏览器打印完成（@media print 隐藏界面元素）。
const route = useRoute()

const contract = ref<ContractDetail | null>(null)
const errorMsg = ref('')
const loading = ref(true)

onMounted(async () => {
  try {
    const res = await getContract(route.params.id as string)
    if (res.data?.code === 0) {
      contract.value = res.data.data
    } else {
      errorMsg.value = res.data?.message || '合同不存在'
    }
  } catch (err: any) {
    errorMsg.value = err.response?.data?.message || '合同不存在'
  } finally {
    loading.value = false
  }
})

function print() {
  window.print()
}

// 返回应用：打印页通常从新标签打开，能关就关。
function goBack() {
  if (window.history.length > 1) {
    window.history.back()
  } else {
    window.close()
  }
}
</script>

<template>
  <div class="print-page min-h-screen bg-muted/40 dark:bg-black/40">
    <div class="print-toolbar no-print sticky top-0 z-10 bg-surface/95 dark:bg-surface/80 backdrop-blur border-b border-border px-4 py-2.5 flex items-center gap-3">
      <button class="btn-ghost !px-3 !py-1.5 text-sm" @click="goBack">返回</button>
      <span class="text-sm text-muted-foreground truncate flex-1 text-center">{{ contract?.title }}</span>
      <button class="btn-brand !px-4 !py-1.5 text-sm" @click="print">打印 / 导出 PDF</button>
    </div>

    <div v-if="loading" class="py-20 text-center text-sm text-muted-foreground">加载中…</div>
    <div v-else-if="errorMsg" class="py-20 text-center text-sm text-red-500">{{ errorMsg }}</div>

    <div v-else-if="contract" class="paper mx-auto my-6">
      <div class="contract-body" v-html="contract.rendered"></div>
    </div>
  </div>
</template>

<style scoped>
.paper {
  width: 210mm;
  max-width: calc(100vw - 32px);
  padding: 20mm 18mm;
  background: #ffffff;
  color: #1a1a1a;
  box-shadow: 0 4px 24px rgba(0, 0, 0, 0.12);
  border-radius: 4px;
  box-sizing: border-box;
}

/* 手机上收缩纸面留白，且不强制 A4 高度（避免大片空白） */
@media (max-width: 640px) {
  .paper {
    padding: 20px 16px;
  }
}

@media (min-width: 768px) {
  .paper {
    min-height: 297mm;
  }
}

.contract-body {
  font-size: 12pt;
  line-height: 1.9;
  color: #1a1a1a;
  word-break: break-word;
}
.contract-body :deep(h1) {
  font-size: 18pt;
  font-weight: 700;
  text-align: center;
  margin: 0 0 1em;
}
.contract-body :deep(h2) {
  font-size: 15pt;
  font-weight: 700;
  margin: 1em 0 0.5em;
}
.contract-body :deep(h3) {
  font-size: 13pt;
  font-weight: 700;
  margin: 1em 0 0.4em;
}
.contract-body :deep(p) {
  margin: 0.5em 0;
  text-align: justify;
}
.contract-body :deep(table) {
  width: 100%;
  border-collapse: collapse;
  margin: 1em 0;
}
.contract-body :deep(td),
.contract-body :deep(th) {
  border: 1px solid #999;
  padding: 6px 10px;
}
.contract-body :deep(a) {
  color: inherit;
}
/* 值与空位 */
.contract-body :deep(.ct-value) {
  color: #1a1a1a;
}
.contract-body :deep(.ct-blank) {
  display: inline-block;
  min-width: 8em;
  border-bottom: 1px solid #333;
  height: 1em;
  /* 比基线再低一截，横线落到行的底部，横线上方留出手写空间 */
  vertical-align: -0.3em;
}

@page {
  size: A4;
  margin: 0;
}

@media print {
  .no-print {
    display: none !important;
  }
  .print-page {
    background: #fff !important;
  }
  .paper {
    box-shadow: none;
    border-radius: 0;
    margin: 0;
    max-width: none;
    width: auto;
  }
  body {
    background: #fff !important;
  }
}
</style>
