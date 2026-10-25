/** 文件作用：集中封装公共信息、公告已读、文章编辑与目标搜索，页面不直接构造网络请求。 */
import { request,requestPublic,queryString } from './client'
import type { AdminArticle,AdminPublicContent,ArticleInput,ArticlePage,ArticleStatus,ContentTarget,ContentTargetOption,PublicArticle,PublicContent,PublicContentPatch } from '../types/content'
/** 访客读取当前游戏已展开的运营信息。 */
export function getPublicContent(gameId:string):Promise<PublicContent>{return requestPublic('/public-content'+queryString({game_id:gameId}))}
/** 管理员读取完整运营编辑稿。 */
export function getAdminPublicContent():Promise<AdminPublicContent>{return request('/admin/public-content')}
/** 按版本保存当前分节，不覆盖其它游戏或运营设置。 */
export function savePublicContent(body:PublicContentPatch):Promise<AdminPublicContent>{return request('/admin/public-content','PATCH',body)}
/** 读取当前账号的公告版本已读记录，不在读取时写已读。 */
export function getAnnouncementState(version:number):Promise<{id:string;version:number;seen:boolean}>{return request('/me/announcement-state'+queryString({version}))}
/** 公告真正打开后幂等确认已读。 */
export function acknowledgeAnnouncement(version:number):Promise<{ok:boolean}>{return request('/me/announcement-ack','POST',{id:'home',version})}
/** 访客分页搜索已发布文章。 */
export function listArticles(query:{q?:string;limit?:number;offset?:number}={}):Promise<ArticlePage>{return requestPublic('/articles'+queryString(query))}
/** 访客读取当前安全正文，失效目标由后端返回404。 */
export function getArticle(id:string):Promise<PublicArticle>{return requestPublic('/articles/'+encodeURIComponent(id))}
/** 管理员分页读取文章草稿与发布状态。 */
export function listAdminArticles(query:{q?:string;status?:ArticleStatus|'';limit?:number;offset?:number}={}):Promise<ArticlePage>{return request('/admin/articles'+queryString(query))}
/** 管理员读取文章Markdown源。 */
export function getAdminArticle(id:string):Promise<AdminArticle>{return request('/admin/articles/'+encodeURIComponent(id))}
/** 新建文章并在同事务加入图文审核。 */
export function createArticle(body:ArticleInput):Promise<AdminArticle>{return request('/admin/articles','POST',body)}
/** 保存既有文章，修订冲突不能覆盖新内容。 */
export function updateArticle(id:string,body:ArticleInput):Promise<AdminArticle>{return request('/admin/articles/'+encodeURIComponent(id),'PATCH',body)}
/** 安全预览不保存业务记录或创建审核任务。 */
export function previewArticle(body:string):Promise<{body_html:string}>{return request('/admin/articles/preview','POST',{body_markdown:body})}
/** 只搜索公开服务、可接单需求及已发布文章。 */
export function searchContentTargets(kind:ContentTarget['kind'],q:string,offset=0):Promise<{items:ContentTargetOption[];total:number}>{return request('/admin/content-targets'+queryString({kind,q,limit:20,offset}))}
