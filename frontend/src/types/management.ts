/** 文件作用：声明平台用户及俱乐部成员管理的安全摘要、权限、分页与资金范围；不包含微信私密身份。 */
import type { User,ClubMember } from './models'
import type { WalletEntry } from './wallet'
/** 所有管理关联列表使用服务端范围过滤后的分页元信息。 */
export interface ManagementPage<T>{items:T[];total:number;limit:number;offset:number;has_more:boolean}
/** 前端仅按服务端允许的操作显示按钮，不自行授予权限。 */
export interface ManagementPermissions{can_edit:boolean;can_manage_admins:boolean;can_send_message:boolean}
/** 平台用户基本资料与当前操作者权限。 */
export interface ManagedUserDetail{user:User;permissions:ManagementPermissions}
/** 审核结果安全摘要，不携带交易证据、原图或模型密钥。 */
export interface ModerationRecord{id:string;target_type:string;target_id:string;field_label:string;kind:string;status:string;reason:string;decision_source:string;attempts:number;created_at:string;updated_at:string}
/** 组织当前管理角色，负责人不能由副管理员修改。 */
export type ClubManagementRole=''|'owner'|'deputy'
/** 有权管理者读取的组织成员摘要。 */
export interface ManagedClubMember extends ClubMember{avatar:string;phone_masked:string;management_role:ClubManagementRole}
/** 仅该俱乐部累计结算及托管押金，不是个人可提现钱包。 */
export interface ScopedMemberFinance{club_id:string;user_id:string;settled_income_cents:number;ongoing_cents:number;settling_cents:number;club_deposit_cents:number;club_deposit_refunding_cents:number}
/** 成员资金明细限定为本组织订单收入或托管押金。 */
export interface ScopedMemberEntry extends WalletEntry{scope:'club_order_income'|'club_member_deposit';order_id?:string}
/** 成员详情及组织内权限，不能据此编辑成员全局资料。 */
export interface ManagedMemberDetail{user:Pick<User,'id'|'nickname'|'avatar'>;membership:ClubMember;management_role:ClubManagementRole;permissions:ManagementPermissions;scope_finance:ScopedMemberFinance}
