/** 文件作用：声明公共运营信息、独立轮播图层、公告、Markdown文章及安全跳转契约。 */
import type { CoverLabel } from './models'
/** 运营内容只允许三类站内目标，禁止保存任意URL。 */
export interface ContentTarget { kind:'service'|'request'|'article';id:string }
/** 图片项与唯一游戏槽沿用同一顺序数组，游戏槽不含图文。 */
export interface ContentSlide { id:string;kind:'image'|'game_slot';image_url:string;labels:CoverLabel[];target:ContentTarget|null }
/** 首页公告以服务端版本区分已读，不因排序或开关重复生成版本。 */
export interface ContentAnnouncement { id:string;version:number;enabled:boolean;frequency:'always'|'once';mode:'image'|'text';image_url:string;title:string;body:string;target:ContentTarget|null }
/** 四主视图提示允许多行文字和可选已发布文章。 */
export interface ContentTip { text:string;article_id:string }
/** 四主导航各自拥有独立提示配置。 */
export type PageTips=Record<'player'|'worker'|'orders'|'me',ContentTip>
/** 管理配置使用修订号避免多人编辑互相覆盖。 */
export interface AdminPublicContent { revision:number;global_slides:ContentSlide[];game_slides:Record<string,ContentSlide[]>;announcement:ContentAnnouncement;page_tips:PageTips;service_notice:string;request_notice:string;updated_at:string }
/** 公开结果已展开当前游戏轮播，不暴露后台草稿或私密字段。 */
export interface PublicContent { revision:number;game_id:string;slides:ContentSlide[];announcement:ContentAnnouncement;page_tips:PageTips;service_notice:string;request_notice:string }
/** 配置保存只提交当前分节，游戏数组按提供的游戏键合并。 */
export type PublicContentPatch=Pick<AdminPublicContent,'revision'>&Partial<Omit<AdminPublicContent,'revision'|'updated_at'>>
/** 文章发布状态。 */
export type ArticleStatus='draft'|'published'|'hidden'
/** 列表摘要不包含编辑源与作者私密信息。 */
export interface ArticleSummary {id:string;title:string;summary:string;cover_url:string;status:ArticleStatus;revision:number;created_at:string;updated_at:string;published_at:string}
/** 管理员编辑使用Markdown原文。 */
export interface AdminArticle extends ArticleSummary {body_markdown:string}
/** 公开正文由服务器当前审核投影安全生成。 */
export interface PublicArticle extends ArticleSummary {body_html:string}
/** 管理表单沿用后端长度及修订检查。 */
export interface ArticleInput {title:string;summary:string;body_markdown:string;cover_url:string;status:ArticleStatus;revision?:number}
/** 文章按真实总数分页。 */
export interface ArticlePage {items:ArticleSummary[];total:number}
/** 搜索结果只列出当前可公开的站内目标。 */
export interface ContentTargetOption extends ContentTarget {title:string;image_url:string;game_id:string;subtitle:string}
