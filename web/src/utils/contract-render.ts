// 合同正文的占位符与渲染工具。占位符结构是与后端（server/contract/render.go、
// sanitize.go）约定的协议，改一处必须三处同步：
//
//   <span class="ct-field" data-field="key" contenteditable="false">【标签】</span>
//
// 渲染规则与后端一致：值非空替换为 ct-value 文本；留空或未知字段渲染为
// ct-blank 空位（打印样式显示下划线，供手工补填）。
import type { TemplateField } from '../api/contract'

export function fieldPlaceholderHTML(field: { key: string; label: string }): string {
  return `<span class="ct-field" data-field="${escapeAttr(field.key)}" contenteditable="false">【${escapeHTML(field.label)}】</span>`
}

// renderContent 本地近似渲染（预览用）；保存后以服务端渲染的快照为准。
export function renderContent(content: string, fields: TemplateField[], values: Record<string, string>): string {
  if (!content) return ''
  const wrapper = document.createElement('div')
  wrapper.innerHTML = content
  const known = new Set(fields.map((f) => f.key))

  const walker = document.createTreeWalker(wrapper, NodeFilter.SHOW_ELEMENT)
  const spans: HTMLElement[] = []
  while (walker.nextNode()) {
    const node = walker.currentNode as HTMLElement
    if (node.tagName === 'SPAN' && node.dataset.field) {
      spans.push(node)
    }
  }
  for (const span of spans) {
    const key = span.dataset.field || ''
    const value = (values[key] || '').trim()
    const out = document.createElement('span')
    if (known.has(key) && value) {
      out.className = 'ct-value'
      out.textContent = value
    } else {
      out.className = 'ct-blank'
    }
    span.replaceWith(out)
  }
  return wrapper.innerHTML
}

function escapeHTML(s: string): string {
  return s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;')
}

function escapeAttr(s: string): string {
  return escapeHTML(s).replace(/"/g, '&quot;')
}
