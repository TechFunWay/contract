import { createApp } from 'vue'
import { createPinia } from 'pinia'
import router from './router'
import App from './App.vue'
import './style.css'
import { SK } from './utils/storage-keys'

// 飞牛授权整页跳转路径的收尾：交互式授权页确认后把票据放进 hash 跳回应用页，
// 这里先接住；后续 /api/auth/fnos/login、/bind 请求由 request 拦截器附带
// X-FnOS-Ticket。这条路径只在直连端口（局域网直连、旧书签）上才会走到——
// 网关域（飞牛桌面 / 手机 App）上的正常路径是就地签票，全程零跳转。
function captureFnOSTicket() {
  const hash = window.location.hash.replace(/^#/, '')
  if (!hash) return
  const params = new URLSearchParams(hash)
  const ticket = params.get('fnos_ticket')
  if (!ticket) return
  sessionStorage.setItem(SK.fnosTicket, ticket)
  const fnosUsername = params.get('fnos_username')
  if (fnosUsername) sessionStorage.setItem(SK.fnosTicketUsername, fnosUsername)
  params.delete('fnos_ticket')
  params.delete('fnos_username')
  params.delete('fnos_gateway')
  const rest = params.toString()
  history.replaceState(null, '', window.location.pathname + window.location.search + (rest ? `#${rest}` : ''))
}

// 跳板页取票/跳转失败时把错误标记带回直连端口，登录页据此就地提示，
// 避免用户停在空白跳板页不知所措。
function captureFnOSEntryError() {
  const hash = window.location.hash.replace(/^#/, '')
  if (!hash) return
  const params = new URLSearchParams(hash)
  const error = params.get('fnos_error')
  if (!error) return
  sessionStorage.setItem(SK.fnosEntryError, error)
  params.delete('fnos_error')
  params.delete('fnos_gateway')
  const rest = params.toString()
  history.replaceState(null, '', window.location.pathname + window.location.search + (rest ? `#${rest}` : ''))
}

// 跳板页回带网关入口：从桌面图标打开时 referrer 往往为空，这是最可靠的
// 网关地址来源，登录页据此才能重新发起飞牛授权登录。
function rememberFnOSGatewayFromHash() {
  const hash = window.location.hash.replace(/^#/, '')
  if (!hash) return
  const params = new URLSearchParams(hash)
  const gateway = params.get('fnos_gateway')
  if (gateway && gateway.includes(import.meta.env.BASE_URL)) {
    localStorage.setItem(SK.fnosGatewayUrl, gateway)
  }
}

// 从飞牛桌面跳转过来时记住网关入口，登录页「使用飞牛 NAS 登录」按钮据此
// 打开跳板页。
function rememberFnOSGateway() {
  if (!document.referrer) return
  try {
    const desktop = new URL(document.referrer)
    if (desktop.protocol !== 'http:' && desktop.protocol !== 'https:') return
    if (desktop.hostname !== window.location.hostname) return
    if (desktop.origin === window.location.origin) return
    localStorage.setItem(SK.fnosGatewayUrl, `${desktop.origin}${import.meta.env.BASE_URL}fnos-entry.html`)
  } catch {}
}

captureFnOSTicket()
captureFnOSEntryError()
rememberFnOSGatewayFromHash()
rememberFnOSGateway()

const app = createApp(App)
app.use(createPinia())
app.use(router)
app.mount('#app')
