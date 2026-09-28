import request from './request'

// 退出登录走 optionalAuth：幂等，重复调用也返回成功。网关域上这一步会把
// 「这位用户主动登出了」落到服务端（见 stores/auth.ts 的 endSession）——
// 只清前端的话，服务端下一个请求就把登录态认回来了，用户「退不出去」。
export function logout() {
  return request.post('/api/auth/logout')
}

export function login(username: string, password: string) {
  return request.post('/api/auth/login', {
    username,
    password,
  })
}

// 就地签发一次性飞牛登录票据：应用跑在网关域（飞牛手机 App / 桌面图标 /
// 远程入口）时成功即可就地完成授权，无需跳板页。直连端口上该请求必然 401，
// 属预期分支，要跳过全局的「401 即登出」拦截。
export function fnosTicket() {
  return request.post('/api/auth/fnos/ticket', null, {
    headers: { 'X-Skip-Auth-Redirect': '1' },
  })
}

export function fnosLogin() {
  return request.post('/api/auth/fnos/login')
}

export function bindFnOSAccount(mode: 'register' | 'bind', username: string, password: string) {
  return request.post('/api/auth/fnos/bind', { mode, username, password })
}

export function register(username: string, password: string) {
  return request.post('/api/auth/register', {
    username,
    password,
  })
}

export function checkAuth() {
  return request.get('/api/auth/check')
}

export function getCurrentUser() {
  return request.get('/api/auth/me')
}

export function checkSetupRequired() {
  return request.get('/api/auth/setup-required')
}

export function changePassword(oldPassword: string, newPassword: string) {
  return request.put('/api/auth/password', {
    old_password: oldPassword,
    new_password: newPassword,
  })
}

export function regenerateAPIKey() {
  return request.post('/api/auth/apikey')
}
