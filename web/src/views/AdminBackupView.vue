<template>
  <div class="page-container animate-fade-in">
    <!-- 自动备份说明 -->
    <div class="surface rounded-2xl p-5 sm:p-6">
      <div class="flex flex-wrap items-start justify-between gap-4">
        <div class="flex items-start gap-3">
          <div class="w-10 h-10 rounded-xl bg-brand-500/10 text-brand-500 dark:text-brand-300 flex items-center justify-center shrink-0">
            <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 8h14M5 8a2 2 0 110-4h14a2 2 0 110 4M5 8v10a2 2 0 002 2h10a2 2 0 002-2V8m-9 4h4"/></svg>
          </div>
          <div>
            <h3 class="text-sm font-bold text-foreground">数据库备份</h3>
            <p class="text-sm text-muted-foreground mt-1 leading-relaxed">
              备份为 SQLite 一致性快照，存放于服务器数据目录
              <code class="px-1.5 py-0.5 rounded bg-muted text-xs">{{ backupDir || 'backups/' }}</code> 下。
              自动备份{{ intervalDesc }}，保留最近 {{ keepCount }} 份；
              可在 <RouterLink to="/admin/configs" class="text-brand-500 dark:text-brand-300 hover:underline">系统配置</RouterLink> 中调整开关与数量。
              每份备份都可在下方列表一键「恢复」，恢复前系统会自动留存当前数据的快照。
              也支持从本地上传备份文件（如换机迁移时下载过的 .db），上传后同样在列表点击「恢复」。
            </p>
            <div class="flex items-center gap-2 mt-3">
              <span class="text-sm text-muted-foreground shrink-0">备份频率</span>
              <select
                v-model="intervalHours"
                :disabled="savingInterval"
                @change="saveInterval"
                aria-label="备份频率"
                class="input-field !w-auto !py-1.5 text-sm cursor-pointer disabled:opacity-60"
              >
                <option v-for="opt in INTERVAL_OPTIONS" :key="opt.value" :value="opt.value">{{ opt.label }}</option>
              </select>
            </div>
          </div>
        </div>
        <div class="flex items-center gap-2">
          <span class="badge" :class="autoEnabled ? 'bg-emerald-100 text-emerald-700 dark:bg-emerald-500/15 dark:text-emerald-300' : 'bg-rose-100 text-rose-700 dark:bg-rose-500/15 dark:text-rose-300'">
            自动备份{{ autoEnabled ? '已开启' : '已关闭' }}
          </span>
          <button class="btn-brand !px-4 !py-2" :disabled="creating" @click="handleCreate">
            <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4v16m8-8H4"/></svg>
            {{ creating ? '备份中…' : '立即备份' }}
          </button>
          <button
            class="btn-ghost !px-4 !py-2"
            :disabled="uploading"
            @click="fileInput?.click()"
            title="从本地上传备份文件（.db），上传后在列表点击「恢复」"
          >
            <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 16v2a2 2 0 002 2h12a2 2 0 002-2v-2M12 4v12m0-12l-4 4m4-4l4 4"/></svg>
            {{ uploading ? '上传中…' : '上传恢复' }}
          </button>
          <input
            ref="fileInput"
            type="file"
            accept=".db,application/octet-stream"
            class="hidden"
            @change="handleUploadFile"
          >
        </div>
      </div>
    </div>

    <!-- 备份列表 -->
    <div class="surface rounded-2xl overflow-hidden">
      <div class="flex flex-wrap items-center justify-between gap-3 p-5 sm:p-6 border-b border-border">
        <div>
          <h2 class="text-lg font-bold text-foreground">备份文件</h2>
          <p class="text-sm text-muted-foreground">共 {{ items.length }} 份 · 新的在前</p>
        </div>
        <button @click="load" class="btn-ghost !px-4 !py-2">
          <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15"/></svg>
          刷新
        </button>
      </div>

      <!-- 表格只在 md 及以上显示，手机端用卡片列表 -->
      <div class="hidden md:block overflow-x-auto">
        <table class="w-full text-sm">
          <thead>
            <tr class="text-left text-xs uppercase tracking-wider text-muted-foreground bg-muted/60">
              <th class="py-3.5 px-6 font-semibold">文件名</th>
              <th class="py-3.5 px-4 font-semibold">大小</th>
              <th class="py-3.5 px-4 font-semibold">创建时间</th>
              <th class="py-3.5 px-6 font-semibold text-right">操作</th>
            </tr>
          </thead>
          <tbody class="divide-y divide-border">
            <tr v-for="item in items" :key="item.name" class="hover:bg-muted/50 transition-colors">
              <td class="py-3.5 px-6 font-medium text-foreground">
                <div class="flex items-center gap-2.5">
                  <svg class="w-4 h-4 text-brand-500 dark:text-brand-300 shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 7v8a4 4 0 008 0V7m-12 0h16M5 3h14a2 2 0 012 2v14a2 2 0 01-2 2H5a2 2 0 01-2-2V5a2 2 0 012-2z"/></svg>
                  <code class="text-xs">{{ item.name }}</code>
                </div>
              </td>
              <td class="py-3.5 px-4 text-muted-foreground whitespace-nowrap">{{ formatSize(item.size) }}</td>
              <td class="py-3.5 px-4 text-muted-foreground whitespace-nowrap">{{ formatTime(item.created_at) }}</td>
              <td class="py-3.5 px-6 text-right whitespace-nowrap">
                <button @click="handleDownload(item)" class="text-sm font-medium text-brand-600 dark:text-brand-300 hover:underline mr-4">下载</button>
                <button
                  @click="askRestore(item)"
                  :disabled="restoring"
                  class="text-sm font-medium text-amber-600 dark:text-amber-300 hover:underline mr-4 disabled:opacity-50 disabled:no-underline"
                >恢复</button>
                <button @click="askDelete(item)" class="text-sm font-medium text-destructive hover:underline">删除</button>
              </td>
            </tr>
            <tr v-if="items.length === 0">
              <td colspan="4" class="py-16 text-center">
                <div class="flex flex-col items-center gap-3 text-muted-foreground">
                  <svg class="w-12 h-12 opacity-40" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M5 8h14M5 8a2 2 0 110-4h14a2 2 0 110 4M5 8v10a2 2 0 002 2h10a2 2 0 002-2V8m-9 4h4"/></svg>
                  <span class="text-sm">还没有备份，点击右上角「立即备份」创建第一份</span>
                </div>
              </td>
            </tr>
          </tbody>
        </table>
      </div>

      <!-- 手机端备份卡片列表 -->
      <div class="md:hidden divide-y divide-border">
        <div v-for="item in items" :key="item.name" class="px-4 py-3">
          <div class="flex items-center gap-2 min-w-0">
            <svg class="w-4 h-4 text-brand-500 dark:text-brand-300 shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 7v8a4 4 0 008 0V7m-12 0h16M5 3h14a2 2 0 012 2v14a2 2 0 01-2 2H5a2 2 0 01-2-2V5a2 2 0 012-2z"/></svg>
            <code class="text-xs text-foreground truncate flex-1">{{ item.name }}</code>
            <span class="shrink-0 text-xs text-muted-foreground whitespace-nowrap">{{ formatSize(item.size) }} · {{ formatTime(item.created_at) }}</span>
          </div>
          <div class="flex items-center justify-end gap-4 mt-1.5">
            <button @click="handleDownload(item)" class="text-sm font-medium text-brand-600 dark:text-brand-300 hover:underline">下载</button>
            <button
              @click="askRestore(item)"
              :disabled="restoring"
              class="text-sm font-medium text-amber-600 dark:text-amber-300 hover:underline disabled:opacity-50 disabled:no-underline"
            >恢复</button>
            <button @click="askDelete(item)" class="text-sm font-medium text-destructive hover:underline">删除</button>
          </div>
        </div>
        <div v-if="items.length === 0" class="py-12 text-center text-sm text-muted-foreground">还没有备份，点击右上角「立即备份」创建第一份</div>
      </div>
    </div>

    <Toast :message="toastMsg" :type="toastType" />

    <ConfirmDialog
      v-model="confirmVisible"
      :title="confirmTitle"
      :message="confirmMessage"
      :confirm-text="confirmText"
      :confirm-type="confirmType"
      @confirm="onConfirm"
      @cancel="onCancel"
    />
  </div>
