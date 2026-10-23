/** 文件作用：统一微信订阅的模板去重、直接手势调用、结果解析及管理员字段校验，拒绝不影响业务。 */
import type { SubscriptionAuthorization,SubscriptionAuthorizationStatus,SubscriptionOverview,SubscriptionScene,SubscriptionSettingsBody,SubscriptionFieldSource } from '../types/subscriptions'
/** 原生接口边界仅保留本次订阅需要的方法。 */
export type SubscribeNative=(options:{tmplIds:string[];success:(result:Record<string,unknown>)=>void;fail:(error:unknown)=>void})=>unknown
/** 用户可理解的固定消息场景。 */
export const SUBSCRIPTION_LABELS:Record<SubscriptionScene,string>={provider_order:'打手新订单',provider_refund:'打手退款申请',buyer_accepted:'买家接单结果',club_member:'俱乐部成员申请',club_dispute:'俱乐部争议协助',admin_club:'平台俱乐部申请'}
/** 模板来源只能映射这些安全摘要，不能上传任意模板内容。 */
export const SUBSCRIPTION_SOURCES:Record<SubscriptionFieldSource,string>={title:'消息标题',status:'处理状态',amount:'金额',time:'事件时间',order_no:'订单编号',target_name:'对象名称',count:'待办数量',summary:'简短说明',service_name:'服务名称',actor_name:'相关玩家昵称',club_name:'俱乐部名称',created_time:'下单／申请时间'}
/** 把可用且用户允许的场景按模板去重，单次最多申请三个不同模板。 */
export function subscriptionCandidates(overview:SubscriptionOverview|null,scenes:SubscriptionScene[]){if(!overview?.enabled)return[];const templates=new Set<string>();return overview.scenes.filter(item=>{if(!scenes.includes(item.scene)||!item.enabled||!item.available||item.preference===false||overview.preferences[item.scene]===false||!item.template_id)return false;if(!templates.has(item.template_id)&&templates.size>=3)return false;templates.add(item.template_id);return true})}
/** 仅编译到微信端的原生入口；H5不调用不支持的授权接口。 */
export function wechatSubscribeAPI():SubscribeNative|null{let native:SubscribeNative|null=null
  // #ifdef MP-WEIXIN
  if(typeof uni.requestSubscribeMessage==='function')native=options=>uni.requestSubscribeMessage({tmplIds:options.tmplIds,success:result=>options.success(Object.fromEntries(Object.entries(result))),fail:options.fail})
  // #endif
  return native
}
/** 生成不含个人身份的本次请求号，不能复用为永久授权。 */
export function subscriptionRequestID():string{return 'sub_'+Date.now().toString(36)+'_'+Math.random().toString(36).slice(2,14)}
/** 将未知原生结果严格转为固定枚举，不把缺失结果当作同意。 */
export function subscriptionResult(value:unknown):SubscriptionAuthorizationStatus{return value==='accept'||value==='reject'||value==='ban'||value==='filter'?value:'fail'}
/**
 * 在调用者点击栈内立即发起官方授权，函数内部首次调用原生方法前没有 await。
 * @param overview - 页面预先读取的可用模板。
 * @param scenes - 用户此次明确选择的场景。
 * @param native - 当前微信运行环境接口。
 * @returns 真实原生返回，拒绝也正常结束，不阻断任何业务。
 */
export function requestSubscriptionFromGesture(overview:SubscriptionOverview|null,scenes:SubscriptionScene[],native:SubscribeNative|null):Promise<SubscriptionAuthorization>{const candidates=subscriptionCandidates(overview,scenes);if(!native)return Promise.reject(new Error('请在微信小程序内设置提醒'));if(!candidates.length)return Promise.reject(new Error('当前暂无可订阅模板，可稍后到消息中心设置'));const requestID=subscriptionRequestID(),tmplIds=[...new Set(candidates.map(item=>item.template_id))];return new Promise(resolve=>{const finish=(result:Record<string,unknown>)=>resolve({request_id:requestID,results:candidates.map(item=>({scene:item.scene,template_id:item.template_id,status:subscriptionResult(result[item.template_id])}))});try{native({tmplIds,success:finish,fail:()=>finish({})})}catch{finish({})}})}
/** 检查模板草稿的固定来源、字段名、重复字段及启用前置条件。 */
export function validateSubscriptionSettings(body:SubscriptionSettingsBody):void{for(const scene of body.scenes){const id=scene.template_id.trim(),entries=Object.entries(scene.fields);if(id&&!/^[A-Za-z0-9_-]{10,128}$/.test(id))throw new Error(SUBSCRIPTION_LABELS[scene.scene]+'的模板ID格式不正确');if(entries.length>10)throw new Error('每个模板最多配置10个字段');for(const [key,source]of entries){if(!/^(thing|phrase|character_string|amount|number|time|date)\d{1,3}$/.test(key))throw new Error('模板字段名格式不正确，例如thing1、time2');if(!Object.hasOwn(SUBSCRIPTION_SOURCES,source))throw new Error('请选择平台允许的字段来源');if(key.startsWith('amount')&&source!=='amount'||key.startsWith('number')&&source!=='count'||/^(time|date)/.test(key)&&!['time','created_time'].includes(source))throw new Error('金额、数量和时间字段须选择对应来源')}if(scene.enabled&&(!id||!entries.length)||!scene.enabled&&(!!id!==!!entries.length))throw new Error('请完整配置'+SUBSCRIPTION_LABELS[scene.scene]+'的模板ID和字段映射，未配置场景请保持关闭')}}
/** 按官方字段类型生成安全初值，文字字段仍需管理员核对具体语义。 */
export function subscriptionSourceForField(key:string):SubscriptionFieldSource{return key.startsWith('amount')?'amount':key.startsWith('number')?'count':/^(time|date)/.test(key)?'time':'title'}
/** 固定场景只可选择含义匹配的一次性官方模板，服务端仍再次验证。 */
export function subscriptionTemplateAllowed(scene:SubscriptionScene,title:string):boolean{const titles:Record<SubscriptionScene,string[]>={provider_order:['交易提醒','新订单通知','订单付款通知'],provider_refund:['退款申请通知','退款申请提醒'],buyer_accepted:['接单成功提醒','接单成功通知','订单接单通知'],club_member:['新成员邀请通知','新成员申请通知','入会申请通知'],club_dispute:['订单异常通知','订单争议通知'],admin_club:['商家入驻申请通知','俱乐部申请通知','新俱乐部申请通知']};return titles[scene].includes(title.trim())}
/** 模板仅可包含本功能支持的正整数编号字段，电话等类型与空模板不进入选择列表。 */
export function subscriptionTemplateFieldsSupported(fields:ReadonlyArray<{key:string}>):boolean{return fields.length>0&&fields.length<=10&&fields.every(field=>/^(thing|phrase|character_string|amount|number|time|date)[1-9]\d{0,2}$/.test(field.key))}
