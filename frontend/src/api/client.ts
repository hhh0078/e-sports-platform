/** 文件作用：唯一 HTTP/上传传输层，统一超时、令牌、业务解包及可读错误。 */
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
function envelope(value: unknown): { data?: unknown; error?: string; code?: string } {
  if (typeof value !== 'object' || value === null) throw new ApiError('服务器返回格式异常')
  const body = value as Record<string, unknown>
  if (!Object.prototype.hasOwnProperty.call(body, 'data') && typeof body.error !== 'string') throw new ApiError('服务器返回格式异常')
  return { data: body.data, error: typeof body.error === 'string' ? body.error : undefined, code: typeof body.code === 'string' ? body.code : undefined }
}
/** 向服务端发送请求，由业务 API 指定并约束返回模型。 */
export function request<T>(path: string, method: HttpMethod = 'GET', data?: unknown): Promise<T> {
  return new Promise((resolve, reject) => {
    const authToken = token()
    // uni-app 的类型枚举遗漏了微信与 H5 实际支持的 PATCH，仅在传输边界补全该方法。
    const send = uni.request as (options: Omit<UniApp.RequestOptions, 'method'> & { method: HttpMethod }) => unknown
    send({ url: API_BASE + path, method, data: data as UniApp.RequestOptions['data'], timeout: 15000,
      header: { 'Content-Type': 'application/json', ...(authToken ? { Authorization: 'Bearer ' + authToken } : {}) },
      /** 检查 HTTP 状态后解包业务数据。 */
      success(response) { try { const body = envelope(response.data); if (response.statusCode >= 200 && response.statusCode < 300) resolve(body.data as T); else { const error = new ApiError(body.error || '请求失败，请稍后重试'); error.status = response.statusCode; error.code = body.code; reject(error) } } catch (error) { reject(error) } },
      /** 网络错误不应清理用户会话。 */
      fail() { reject(new ApiError('暂时连接不上服务，请确认服务已启动后重试')) },
    })
  })
}
/** 上传本地图片并按同一响应规则解包。 */
export function upload<T>(path: string, filePath: string, uploadToken = token()): Promise<T> {
  return new Promise((resolve, reject) => uni.uploadFile({ url: API_BASE + path, filePath, name: 'file', header: { Authorization: 'Bearer ' + uploadToken },
    /** 处理上传响应及服务端拒绝。 */
    success(response) { try { const body = envelope(JSON.parse(response.data)); if (response.statusCode >= 200 && response.statusCode < 300) resolve(body.data as T); else { const error = new ApiError(body.error || '图片上传失败'); error.status = response.statusCode; error.code = body.code; reject(error) } } catch (error) { reject(error instanceof ApiError ? error : new ApiError('图片上传响应异常')) } },
    /** 显示可重试的上传错误。 */
    fail() { reject(new ApiError('图片上传失败，请重试')) },
  }))
}
/** 对查询条件逐项编码，跳过未定义字段。 */
export function queryString(query: object = {}): string { const pairs = Object.entries(query).filter(([, value]) => value !== undefined && value !== null && value !== ''); return pairs.length ? '?' + pairs.map(([key, value]) => encodeURIComponent(key) + '=' + encodeURIComponent(String(value))).join('&') : '' }
