// 应用内部地址的唯一拼接入口：所有「开新窗口 / 整页跳转到应用内页面」的
// 场景都必须经过这里，绝不要直接写 `/xxx` 这种从站点根开始的绝对路径。
//
// 为什么关键（2026-10-11 打印页踩坑）：飞牛网关模式下前端构建带
// VITE_BASE_PATH=/app/techfunway-contract/，应用的全部页面都住在网关前缀
// 之下；而 `window.open('/print/contract/1')` 这种根绝对路径会把前缀丢掉，
// 浏览器直接去敲网关根路径 `/print/contract/1`。网关认不出这个路径，把它
// 兜底转给了飞牛相册（地址是 `/p`），现场表现就是「点打印报错、后退回了
// 相册」。router.push 走 vue-router 的 base 是安全的，window.open /
// location.href 这类原生跳转没有 base 概念，必须自己带上前缀。
export function appHref(path: string): string {
  // Vite 保证 BASE_URL 以 `/` 结尾：普通部署是 `/`，飞牛网关部署是
  // `/app/techfunway-contract/`。
  const base = import.meta.env.BASE_URL
  const rel = path.replace(/^\/+/, '')
  return `${base}${rel}`
}
