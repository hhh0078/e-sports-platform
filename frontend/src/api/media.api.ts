/** 文件作用：根据图片展示用途封装上传接口，页面无需接触上传地址和传输方法。 */
import { upload, queryString, token } from './client'
import type { UploadedImage } from '../types/models'
/** 允许上传的图片用途。 */
export type ImagePurpose = 'avatar' | 'profile_cover' | 'club_cover' | 'club_detail' | 'club_contact_qr' | 'support_qr' | 'game_cover' | 'game_icon' | 'evidence' | 'promotion_banner' | 'announcement_image' | 'article_cover' | 'article_image'
/** 上传已完成官方裁剪压缩的安全图片。 */
export function uploadMediaImage(purpose: ImagePurpose, path: string, clubId?: string, uploadToken = token()): Promise<UploadedImage> { return upload(purpose.startsWith('game_') ? '/admin/covers' : '/media/images' + queryString({ purpose, club_id: clubId }), path, uploadToken) }
