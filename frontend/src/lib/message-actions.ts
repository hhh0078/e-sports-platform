/** 文件作用：把通知中的可选行动信息限制为现有站内业务入口，拒绝外域、任意路径和管理越权跳转。 */
import type { SystemMessage } from '../types/messages'

/** 已校验、可直接交给站内导航的轻量行动。 */
export interface MessageAction { path: string; label: string }
const plainRoutes:Record<string,string>={'/pages/account/profile':'修改个人资料','/pages/community/clubs/index':'查看俱乐部'}
const managementRoutes:Record<string,string>={'/pages/manage/support':'修改客服信息','/pages/manage/platform':'修改平台信息','/pages/manage/games':'查看游戏配置','/pages/manage/messages':'查看消息管理','/pages/manage/content/index':'修改公共信息','/pages/manage/articles/index':'查看文章管理'}
const objectRoutes:Record<string,string>={'/pages/services/publish':'修改服务','/pages/community/clubs/edit':'修改俱乐部资料','/pages/community/clubs/detail':'查看俱乐部','/pages/community/clubs/console':'管理俱乐部','/pages/orders/detail':'查看订单','/pages/manage/articles/edit':'修改文章内容'}

/**
 * 校验服务端消息行动路径，仅接受精确页面和单个合法对象ID参数。
 * @param message - 当前账号已缓存的消息；正文和URL均不能充当导航代码。
 * @param role - 当前服务端会话角色，管理配置入口只向平台管理员显示。
 * @returns 安全站内行动；缺省、恶意或无权限时返回null。
 */
export function safeMessageAction(message:Pick<SystemMessage,'action_path'|'action_label'>|null|undefined,role?:string):MessageAction|null{
  const raw=message?.action_path
  if(typeof raw!=='string'||!raw.startsWith('/pages/')||raw!==raw.trim()||raw.length>500||/[\r\n\\#]/.test(raw))return null
  let path=raw,fallback=plainRoutes[raw]
  if(managementRoutes[raw]){if(role!=='admin')return null;fallback=managementRoutes[raw]}
  if(!fallback){
    const match=raw.match(/^(\/pages\/[a-z/]+)\?id=([^&?#]+)$/)
    if(!match||!objectRoutes[match[1]])return null
    if(match[1].startsWith('/pages/manage/')&&role!=='admin')return null
    let id:string
    try{id=decodeURIComponent(match[2].replace(/\+/g,' '))}catch{return null}
    if(!/^[A-Za-z0-9][A-Za-z0-9_.:-]{0,127}$/.test(id))return null
    path=match[1]+'?id='+encodeURIComponent(id);fallback=objectRoutes[match[1]]
  }
  const proposed=typeof message?.action_label==='string'?message.action_label.trim():''
  const label=proposed&&Array.from(proposed).length<=30&&!/[\r\n\t]/.test(proposed)?proposed:fallback
  return{path,label}
}
