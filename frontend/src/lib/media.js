/** 文件作用：统一按展示用途裁剪、压缩、检查并上传图片；微信端采用官方媒体接口。 */
import { API_BASE, token } from './api'

export const IMAGE_RULES = {
  avatar: { label: '头像 · 1:1', ratio: '1:1', width: 512 },
  profile_cover: { label: '个人封面 · 16:9', ratio: '16:9', width: 1280 },
  club_cover: { label: '俱乐部封面 · 16:9', ratio: '16:9', width: 1280 },
  club_detail: { label: '图文详情 · 保留原比例', width: 1280 },
  game_cover: { label: '游戏封面 · 16:9（服务卡片居中显示8:5）', ratio: '16:9', width: 1280 },
  game_icon: { label: '游戏图标 · 1:1', ratio: '1:1', width: 256 },
  evidence: { label: '订单证据 · 保留完整图片比例', width: 1600 },
}
const MAX_BYTES = 3 * 1024 * 1024

/** 将官方回调接口转换为 Promise，同时将取消视为可识别的用户行为。 */
function invoke(method, options) {
  return new Promise((resolve, reject) => method({ ...options, success: resolve, fail: error => { const message = error?.errMsg || '图片处理失败'; const failure = new Error(/cancel/.test(message) ? '已取消选择' : message); failure.cancelled = /cancel/.test(message); reject(failure) } }))
}

/** 在选择前解释图片展示位置，避免用户不知道为什么需要裁剪。 */
async function explain(rule) {
  let cropHint = rule.ratio ? '下一步可调整裁剪区域。' : '不会裁掉图片内容。'
  // #ifdef H5
  if (rule.ratio) cropHint = '本地预览将按显示比例居中裁剪。'
  // #endif
  const result = await invoke(uni.showModal, { title: rule.label, content: `图片会压缩至宽度不超过 ${rule.width}px，上传大小不超过 3MB。${cropHint}`, confirmText: '选择图片' })
  if (!result.confirm) { const error = new Error('已取消选择'); error.cancelled = true; throw error }
}

/** 微信端读取压缩后实际大小，不能只依赖调用前的原文件大小。 */
async function fileSize(path) {
  // #ifdef MP-WEIXIN
  const result = await invoke(wx.getFileSystemManager().getFileInfo.bind(wx.getFileSystemManager()), { filePath: path })
  return result.size
  // #endif
  // #ifdef H5
  return (await (await fetch(path)).blob()).size
  // #endif
}

/** H5 本地预览用中心裁剪与 JPEG 压缩；正式小程序走官方裁剪界面。 */
async function processWeb(path, rule) {
  // #ifdef H5
  const source = await new Promise((resolve, reject) => { const item = new Image(); item.onload = () => resolve(item); item.onerror = () => reject(new Error('无法读取所选图片')); item.src = path })
  const ratio = rule.ratio ? rule.ratio.split(':').map(Number).reduce((a,b) => a/b) : source.width / source.height
  const cropWidth = Math.min(source.width, source.height * ratio), cropHeight = cropWidth / ratio
  const width = Math.min(rule.width, cropWidth), height = Math.round(width / ratio)
  const canvas = document.createElement('canvas'); canvas.width = Math.round(width); canvas.height = height
  const context = canvas.getContext('2d'); context.drawImage(source, (source.width-cropWidth)/2, (source.height-cropHeight)/2, cropWidth, cropHeight, 0, 0, canvas.width, height)
  const blob = await new Promise(resolve => canvas.toBlob(resolve, 'image/jpeg', 0.82))
  if (!blob || blob.size > MAX_BYTES) throw new Error('图片压缩后仍大于3MB，请选择较小图片')
  return URL.createObjectURL(blob)
  // #endif
}

/** 按图片用途选择、官方裁剪和压缩，返回经过大小与比例检查的本地文件路径。 */
export async function prepareImage(purpose, options = {}) {
  const rule = IMAGE_RULES[purpose]
  if (!rule) throw new Error('未知的图片用途')
  let path = options.filePath
  if (!path) {
    await explain(rule)
    // #ifdef MP-WEIXIN
    const chosen = await invoke(wx.chooseMedia, { count: 1, mediaType: ['image'], sizeType: ['original','compressed'], sourceType: ['album','camera'] })
    path = chosen.tempFiles?.[0]?.tempFilePath
    // #endif
    // #ifdef H5
    const chosen = await invoke(uni.chooseImage, { count: 1, sizeType: ['original','compressed'] })
    path = chosen.tempFilePaths?.[0]
    // #endif
  }
  if (!path) throw new Error('没有选中图片')
  // #ifdef MP-WEIXIN
  if (rule.ratio) {
    if (!wx.canIUse('cropImage')) throw new Error('当前微信不支持官方图片裁剪，请升级微信后重试')
    path = (await invoke(wx.cropImage, { src: path, cropScale: rule.ratio })).tempFilePath
  }
  const info = await invoke(wx.getImageInfo, { src: path })
  if (info.width * info.height > 40000000) throw new Error('图片分辨率过大，请先选择较小图片')
  for (const quality of [82, 65, 45]) {
    path = (await invoke(wx.compressImage, { src: path, quality, compressedWidth: Math.min(rule.width, info.width) })).tempFilePath
    if (await fileSize(path) <= MAX_BYTES) break
  }
  if (await fileSize(path) > MAX_BYTES) throw new Error('图片压缩后仍大于3MB，请换一张图片')
  // #endif
  // #ifdef H5
  path = await processWeb(path, rule)
  // #endif
  return path
}

/** 上传经过处理的图片并校验响应，生产环境只传递服务端生成的安全媒体地址。 */
export function uploadImageFile(path, endpoint, uploadToken = token()) {
  return new Promise((resolve, reject) => uni.uploadFile({ url: API_BASE + endpoint, filePath: path, name: 'file', header: { Authorization: 'Bearer ' + uploadToken },
    success(response) { try { const body = JSON.parse(response.data); if (response.statusCode >= 200 && response.statusCode < 300 && body.data?.url) resolve(body.data); else reject(new Error(body.error || '图片上传失败')) } catch { reject(new Error('图片上传返回格式异常')) } },
    fail() { reject(new Error('图片上传失败，请重试')) },
    complete() {
      // #ifdef H5
      if (path.startsWith('blob:')) URL.revokeObjectURL(path)
      // #endif
    },
  }))
}

/** 个人与俱乐部统一上传入口，返回可保存到个人资料或装修内容的图片 URL。 */
export async function chooseUploadImage(purpose, options = {}) {
  const uploadToken = token()
  const path = await prepareImage(purpose, options)
  if (!uploadToken || uploadToken !== token()) throw new Error('账号已切换，请重新选择图片')
  const endpoint = purpose.startsWith('game_') ? '/admin/covers' : '/media/images?purpose=' + encodeURIComponent(purpose) + (options.clubId ? '&club_id=' + encodeURIComponent(options.clubId) : '')
  return (await uploadImageFile(path, endpoint, uploadToken)).url
}
