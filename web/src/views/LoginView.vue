<template>
  <AuthShell title="登录" :subtitle="fnosBindingRequired ? '登录已有应用账号并绑定当前飞牛 NAS 用户' : '输入账号信息以访问控制台'">
    <form @submit.prevent="handleLogin" class="space-y-5">
      <div v-if="fnosBindingRequired" class="rounded-2xl border border-brand-400/30 bg-brand-400/10 px-5 py-4 text-sm leading-6 text-white/85">
        当前飞牛 NAS 用户 <span class="font-semibold text-brand-200">{{ fnosUsername || '已登录用户' }}</span> 尚未绑定。
        系统中已有应用账号，请登录现有账号；验证成功后会自动完成绑定，并继续使用原账号中的数据与设置。
      </div>

      <AuthField v-model="username" label="用户名" autocomplete="username" required placeholder="请输入用户名" autofocus>
        <template #icon>
          <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M16 7a4 4 0 11-8 0 4 4 0 018 0zM12 14a7 7 0 00-7 7h14a7 7 0 00-7-7z"/></svg>
        </template>
      </AuthField>

      <AuthField v-model="password" label="密码" type="password" autocomplete="current-password" required placeholder="请输入密码">
        <template #icon>
          <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><rect x="5" y="11" width="14" height="9" rx="2" stroke-width="2"/><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 11V8a4 4 0 118 0v3"/></svg>
        </template>
        <template #labelRight>
          <router-link to="/forgot-password" class="text-xs font-medium text-brand-300 hover:text-brand-200 transition-colors">忘记密码？</router-link>
        </template>
      </AuthField>

      <transition enter-active-class="transition duration-200" enter-from-class="opacity-0 -translate-y-1" leave-active-class="transition duration-150" leave-to-class="opacity-0">
        <div v-if="errorMsg" class="flex items-center gap-2 text-sm text-red-300 bg-red-500/10 border border-red-500/25 rounded-xl px-3.5 py-2.5">
          <svg class="w-4 h-4 shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 9v2m0 4h.01M5.07 19h13.86a2 2 0 001.74-3L13.74 4a2 2 0 00-3.48 0L3.34 16a2 2 0 001.73 3z"/></svg>
          {{ errorMsg }}
        </div>
      </transition>

      <button type="submit" :disabled="loading" class="btn-premium">
        <svg v-if="loading" class="w-5 h-5 animate-spin" fill="none" viewBox="0 0 24 24"><circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"/><path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"/></svg>
        {{ loading ? (fnosBindingRequired ? '登录并绑定中...' : '登录中...') : (fnosBindingRequired ? '登录并绑定' : '登录') }}
      </button>

      <button v-if="authStore.fnosApp" type="button" :disabled="loading" @click="handleFnOSLogin" class="w-full min-h-12 inline-flex items-center justify-center rounded-xl border border-white/15 bg-transparent px-4 py-3 text-sm font-semibold text-white transition-colors hover:bg-white/[0.08] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-brand-300/70 disabled:cursor-not-allowed disabled:opacity-60">
        {{ loading ? '正在获取飞牛账号…' : '使用飞牛 NAS 登录' }}
      </button>
    </form>

    <Teleport to="body">
      <transition enter-active-class="transition duration-200" enter-from-class="opacity-0" leave-active-class="transition duration-150" leave-to-class="opacity-0">
        <div v-if="showFnOSConfirm" class="fixed inset-0 z-[100] flex items-center justify-center bg-black/65 px-4 backdrop-blur-sm" @click.self="closeFnOSConfirm">
          <section role="dialog" aria-modal="true" aria-labelledby="fnos-confirm-title" class="w-full max-w-md rounded-3xl border border-white/15 bg-[#0f1a1c] p-6 text-white shadow-2xl sm:p-8">
            <div class="mx-auto flex h-16 w-16 items-center justify-center rounded-2xl bg-brand-gradient text-2xl font-bold shadow-lg shadow-brand-500/25">
              {{ fnosConfirmUsername.slice(0, 1) || '飞' }}
            </div>
            <h2 id="fnos-confirm-title" class="mt-5 text-center text-2xl font-bold">确认使用飞牛 NAS 登录</h2>
            <p class="mt-2 text-center text-sm leading-6 text-white/65">合同管家正在请求使用下面的飞牛 NAS 账号登录</p>
            <div class="mt-6 flex items-center justify-center gap-3 rounded-2xl border border-white/10 bg-white/[0.06] px-4 py-4">
              <span class="flex h-10 w-10 items-center justify-center rounded-full bg-brand-400/25 font-semibold text-brand-100">{{ fnosConfirmUsername.slice(0, 1) || '飞' }}</span>
              <span class="font-semibold">{{ fnosConfirmUsername || '当前飞牛 NAS 用户' }}</span>
            </div>
            <button type="button" :disabled="loading" @click="confirmFnOSLogin" class="btn-premium mt-6">
              {{ loading ? '正在登录…' : '确认登录' }}
            </button>
            <button v-if="!isMobileClient" type="button" :disabled="loading" @click="switchFnOSAccount" class="mt-3 w-full rounded-xl px-4 py-3 text-sm font-semibold text-brand-200 transition-colors hover:bg-white/[0.06] hover:text-brand-100 disabled:opacity-60">
              使用其他飞牛账号
            </button>
            <p v-if="isMobileClient" class="mt-3 text-center text-xs leading-5 text-white/45">如需更换飞牛账号，请在飞牛 App 中切换后重新打开本应用。</p>
            <button type="button" :disabled="loading" @click="closeFnOSConfirm" class="mt-1 w-full rounded-xl px-4 py-2 text-sm text-white/55 transition-colors hover:text-white/80 disabled:opacity-60">取消</button>
          </section>
        </div>
      </transition>
    </Teleport>

    <template #footer v-if="authStore.allowRegister">
      还没有账号？
      <router-link to="/register" class="font-semibold text-brand-300 hover:text-brand-200 transition-colors">立即注册</router-link>
    </template>
  </AuthShell>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { bindFnOSAccount, login, fnosLogin } from '../api/auth'
