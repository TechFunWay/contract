import { createRouter, createWebHistory } from 'vue-router'
import { useAuthStore } from '../stores/auth'
import { SK } from '../utils/storage-keys'

const router = createRouter({
  history: createWebHistory(import.meta.env.BASE_URL),
  routes: [
    {
      path: '/login',
      name: 'Login',
      component: () => import('../views/LoginView.vue'),
      meta: { requiresAuth: false }
    },
    {
      path: '/register',
      name: 'Register',
      component: () => import('../views/RegisterView.vue'),
      meta: { requiresAuth: false }
    },
    {
      path: '/forgot-password',
      name: 'ForgotPassword',
      component: () => import('../views/ForgotPasswordView.vue'),
      meta: { requiresAuth: false }
    },
    {
      // 后台路由统一前缀为 /admin，需要登录。
      path: '/admin',
      component: () => import('../layouts/MainLayout.vue'),
      meta: { requiresAuth: true },
      children: [
        { path: '', name: 'Home', component: () => import('../views/HomeView.vue') },
        { path: 'contract-templates', name: 'ContractTemplates', component: () => import('../views/ContractTemplatesView.vue') },
        // new 与 :id 共用一条路由：编辑器视图以 params.id === 'new' 区分新建
        { path: 'contract-templates/:id', name: 'ContractTemplateEdit', component: () => import('../views/ContractTemplateEditorView.vue') },
        { path: 'contract-contracts', name: 'Contracts', component: () => import('../views/ContractsView.vue') },
        { path: 'contract-contracts/:id', name: 'ContractEdit', component: () => import('../views/ContractEditorView.vue') },
        { path: 'profile', name: 'Profile', component: () => import('../views/ProfileView.vue') },
        { path: 'settings', name: 'Settings', component: () => import('../views/SettingsView.vue') },
        { path: 'users', name: 'AdminUsers', component: () => import('../views/AdminUsersView.vue'), meta: { requiresAdmin: true } },
        { path: 'configs', name: 'AdminConfigs', component: () => import('../views/AdminConfigView.vue'), meta: { requiresAdmin: true } },
        { path: 'backups', name: 'AdminBackups', component: () => import('../views/AdminBackupView.vue'), meta: { requiresAdmin: true } },
        { path: 'audit', name: 'AdminAudit', component: () => import('../views/AdminAuditView.vue'), meta: { requiresAdmin: true } },
      ]
    },
    {
      // 打印页独立于后台布局：A4 版式 + 浏览器打印导出 PDF。
      path: '/print/contract/:id',
      name: 'ContractPrint',
      component: () => import('../views/ContractPrintView.vue'),
      meta: { requiresAuth: true }
    },
    {
      // 后期 / 用于免登录的门户或前端页面，当前先重定向到后台首页。
      path: '/',
      redirect: '/admin',
      meta: { requiresAuth: false }
    }
  ]
})

router.beforeEach(async (to, from, next) => {
  const authStore = useAuthStore()
  await authStore.init()

  const fnosEnabled = import.meta.env.VITE_FNOS_APP === 'true'
  if (authStore.setupRequired && to.name !== 'Register') {
    // 只有在跳板页带回票据时才直接进入「创建管理员并绑定 NAS」流程；没有票据
    // 时给出普通注册页，页面上另有「使用飞牛 NAS 授权登录」入口去取票。
    const pendingTicket = !!sessionStorage.getItem(SK.fnosTicket)
    next({
      name: 'Register',
      query: fnosEnabled && pendingTicket ? { fnos: 'bind', fnos_mode: 'register' } : {},
    })
    return
  }

  if (to.meta.requiresAuth !== false && !authStore.isAuthenticated && authStore.requireLogin) {
    next({ name: 'Login' })
  } else if (to.meta.requiresAdmin && !authStore.isAdmin) {
    next({ name: 'Home' })
  } else {
    next()
  }
})

export default router
