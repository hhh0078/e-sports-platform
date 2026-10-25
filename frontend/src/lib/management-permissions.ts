/** 文件作用：集中管理平台根/副管理员、组织管理者及原负责人收款的显示权限；最终鉴权由服务端执行。 */
import type { Club,User } from '../types/models'
import type { WalletWithdrawal } from '../types/wallet'
/** 兼容历史管理员为根身份，新副管理员不能继续授权。 */
export function isPlatformRoot(user:User|null|undefined):boolean{return user?.role==='admin'&&user.platform_admin_role!=='deputy'}
/** 管理组织只接受服务端明确管理权或管理角色，不将普通挂靠当作管理。 */
export function isClubManager(value:{can_manage?:boolean;management_role?:string}|null|undefined):boolean{return typeof value?.can_manage==='boolean'?value.can_manage:value?.management_role==='owner'||value?.management_role==='deputy'}
/** 只有组织当前负责人且原提现受益人一致时才能拉起微信确认收款。 */
export function canConfirmClubTransfer(user:Pick<User,'id'>|null|undefined,club:Pick<Club,'owner_id'>|null|undefined,record?:Pick<WalletWithdrawal,'user_id'>):boolean{return !!user?.id&&user.id===club?.owner_id&&(!record||record.user_id===user.id)}
/** 展示真实平台角色，不能从昵称或组织挂靠猜管理员身份。 */
export function platformRoleName(user:Pick<User,'role'|'platform_admin_role'>):string{return user.role==='admin'?(user.platform_admin_role==='deputy'?'平台副管理员':'平台管理员'):'玩家 / 打手'}