import { useAuthStore } from '../stores/auth'
import {
  startFnOSAuthorize,
  startFnOSFullPageAuthorize,
  startFnOSAccountSwitch,
  tryIssueFnOSTicket,
  resolveFnOSGatewayEntry,
  fnOSMobileClient,
} from '../utils/fnos-auth'
import { SK } from '../utils/storage-keys'
import AuthShell from '../components/auth/AuthShell.vue'
import AuthField from '../components/auth/AuthField.vue'

const router = useRouter()
const authStore = useAuthStore()

const username = ref('')
const password = ref('')
const loading = ref(false)
const errorMsg = ref('')
const fnosBindingRequired = ref(false)
const fnosUsername = ref('')
const fnosConfirmUsername = ref('')
const showFnOSConfirm = ref(false)
// 手机端（含飞牛 App 网页视图）不提供「切换飞牛账号」入口：换号必须整页去飞牛
// 登录页，手机端不可靠；提示用户在飞牛 App 内切换。
const isMobileClient = fnOSMobileClient()

function clearFnOSTicket() {
  sessionStorage.removeItem(SK.fnosTicket)
  sessionStorage.removeItem(SK.fnosTicketUsername)
}

function storeFnOSTicket(ticket: string, username: string) {
  sessionStorage.setItem(SK.fnosTicket, ticket)
  if (username) sessionStorage.setItem(SK.fnosTicketUsername, username)
  else sessionStorage.removeItem(SK.fnosTicketUsername)
  sessionStorage.removeItem(SK.fnosEntryError)
}

async function handleLogin() {
  loading.value = true
  errorMsg.value = ''
  try {
    const res = fnosBindingRequired.value
      ? await bindFnOSAccount('bind', username.value, password.value)
      : await login(username.value, password.value)
    if (res.data?.code === 0) {
      if (fnosBindingRequired.value) clearFnOSTicket()
      authStore.setToken(res.data.data.token)
      authStore.setUser(res.data.data.user)
      router.push('/admin')
    } else {
      errorMsg.value = res.data?.message || '登录失败'
    }
  } catch (err: any) {
    errorMsg.value = err.response?.data?.message || '网络错误'
  } finally {
    loading.value = false
  }
}

// 飞牛授权：优先弹窗打开网关授权页（fnos-entry.html），账号展示、换账号、确认
// 全部在弹窗里完成，本页不跳转不刷新；弹窗确认后 postMessage 交回一次性票据，
// 本页拿到直接登录。弹窗不可用（被拦截 / 飞牛手机 App 网页视图假句柄）时由
// startFnOSAuthorize 回调兜底：网关域就地签票弹页内确认框，直连端口整页跳授权页。
async function handleFnOSLogin() {
  if (loading.value) return
  loading.value = true
  errorMsg.value = ''
  const gatewayURL = await resolveFnOSGatewayEntry({
    servicePort: authStore.servicePort,
    entry: authStore.fnosGatewayEntry,
  })
  loading.value = false
  if (!gatewayURL) {
    errorMsg.value = '请先从飞牛桌面打开本应用，再使用飞牛一键登录'
    return
  }
  startFnOSAuthorize(gatewayURL, {
    onTicket: (ticket, username) => {
      storeFnOSTicket(ticket, username)
      confirmFnOSLogin()
    },
    onUnavailable: () => fallbackFnOSAuthorize(gatewayURL),
  })
}

