import request from './request'

// 系统级通用接口：版本检查、赞赏支持、数据库备份管理。

export interface UpdateInfo {
  current: string
  latest: string
  download_url: string
  has_update: boolean
}

export function checkUpdate() {
  return request.get<{ code: number; message: string; data: UpdateInfo }>('/api/version/check')
}

// amount 赞赏金额（元），随 donate_support 事件上报到统计端点，
// 由接收端记录到 app_support_log.amount；缺省 0 表示未填写。
export function donateSupport(amount?: number) {
  return request.post<{ code: number; message: string; data: { ok: boolean } }>('/api/donate/support', {
    amount: amount && amount > 0 ? amount : 0,
  })
}

export interface BackupItem {
  name: string
  size: number
  created_at: string
}

export interface BackupList {
  items: BackupItem[]
  dir: string
  auto_enabled: boolean
  keep_count: number
}

export function listBackups() {
  return request.get<{ code: number; message: string; data: BackupList }>('/api/backups')
}

export function createBackup() {
  return request.post<{ code: number; message: string; data: { name: string } }>('/api/backups')
}

export function deleteBackup(name: string) {
  return request.delete<{ code: number; message: string; data: { ok: boolean } }>(`/api/backups/${name}`)
}

// 恢复备份：用该备份覆盖当前数据库。服务端先自动留存一份恢复前快照
//（pre_backup），整体在一个事务内完成，失败不变更当前数据。
export function restoreBackup(name: string) {
  return request.post<{
    code: number
    message: string
    data: { pre_backup: string; tables: number; rows: number }
  }>(`/api/backups/${name}/restore`)
}

// 从本地上传备份文件（.db）：服务端校验通过后存为一份备份（重新命名，
// 返回 name），之后在列表上点「恢复」走与常规恢复完全相同的流程。
export function uploadBackup(file: File) {
  const form = new FormData()
  form.append('file', file)
  return request.post<{ code: number; message: string; data: { name: string } }>(
    '/api/backups/upload',
    form,
    { headers: { 'Content-Type': 'multipart/form-data' }, timeout: 120000 }
  )
}

// 下载备份需要 Authorization 头，所以像审计 CSV 导出一样取回 blob，
// 由调用方用 object URL 触发浏览器保存。
export function downloadBackup(name: string) {
  return request.get<Blob>(`/api/backups/${name}/download`, { responseType: 'blob' })
}
