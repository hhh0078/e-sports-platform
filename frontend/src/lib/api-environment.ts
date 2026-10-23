/** 文件作用：统一开发与正式构建的 API 地址，并阻止正式包连接本机服务。 */
export const PRODUCTION_API_BASE = 'https://game.ai.58it.cn/api'
export const DEVELOPMENT_API_BASE = 'http://127.0.0.1:8090/api'

/**
 * 按运行环境解析 API，正式环境仅允许 HTTPS 域名与 /api 路径。
 * @param production - 是否为正式构建。
 * @param configured - 显式配置的 API 地址，未提供时使用对应环境默认值。
 * @returns 去除末尾斜杠的完整地址；非法配置会抛出错误。
 */
export function resolveAPIBase(production: boolean, configured?: string): string {
  const value = (configured || (production ? PRODUCTION_API_BASE : DEVELOPMENT_API_BASE)).replace(/\/$/, '')
  const url = new URL(value)
  if (!['http:', 'https:'].includes(url.protocol) || url.username || url.password || url.search || url.hash || url.pathname !== '/api') throw new Error('API 地址必须是完整的 HTTP(S) 域名与 /api 路径')
  if (production && (url.protocol !== 'https:' || url.hostname === 'localhost' || url.hostname.endsWith('.localhost') || url.hostname === '[::1]' || /^\d+\.\d+\.\d+\.\d+$/.test(url.hostname))) throw new Error('正式构建必须使用公网 HTTPS API 域名，不能使用本机或 IP 地址')
  return value
}
