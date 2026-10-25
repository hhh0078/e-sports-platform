/** 文件作用：封装平台用户与组织成员管理接口，页面只调用具名方法，所有范围与权限由服务端确认。 */
import { request,queryString } from './client'
import type { User,Order,ListQuery } from '../types/models'
import type { ProfileInput } from './users.api'
import type { Wallet,WalletEntry } from '../types/wallet'
import type { ManagementPage,ManagedUserDetail,ManagedMemberDetail,ManagedClubMember,ScopedMemberEntry,ModerationRecord } from '../types/management'
/** 生成已编码的平台用户管理前缀，不接受客户端额外账户范围。 */
function userPath(id:string):string{return '/admin/users/'+encodeURIComponent(id)}
/** 生成已编码的本组织成员前缀。 */
function memberPath(clubID:string,userID:string):string{return '/clubs/'+encodeURIComponent(clubID)+'/members/'+encodeURIComponent(userID)}
/** 读取平台用户资料及操作权限。 */
export function getManagedUser(id:string):Promise<ManagedUserDetail>{return request(userPath(id))}
/** 只保存允许编辑的公开基础资料。 */
export function updateManagedUser(id:string,body:ProfileInput):Promise<User>{return request(userPath(id)+'/profile','PATCH',body)}
/** 根管理员授予或撤销平台副管理员。 */
export function setPlatformDeputy(id:string,enabled:boolean):Promise<User>{return request(userPath(id)+'/deputy','PATCH',{enabled})}
/** 向当前目标用户定向发送站内消息。 */
export function sendManagedUserMessage(id:string,body:{title:string;body:string}):Promise<{recipient_count:number}>{return request(userPath(id)+'/messages','POST',body)}
/** 只读该用户真实钱包，不触发结算或改账。 */
export function getManagedUserWallet(id:string):Promise<Wallet>{return request(userPath(id)+'/wallet')}
/** 按目标用户读取资金流水。 */
export function listManagedUserEntries(id:string,query:ListQuery):Promise<ManagementPage<WalletEntry>>{return request(userPath(id)+'/wallet/entries'+queryString(query))}
/** 按目标用户和买卖身份读取订单。 */
export function listManagedUserOrders(id:string,query:ListQuery&{role?:'all'|'buyer'|'provider'}):Promise<ManagementPage<Order>>{return request(userPath(id)+'/orders'+queryString(query))}
/** 读取该用户图文审核安全摘要。 */
export function listManagedUserModeration(id:string,query:ListQuery):Promise<ManagementPage<ModerationRecord>>{return request(userPath(id)+'/moderation'+queryString(query))}
/** 仅当前组织有权者搜索和分页读取成员。 */
export function listManagedMembers(clubID:string,query:ListQuery):Promise<ManagementPage<ManagedClubMember>>{return request('/clubs/'+encodeURIComponent(clubID)+'/members'+queryString(query))}
/** 读取组织内目标成员资料及范围资金。 */
export function getManagedMember(clubID:string,userID:string):Promise<ManagedMemberDetail>{return request(memberPath(clubID,userID))}
/** 仅负责人或平台根管理员可授撤当前组织副管理员。 */
export function setClubDeputy(clubID:string,userID:string,enabled:boolean):Promise<ManagedClubMember>{return request('/clubs/'+encodeURIComponent(clubID)+'/deputies/'+encodeURIComponent(userID),'PATCH',{enabled})}
/** 仅向当前组织真实成员发送站内消息。 */
export function sendManagedMemberMessage(clubID:string,userID:string,body:{title:string;body:string}):Promise<{recipient_count:number}>{return request(memberPath(clubID,userID)+'/messages','POST',body)}
/** 只读该成员在当前组织提供的订单。 */
export function listManagedMemberOrders(clubID:string,userID:string,query:ListQuery):Promise<ManagementPage<Order>>{return request(memberPath(clubID,userID)+'/orders'+queryString(query))}
/** 只读本组织订单收入和本组织成员托管押金。 */
export function listManagedMemberEntries(clubID:string,userID:string,query:ListQuery):Promise<ManagementPage<ScopedMemberEntry>>{return request(memberPath(clubID,userID)+'/finance/entries'+queryString(query))}
/** 只读该组织与成员关联的图文审核安全摘要。 */
export function listManagedMemberModeration(clubID:string,userID:string,query:ListQuery):Promise<ManagementPage<ModerationRecord>>{return request(memberPath(clubID,userID)+'/moderation'+queryString(query))}
