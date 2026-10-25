/** 文件作用：按展示用途调用官方媒体裁剪压缩，复核大小后交给业务上传接口。 */
import { token, ApiError } from '../api/client'
import { uploadMediaImage, type ImagePurpose } from '../api/media.api'
/** 图片按最终展示比例和分辨率限制。 */
interface ImageRule { label: string; ratio?: string; width: number; qr?: boolean }
/** 原生媒体回调。 */
interface MediaCallbacks<T> { success: (result: T) => void; fail: (error: { errMsg?: string }) => void }
/** 仅声明实际使用的微信官方图片接口。 */
interface WechatMedia {
  canIUse: (name: string) => boolean;
  chooseMedia: (options: MediaCallbacks<{ tempFiles: Array<{ tempFilePath: string }> }> & { count: number; mediaType: string[]; sizeType: string[]; sourceType: string[] }) => void;
  cropImage: (options: MediaCallbacks<{ tempFilePath: string }> & { src: string; cropScale: string }) => void;
  compressImage: (options: MediaCallbacks<{ tempFilePath: string }> & { src: string; quality: number; compressedWidth: number }) => void;
  getImageInfo: (options: MediaCallbacks<{ width: number; height: number }> & { src: string }) => void;
  getFileSystemManager: () => { getFileInfo: (options: MediaCallbacks<{ size: number }> & { filePath: string }) => void };
}
declare const wx: WechatMedia
/** 原生或网页选图选项。 */
export interface ImageOptions { filePath?: string; clubId?: string }
export const IMAGE_RULES: Record<ImagePurpose, ImageRule> = {
  avatar: { label: '头像 · 1:1', ratio: '1:1', width: 512 },
  profile_cover: { label: '个人封面 · 16:9', ratio: '16:9', width: 1280 },
  club_cover: { label: '俱乐部封面 · 16:9', ratio: '16:9', width: 1280 },
  club_detail: { label: '图文详情 · 保留原比例', width: 1280 },
  support_qr: { label: '客服二维码 · 1:1 · 保留完整白边', ratio: '1:1', width: 800, qr: true },
  club_contact_qr: { label: '俱乐部二维码 · 1:1 · 保留完整白边', ratio: '1:1', width: 800, qr: true },
  game_cover: { label: '游戏封面 · 16:9', ratio: '16:9', width: 1280 },
  game_icon: { label: '游戏图标 · 1:1', ratio: '1:1', width: 256 },
  evidence: { label: '订单证据 · 保留完整图片比例', width: 1600 },
  promotion_banner: { label: '运营轮播 · 16:9', ratio:'16:9',width:1280 },
  announcement_image: { label: '公告图片 · 保留原比例，不裁剪',width:1280 },
  article_cover: { label: '文章封面 · 16:9',ratio:'16:9',width:1280 },
  article_image: { label: '文章插图 · 保留原比例',width:1280 },
}
const MAX_BYTES = 3 * 1024 * 1024
/** 将单次官方媒体操作转换为 Promise，并标记主动取消。 */
function invoke<T>(run: (callbacks: MediaCallbacks<T>) => void): Promise<T> { return new Promise((resolve, reject) => run({ success: resolve, fail(error) { const message = error.errMsg || '图片处理失败'; const failure = new ApiError(/cancel/.test(message) ? '已取消选择' : message); failure.cancelled = /cancel/.test(message); reject(failure) } })) }
/** 选择前说明裁剪比例和上传大小限制。 */
async function explain(rule: ImageRule): Promise<void> {
  let cropHint = rule.ratio ? '下一步可调整裁剪区域。' : '不会裁掉图片内容。'
  // #ifdef H5
  if (rule.ratio) cropHint = '本地预览将按显示比例居中裁剪。'
  // #endif
  const result = await invoke<{ confirm: boolean }>(callbacks => uni.showModal({ title: rule.label, content: `图片会压缩至宽度不超过 ${rule.width}px，上传大小不超过 3MB。${rule.qr?'请保留完整二维码和四周白边，优先保留清晰度，通常约500KB以内。':''}${cropHint}`, confirmText: '选择图片', ...callbacks }))
  if (!result.confirm) { const error = new ApiError('已取消选择'); error.cancelled = true; throw error }
}
/** 读取压缩后实际字节数，不能信任原始文件大小。 */
async function fileSize(path: string): Promise<number> {
  // #ifdef MP-WEIXIN
  return (await invoke<{ size: number }>(callbacks => wx.getFileSystemManager().getFileInfo({ filePath: path, ...callbacks }))).size
  // #endif
  // #ifdef H5
  return (await (await fetch(path)).blob()).size
  // #endif
}
/** H5 体验按用途比例居中裁剪并压缩，微信端使用官方裁剪。 */
async function processWeb(path: string, rule: ImageRule): Promise<string> {
  const source = await new Promise<HTMLImageElement>((resolve, reject) => { const item = new Image(); item.onload = () => resolve(item); item.onerror = () => reject(new Error('无法读取所选图片')); item.src = path })
  const dimensions = rule.ratio?.split(':').map(Number)
  const ratio = dimensions ? dimensions[0] / dimensions[1] : source.width / source.height
  const cropWidth = Math.min(source.width, source.height * ratio), cropHeight = cropWidth / ratio
  const canvas = document.createElement('canvas'); canvas.width = Math.round(Math.min(rule.width, cropWidth)); canvas.height = Math.round(canvas.width / ratio)
  const context = canvas.getContext('2d'); if (!context) throw new Error('当前环境不支持图片处理')
  if (rule.qr) { context.fillStyle = '#fff'; context.fillRect(0, 0, canvas.width, canvas.height) }
  context.drawImage(source, (source.width - cropWidth) / 2, (source.height - cropHeight) / 2, cropWidth, cropHeight, 0, 0, canvas.width, canvas.height)
  const blob = await new Promise<Blob | null>(resolve => canvas.toBlob(resolve, rule.qr ? 'image/png' : 'image/jpeg', rule.qr ? undefined : 0.82))
  if (!blob || blob.size > MAX_BYTES) throw new Error('图片压缩后仍大于3MB，请选择较小图片')
  return URL.createObjectURL(blob)
}
/** 按最终用途调用官方选图、裁剪、压缩并检查文件上限。 */
export async function prepareImage(purpose: ImagePurpose, options: ImageOptions = {}): Promise<string> {
  const rule = IMAGE_RULES[purpose]
  if (!rule) throw new Error('未知的图片用途')
  let path = options.filePath
  if (!path) {
    await explain(rule)
    // #ifdef MP-WEIXIN
    path = (await invoke<{ tempFiles: Array<{ tempFilePath: string }> }>(callbacks => wx.chooseMedia({ count: 1, mediaType: ['image'], sizeType: ['original', 'compressed'], sourceType: ['album', 'camera'], ...callbacks }))).tempFiles[0]?.tempFilePath
    // #endif
    // #ifdef H5
    const chosen = await invoke<UniApp.ChooseImageSuccessCallbackResult>(callbacks => uni.chooseImage({ count: 1, sizeType: ['original', 'compressed'], ...callbacks }))
    path = typeof chosen.tempFilePaths === 'string' ? chosen.tempFilePaths : chosen.tempFilePaths[0]
    // #endif
  }
  if (!path) throw new Error('没有选中图片')
  // #ifdef MP-WEIXIN
  if (rule.ratio) { if (!wx.canIUse('cropImage')) throw new Error('当前微信不支持官方图片裁剪，请升级微信后重试'); const src = path; path = (await invoke<{ tempFilePath: string }>(callbacks => wx.cropImage({ src, cropScale: rule.ratio!, ...callbacks }))).tempFilePath }
  const info = await invoke<{ width: number; height: number }>(callbacks => wx.getImageInfo({ src: path!, ...callbacks }))
  if (info.width * info.height > 40000000) throw new Error('图片分辨率过大，请先选择较小图片')
  for (const quality of rule.qr ? [96, 92] : [82, 65, 45]) { const src = path; path = (await invoke<{ tempFilePath: string }>(callbacks => wx.compressImage({ src, quality, compressedWidth: Math.min(rule.width, info.width), ...callbacks }))).tempFilePath; if (await fileSize(path) <= (rule.qr ? 500*1024 : MAX_BYTES)) break }
  if (await fileSize(path) > MAX_BYTES) throw new Error('图片压缩后仍大于3MB，请换一张图片')
  // #endif
  // #ifdef H5
  path = await processWeb(path, rule)
  // #endif
  return path
}
/** 选择并上传图片，原生选图回来后重新核验账号。 */
export async function chooseUploadImage(purpose: ImagePurpose, options: ImageOptions = {}): Promise<string> {
  const uploadToken = token(), path = await prepareImage(purpose, options)
  if (!uploadToken || uploadToken !== token()) throw new Error('账号已切换，请重新选择图片')
  try { const uploaded = await uploadMediaImage(purpose, path, options.clubId, uploadToken); if (!uploaded?.url) throw new Error('图片上传返回格式异常'); return uploaded.url }
  finally {
    // #ifdef H5
    if (path.startsWith('blob:')) URL.revokeObjectURL(path)
    // #endif
  }
}