</template>

<script setup lang="ts">
import { ref, computed, nextTick, onMounted } from 'vue'
import { RouterLink } from 'vue-router'
import Toast from '../components/Toast.vue'
import ConfirmDialog from '../components/ConfirmDialog.vue'
import { getSystemConfigs, updateConfig } from '../api/config'
import {
  listBackups,
  createBackup,
  deleteBackup,
  downloadBackup,
  restoreBackup,
  uploadBackup,
  type BackupItem,
} from '../api/system'

const items = ref<BackupItem[]>([])
const backupDir = ref('')
const autoEnabled = ref(false)
const keepCount = ref(7)
const creating = ref(false)
const restoring = ref(false)
const uploading = ref(false)
const fileInput = ref<HTMLInputElement | null>(null)

// 备份频率：与后端 sysconfig 注册的 backup_interval_hours 选项一一对应
const INTERVAL_KEY = 'backup_interval_hours'
const INTERVAL_OPTIONS: { value: string; label: string; desc: string }[] = [
  { value: '0', label: '每天 03:00', desc: '每天 03:00 执行' },
  { value: '1', label: '每 1 小时', desc: '每 1 小时执行一次' },
  { value: '3', label: '每 3 小时', desc: '每 3 小时执行一次' },
  { value: '6', label: '每 6 小时', desc: '每 6 小时执行一次' },
  { value: '12', label: '每 12 小时', desc: '每 12 小时执行一次' },
  { value: '24', label: '每 24 小时', desc: '每 24 小时执行一次' },
  { value: '72', label: '每 3 天', desc: '每 3 天执行一次' },
  { value: '168', label: '每 7 天', desc: '每 7 天执行一次' },
]
const intervalHours = ref('0')
const savingInterval = ref(false)
const intervalDesc = computed(
  () => INTERVAL_OPTIONS.find((o) => o.value === intervalHours.value)?.desc ?? INTERVAL_OPTIONS[0].desc,
)

