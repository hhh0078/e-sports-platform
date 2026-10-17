/** 文件作用：统一请求、会话鉴权与订单图片证据上传。 */
export const API_BASE = import.meta.env?.VITE_API_BASE || 'http://127.0.0.1:8090/api'

/** 获取持久化会话令牌，避免组件重复读取。 */
export function token() { return uni.getStorageSync('jijie_token') || '' }

/**
 * 请求服务端并统一解析业务错误。
 * @param {string} path - API 相对路径
 * @param {string} method - HTTP 请求方法
 * @param {object} data - 请求正文
 * @returns {Promise<any>} 解包后的业务数据
 */
export function api(path, method = 'GET', data) {
  return new Promise((resolve, reject) => {
    uni.request({
      url: API_BASE + path, method, data, timeout: 15000,
      header: { 'Content-Type': 'application/json', ...(token() ? { Authorization: 'Bearer ' + token() } : {}) },
      /** 解析 HTTP 与业务错误，保留登录失败状态供调用方判断。 */
      success(response) {
        if (response.statusCode >= 200 && response.statusCode < 300) resolve(response.data?.data)
        else { const error = new Error(response.data?.error || '请求失败，请稍后重试'); error.status = response.statusCode; reject(error) }
      },
      /** 将网络错误转为可读提示，不清除用户会话。 */
      fail() { reject(new Error('暂时连接不上服务，请确认服务已启动后重试')) },
    })
  })
}

/** 上传当前订单的图片证据，携带会话身份。 */
export function uploadEvidence(orderId, filePath) {
  return new Promise((resolve, reject) => {
    uni.uploadFile({
      url: API_BASE + '/orders/' + encodeURIComponent(orderId) + '/evidence', filePath, name: 'file',
      header: { Authorization: 'Bearer ' + token() },
      /** 校验响应并返回证据地址及文件摘要。 */
      success(response) { try { const body = JSON.parse(response.data); if (response.statusCode >= 200 && response.statusCode < 300) resolve(body.data); else reject(new Error(body.error || '图片上传失败')) } catch { reject(new Error('图片上传响应异常')) } },
      /** 提示网络上传失败。 */
      fail() { reject(new Error('图片上传失败，请重试')) },
    })
  })
}
