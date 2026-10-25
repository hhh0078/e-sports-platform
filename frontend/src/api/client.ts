/** 文件作用：唯一 HTTP/上传传输层，统一超时、令牌、业务解包及可读错误。 */
import { queueModerationNotice } from '../lib/moderation-notice'
import { beginFinancialActivity, isFinancialRequest } from '../lib/update-manager'
// 地址在 Vite 配置阶段校验并注入，正式包不携带开发地址回退分支。
export const API_BASE: string = import.meta.env?.VITE_API_BASE || 'http://127.0.0.1:8090/api'
/** 支持的请求方法。 */
export type HttpMethod = 'GET' | 'POST' | 'PATCH' | 'DELETE' | 'PUT'
/** 统一业务错误，保留 HTTP 状态并允许识别取消操作。 */
export class ApiError extends Error { status?: number; cancelled?: boolean; code?: string }
/** 取得当前持久化会话令牌。 */
export function token(): string { return String(uni.getStorageSync('jijie_token') || '') }
/** 安全读取未知错误的状态。 */
export function errorStatus(error: unknown): number | undefined { return typeof error === 'object' && error !== null && 'status' in error && typeof error.status === 'number' ? error.status : undefined }
/** 读取稳定业务错误码，避免用中文原因或HTTP状态猜测审核结果。 */
export function errorCode(error: unknown): string | undefined { return typeof error === 'object' && error !== null && 'code' in error && typeof error.code === 'string' ? error.code : undefined }
/** 把未知异常转为可显示文案。 */
export function errorMessage(error: unknown): string { if (typeof error === 'string') return error; if (typeof error === 'object' && error !== null) { if ('message' in error && typeof error.message === 'string') return error.message; if ('errMsg' in error && typeof error.errMsg === 'string') return error.errMsg } return '操作失败，请重试' }
/** 判断是否是用户主动取消图片选择。 */
export function isCancelled(error: unknown): boolean { return typeof error === 'object' && error !== null && 'cancelled' in error && error.cancelled === true }
/** 业务响应边界：必须为对象并且包含 data 或可读 error。 */
function envelope(value: unknown): { data?: unknown; error?: string; code?: string; moderation?: unknown } {
  if (typeof value !== 'object' || value === null) throw new ApiError('服务器返回格式异常')
  const body = value as Record<string, unknown>
  if (!Object.prototype.hasOwnProperty.call(body, 'data') && typeof body.error !== 'string') throw new ApiError('服务器返回格式异常')
  return { data: body.data, error: typeof body.error === 'string' ? body.error : undefined, code: typeof body.code === 'string' ? body.code : undefined, moderation: body.moderation }
}
/** 共享传输实现，公开读取不携带本机持久令牌，避免朋友圈单页混入私有关系。 */
function sendRequest<T>(path: string, method: HttpMethod, data: unknown, authenticated: boolean): Promise<T> {
  return new Promise((resolve, reject) => {
    const authToken = authenticated ? token() : ''
    const finishFinancial=isFinancialRequest(path,method)?beginFinancialActivity():()=>{}
    // uni-app 的类型枚举遗漏了微信与 H5 实际支持的 PATCH，仅在传输边界补全该方法。
    const send = uni.request as (options: Omit<UniApp.RequestOptions, 'method'> & { method: HttpMethod }) => unknown
    try { send({ url: API_BASE + path, method, data: data as UniApp.RequestOptions['data'], timeout: 15000,
      header: { 'Content-Type': 'application/json', ...(authToken ? { Authorization: 'Bearer ' + authToken } : {}) },
      /** 检查 HTTP 状态后解包业务数据。 */
      success(response) { try { const body = envelope(response.data); if (response.statusCode >= 200 && response.statusCode < 300) { if (authenticated && authToken && method !== 'GET') queueModerationNotice(body.moderation, () => token() === authToken); resolve(body.data as T) } else { const error = new ApiError(body.error || '请求失败，请稍后重试'); error.status = response.statusCode; error.code = body.code; reject(error) } } catch (error) { reject(error) } finally { finishFinancial() } },
      /** 网络错误不应清理用户会话。 */
      fail() { finishFinancial(); reject(new ApiError('暂时连接不上服务，请确认服务已启动后重试')) },
    }) } catch(error) { finishFinancial(); reject(error) }
  })
}
/** 向服务端发送正常业务请求，由领域API约束模型与权限。 */
export function request<T>(path: string, method: HttpMethod = 'GET', data?: unknown): Promise<T> { return sendRequest(path,method,data,true) }
/** 只读公共内容时明确不携带身份，不暴露写方法给匿名分享流程。 */
export function requestPublic<T>(path:string):Promise<T> { return sendRequest(path,'GET',undefined,false) }
/** 上传本地图片并按同一响应规则解包。 */
export function upload<T>(path: string, filePath: string, uploadToken = token()): Promise<T> {
  return new Promise((resolve, reject) => uni.uploadFile({ url: API_BASE + path, filePath, name: 'file', header: { Authorization: 'Bearer ' + uploadToken },
    /** 处理上传响应及服务端拒绝。 */
    success(response) { try { const body = envelope(JSON.parse(response.data)); if (response.statusCode >= 200 && response.statusCode < 300) { queueModerationNotice(body.moderation, () => !!uploadToken && token() === uploadToken, true); resolve(body.data as T) } else { const error = new ApiError(body.error || '图片上传失败'); error.status = response.statusCode; error.code = body.code; reject(error) } } catch (error) { reject(error instanceof ApiError ? error : new ApiError('图片上传响应异常')) } },
    /** 显示可重试的上传错误。 */
    fail() { reject(new ApiError('图片上传失败，请重试')) },
  }))
}
/**
 * 下载同源受限图片，令牌只放请求头；旧账号的临时文件不交给页面。
 * @param path - 业务API固定提供的站内路径。
 * @returns 微信或H5运行环境的临时文件地址。
 */