const toastMsg = ref('')
const toastType = ref<'success' | 'error'>('success')
async function toast(message: string, type: 'success' | 'error' = 'success') {
  toastType.value = type
  toastMsg.value = ''
  await nextTick()
  toastMsg.value = message
}

onMounted(() => {
  load()
  loadInterval()
})

async function load() {
  try {
    const res = await listBackups()
    if (res.data?.code === 0 && res.data.data) {
      items.value = res.data.data.items || []
      backupDir.value = res.data.data.dir || ''
      autoEnabled.value = !!res.data.data.auto_enabled
      keepCount.value = res.data.data.keep_count || 7
    }
  } catch {}
}

async function loadInterval() {
  try {
    const res = await getSystemConfigs()
    const v = res.data?.data?.[INTERVAL_KEY]
    if (v && INTERVAL_OPTIONS.some((o) => o.value === v)) intervalHours.value = v
  } catch {}
}

async function saveInterval() {
  savingInterval.value = true
  try {
    const res = await updateConfig(INTERVAL_KEY, intervalHours.value)
    if (res.data?.code === 0) {
      await toast(`备份频率已改为「${intervalDesc.value}」`)
    } else {
      await toast(res.data?.message || '保存失败', 'error')
    }
  } catch (e: any) {
    await toast(e?.response?.data?.message || '保存失败', 'error')
  } finally {
    savingInterval.value = false
  }
}

async function handleCreate() {
  creating.value = true
  try {
    const res = await createBackup()
    if (res.data?.code === 0) {
      await toast('备份创建成功', 'success')
      await load()
    } else {
      await toast(res.data?.message || '备份失败', 'error')
    }
  } catch (e: any) {
    await toast(e?.response?.data?.message || '备份失败', 'error')
  } finally {
    creating.value = false
  }
}