// 弹窗不可用时的兜底：网关域（飞牛桌面入口 / 手机 App 的页面本来就在网关域）
// 就地签票，拿到票据弹页内确认框——手机端网页视图里每多一次整页跳转就多一处
// 断裂点；只有直连端口（局域网直连、旧书签）就地签票必然 401，才整页跳授权页。
async function fallbackFnOSAuthorize(gatewayURL: string) {
  loading.value = true
  const issued = await tryIssueFnOSTicket()
  if (issued) {
    loading.value = false
    storeFnOSTicket(issued.ticket, issued.username)
    fnosConfirmUsername.value = issued.username
    showFnOSConfirm.value = true
    return
  }
  // 网关域也签不到票（直连端口）：整页跳授权页。跳转前探测可达性——入口可能来自
  // 另一个网络，打不开时就地提示，不把用户送到浏览器的「无法访问页面」。
  const opened = await startFnOSFullPageAuthorize(gatewayURL)
  loading.value = false
  if (!opened) {
    errorMsg.value = '当前网络无法打开飞牛授权页，请检查网络后重试，或改用用户名密码登录'
  }
}

function openFnOSConfirm() {
  fnosConfirmUsername.value = sessionStorage.getItem(SK.fnosTicketUsername) || ''
  showFnOSConfirm.value = true
}

function closeFnOSConfirm() {
  if (!loading.value) showFnOSConfirm.value = false
}

function cancelFnOSConfirm() {
  showFnOSConfirm.value = false
  clearFnOSTicket()
}

// 切换飞牛账号：弹窗打开飞牛登录页换账号，应用页全程不跳转。飞牛登录成功
// 后只接受同源回跳（redirect_uri 的 origin 与网关不一致会被直接丢弃并回落到
// 飞牛桌面），所以 redirect_uri 指回网关同源的跳板页并带 fnos_popup=1 标记；
// 跳板页取到新账号的一次性票据后经 postMessage 交回本页并自动关窗。
async function switchFnOSAccount() {
  const gatewayURL = await resolveFnOSGatewayEntry({
    servicePort: authStore.servicePort,
    entry: authStore.fnosGatewayEntry,
  })
  if (!gatewayURL) {
    showFnOSConfirm.value = false
    clearFnOSTicket()
    errorMsg.value = '无法定位飞牛桌面入口，请从飞牛桌面重新打开本应用'
    return
  }
  clearFnOSTicket()
  showFnOSConfirm.value = false
  const ok = await startFnOSAccountSwitch(gatewayURL, authStore.servicePort, {
    onTicket: (ticket, username) => {
      storeFnOSTicket(ticket, username)
      fnosConfirmUsername.value = username
      showFnOSConfirm.value = true
    },
    onCancel: () => {
      clearFnOSTicket()
    },
  })
  if (!ok) {
    errorMsg.value = isMobileClient
      ? '如需更换飞牛账号，请在飞牛 App 中切换后重新打开本应用'
      : '当前网络无法打开飞牛登录页，请检查网络后重试'
  }
}

// 弹窗里点「确认登录」才真正开始登录。票据一次性且两分钟内有效，过期或
// 失败时清掉并提示重新点击飞牛一键登录。
async function confirmFnOSLogin() {
  loading.value = true
  errorMsg.value = ''
  try {
    const res = await fnosLogin()
    if (res.data?.code !== 0) {
      clearFnOSTicket()
      showFnOSConfirm.value = false
      errorMsg.value = res.data?.message || '飞牛一键登录失败'
      return
    }
    if (res.data.data?.binding_required) {
      showFnOSConfirm.value = false
      if (res.data.data.has_accounts || res.data.data.suggested_mode === 'bind') {
        fnosBindingRequired.value = true
        fnosUsername.value = res.data.data.fnos_username || ''
        username.value = res.data.data.suggested_username || ''
        return
      }
      // 没有任何应用账号：转注册页创建并绑定，票据保留给绑定请求使用。
      router.push({
        name: 'Register',
        query: {
          fnos: 'bind',
          fnos_username: res.data.data.fnos_username || '',
          fnos_mode: 'register',
        },
      })
      return
    }
    clearFnOSTicket()
    authStore.setToken(res.data.data.token)
    authStore.setUser(res.data.data.user)
    authStore.resetInit()
    showFnOSConfirm.value = false
    router.push('/admin')
  } catch (err: any) {
    clearFnOSTicket()
    showFnOSConfirm.value = false
    errorMsg.value = err.response?.data?.message || '飞牛一键登录失败'
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  // 跳板页失败回退：就地显示跳板页带回的错误，指引从桌面入口重试。
  const entryError = sessionStorage.getItem(SK.fnosEntryError)
  if (entryError) {
    sessionStorage.removeItem(SK.fnosEntryError)
    if (!authStore.isAuthenticated) errorMsg.value = entryError
    return
  }
  // 跳板页/就地签票带回票据后的入口：先弹窗展示当前飞牛账号，用户点
  // 「确认登录」后才开始登录，绝不静默一键登录。
  if (sessionStorage.getItem(SK.fnosTicket) && !authStore.isAuthenticated) {
    openFnOSConfirm()
  }
})
</script>