export function download(path: string): Promise<string> {
  const requestToken = token()
  if (!path.startsWith('/') || path.startsWith('//') || /[\r\n]/.test(path)) return Promise.reject(new ApiError('图片接口地址无效'))
  if (!requestToken) { const error = new ApiError('请先登录'); error.status = 401; return Promise.reject(error) }
  return new Promise((resolve, reject) => uni.downloadFile({ url: API_BASE + path, header: { Authorization: 'Bearer ' + requestToken }, timeout: 15000,
    /** 只交付当前会话下成功下载的文件，失败和换号时清理临时内容。 */
    success(response) {
      if (requestToken === token() && response.statusCode === 200 && response.tempFilePath) return resolve(response.tempFilePath)
      if (response.tempFilePath) removeDownloadedFile(response.tempFilePath)
      const error = new ApiError(requestToken !== token() ? '账号已切换，已取消图片读取' : '暂时无法读取审核图片')
      if (requestToken !== token()) error.cancelled = true
      else error.status = response.statusCode
      reject(error)
    },
    /** 下载失败不影响原审核任务，允许页面重新读取。 */
    fail() { reject(new ApiError('审核图片下载失败，请重试')) },
  }))
}
/** 删除未交付给页面的临时下载文件，网页环境交由浏览器回收。 */
function removeDownloadedFile(filePath: string): void {
  try { if (typeof uni.getFileSystemManager === 'function') uni.getFileSystemManager().unlink({ filePath, fail() { /* 资源已不存在时无需再次处理。 */ } }) } catch { /* 临时文件删除失败不能掩盖原下载错误。 */ }
}
/** 对查询条件逐项编码，跳过未定义字段。 */
export function queryString(query: object = {}): string { const pairs = Object.entries(query).filter(([, value]) => value !== undefined && value !== null && value !== ''); return pairs.length ? '?' + pairs.map(([key, value]) => encodeURIComponent(key) + '=' + encodeURIComponent(String(value))).join('&') : '' }
