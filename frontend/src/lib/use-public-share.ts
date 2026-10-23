/** 文件作用：共享生成好友/朋友圈分享结果；页面必须直接注册官方钩子，以便uni编译器生成微信事件标记。 */
import { assetUrl } from './format'
/** 页面实时提供公开标题、路径、封面；不能传订单或身份秘密。 */
export interface PublicSharePreview { title: string; path: string; image: string; friendImage?: string; timelineImage?: string; timelineQuery?: string }
/** 微信好友页面分享的同步返回模型。 */
export interface FriendShareResult { title: string; path: string; imageUrl: string }
/** 微信朋友圈只能指定当前页面query，不支持修改页面路径。 */
export interface TimelineShareResult { title: string; query: string; imageUrl: string }
/** 两个纯处理器必须在实际页面script里分别传给官方注册函数。 */
export interface PublicShareHandlers { friend: () => FriendShareResult; timeline: () => TimelineShareResult }
/**
 * 为公开内容创建共享处理器，内部不注册任何生命周期，避免uni静态分析漏掉微信事件。
 * @param read - 每次分享时读取该页面当前有效的公开内容。
 * @returns 分别交给onShareAppMessage和onShareTimeline的同步处理器。
 */
export function createPublicShareHandlers(read: () => PublicSharePreview): PublicShareHandlers {
  return {
    /** 返回原始公开目标路径与独立好友封面，缺图时使用已打包的游戏图。 */
    friend() { const data=read();return { title:data.title,path:data.path,imageUrl:data.friendImage||assetUrl(data.image)||'/static/share/default-cover.jpg' } },
    /** 保持朋友圈当前页路由，使用明确query与独立正方形封面。 */
    timeline() { const data=read(),index=data.path.indexOf('?');return {title:data.title,query:data.timelineQuery ?? (index>=0?data.path.slice(index+1):''),imageUrl:data.timelineImage||assetUrl(data.image)||'/static/share/default-cover.jpg'} },
  }
}