// 上传本地备份文件：服务端校验（SQLite 头 + 完整性 + users 表）通过后存为
// 一份备份；真正恢复仍走列表行的「恢复」，确认弹窗与恢复前快照一处不少。
async function handleUploadFile(e: Event) {
  const input = e.target as HTMLInputElement
  const file = input.files?.[0]
  input.value = ''
  if (!file) return
  if (!file.name.toLowerCase().endsWith('.db')) {
    await toast('请选择 .db 备份文件', 'error')
    return
  }
  uploading.value = true
  try {
    const res = await uploadBackup(file)
    if (res.data?.code === 0) {
      await toast(`上传成功（已存为 ${res.data.data?.name}），点击该行的「恢复」完成恢复`, 'success')
      await load()
    } else {
      await toast(res.data?.message || '上传失败', 'error')
    }
  } catch (e: any) {
    await toast(e?.response?.data?.message || '上传失败', 'error')
  } finally {
    uploading.value = false
  }
}

async function handleDownload(item: BackupItem) {
  try {
    const res = await downloadBackup(item.name)
    const url = window.URL.createObjectURL(new Blob([res.data]))
    const a = document.createElement('a')
    a.href = url
    a.download = item.name
    a.click()
    window.URL.revokeObjectURL(url)
  } catch {
    await toast('下载失败，请重试', 'error')
  }
}

// 危险操作统一走美化确认框（删除/恢复），不使用浏览器原生弹窗。
type PendingAction = { type: 'delete' | 'restore'; item: BackupItem }

const confirmVisible = ref(false)
const confirmTitle = ref('确认')
const confirmMessage = ref('')
const confirmText = ref('确认')
const confirmType = ref<'primary' | 'danger'>('primary')
const pending = ref<PendingAction | null>(null)

function openConfirm(action: PendingAction, title: string, message: string, text: string) {
  pending.value = action
  confirmTitle.value = title
  confirmMessage.value = message
  confirmText.value = text
  confirmType.value = 'danger'
  confirmVisible.value = true
}

function askDelete(item: BackupItem) {
  openConfirm({ type: 'delete', item }, '删除备份', `确定删除备份 ${item.name} 吗？此操作不可恢复。`, '删除')
}

function askRestore(item: BackupItem) {
  openConfirm(
    { type: 'restore', item },
    '恢复备份',
    `确定用 ${item.name} 恢复数据库吗？当前数据会被覆盖（系统会先自动保存一份恢复前快照，可再次用于回退），恢复完成后页面将自动刷新。`,
    '恢复',
  )
}

function onConfirm() {
  const action = pending.value
  pending.value = null
  if (!action) return
  if (action.type === 'delete') handleDelete(action.item)
  else handleRestore(action.item)
}

function onCancel() {
  pending.value = null
}

async function handleDelete(item: BackupItem) {
  try {
    const res = await deleteBackup(item.name)
    if (res.data?.code === 0) {
      await toast('备份已删除', 'success')
      await load()
    } else {
      await toast(res.data?.message || '删除失败', 'error')
    }
  } catch (e: any) {
    await toast(e?.response?.data?.message || '删除失败', 'error')
  }
}

async function handleRestore(item: BackupItem) {
  restoring.value = true
  try {
    const res = await restoreBackup(item.name)
    if (res.data?.code === 0) {
      const pre = res.data.data?.pre_backup
      await toast(pre ? `恢复成功，恢复前快照：${pre}` : '恢复成功', 'success')
      // 恢复改的是服务端整库，页面上的数据都可能变了，重新加载。
      setTimeout(() => window.location.reload(), 1500)
    } else {
      await toast(res.data?.message || '恢复失败', 'error')
    }
  } catch (e: any) {
    await toast(e?.response?.data?.message || '恢复失败', 'error')
  } finally {
    restoring.value = false
  }
}

function formatSize(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`
  return `${(bytes / 1024 / 1024).toFixed(2)} MB`
}

function formatTime(value: string): string {
  const d = new Date(value)
  return isNaN(d.getTime()) ? value : d.toLocaleString('zh-CN', { hour12: false })
}
</script>
