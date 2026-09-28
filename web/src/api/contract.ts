import request from './request'

// 合同管家 API client：模板与合同。字段定义在模板/合同详情里是结构化数组，
// 列表接口不携带正文大字段。

export type FieldType = 'text' | 'textarea' | 'number' | 'date' | 'select'

export interface TemplateField {
  key: string
  label: string
  type: FieldType
  required: boolean
  placeholder?: string
  options?: string[]
  width?: 'full' | 'half'
}

export interface TemplateSummary {
  id: number
  name: string
  category: string
  description: string
  field_count: number
  contract_count: number
  created_at: string
  updated_at: string
}

export interface TemplateDetail extends TemplateSummary {
  content: string
  fields: TemplateField[]
}

export type ContractStatus = 'draft' | 'active' | 'archived'

export interface ContractSummary {
  id: number
  template_id: number
  template_name: string
  title: string
  status: ContractStatus
  remark: string
  sign_date: string | null
  created_at: string
  updated_at: string
}

export interface ContractDetail extends ContractSummary {
  content: string
  fields: TemplateField[]
  values: Record<string, string>
  rendered: string
}

export interface PageData<T> {
  items: T[]
  total: number
  page: number
  pageSize: number
}

export interface TemplatePayload {
  name: string
  category?: string
  description?: string
  content?: string
  fields?: TemplateField[]
}

// ---------- 模板 ----------

export function getTemplates(params: { page?: number; pageSize?: number; keyword?: string; category?: string } = {}) {
  return request.get('/api/contract/templates', { params })
}

export function getTemplate(id: number | string) {
  return request.get(`/api/contract/templates/${id}`)
}

export function createTemplate(payload: TemplatePayload) {
  return request.post('/api/contract/templates', payload)
}

export function updateTemplate(id: number | string, payload: TemplatePayload) {
  return request.put(`/api/contract/templates/${id}`, payload)
}

export function deleteTemplate(id: number) {
  return request.delete(`/api/contract/templates/${id}`)
}

export function duplicateTemplate(id: number) {
  return request.post(`/api/contract/templates/${id}/duplicate`)
}

// ---------- 合同 ----------

export function getContracts(params: { page?: number; pageSize?: number; keyword?: string; status?: string; template_id?: number } = {}) {
  return request.get('/api/contract/contracts', { params })
}

export function getContract(id: number | string) {
  return request.get(`/api/contract/contracts/${id}`)
}

export function createContract(payload: { template_id: number; title?: string; values?: Record<string, string> }) {
  return request.post('/api/contract/contracts', payload)
}

export function updateContract(id: number | string, payload: Partial<{
  title: string
  values: Record<string, string>
  status: ContractStatus
  remark: string
  sign_date: string
}>) {
  return request.put(`/api/contract/contracts/${id}`, payload)
}

export function deleteContract(id: number) {
  return request.delete(`/api/contract/contracts/${id}`)
}

export function duplicateContract(id: number) {
  return request.post(`/api/contract/contracts/${id}/duplicate`)
}

export const CONTRACT_STATUS_LABELS: Record<ContractStatus, string> = {
  draft: '草稿',
  active: '生效',
  archived: '归档',
}

export const FIELD_TYPE_LABELS: Record<FieldType, string> = {
  text: '单行文本',
  textarea: '多行文本',
  number: '数字',
  date: '日期',
  select: '下拉选择',
}
