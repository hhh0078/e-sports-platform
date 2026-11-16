/** 文件作用：校验服务表单并传递本次预览快照；持久化仍由账号隔离的发布草稿负责。 */
import type {ServicePublishDraft} from './publish-draft'
import type {Service} from '../types/models'

/** 一次已校验预览的归属、独立数据及精确草稿清理回调，不包含会话令牌。 */
export interface ServicePreview {
  owner:string
  revision:number
  serviceID:string
  fields:ServicePublishDraft
  service:Service
  valid:()=>boolean
  complete:()=>void
}
let preview:ServicePreview|null=null

/** 校验完整表单；编辑页进入预览及预览页提交共用相同规则。 */
export function serviceFormIssue(form:ServicePublishDraft):string {
  if(!form.title.trim()||!form.description.trim()||!form.category||!form.cover_url)return '请完整填写标题、说明、类型和封面'
  const price=Math.round(Number(form.price)*100)
  if(!/^\d+(\.\d{1,2})?$/.test(form.price)||price<100||price>1000000)return '价格请填写1至10000元，最多两位小数'
  if(form.labels.some(label=>!label.text.trim()||!/^#[0-9a-f]{6}$/i.test(label.color||'')||!(label.background==='transparent'||/^#[0-9a-f]{6}$/i.test(label.background||''))))return '请填写文案，并使用正确的六位颜色值'
  return ''
}
/** 建立独立快照，返回编辑后继续输入不会悄悄改变已打开预览的内容。 */
export function setServicePreview(value:ServicePreview):void {
  preview={...value,fields:JSON.parse(JSON.stringify(value.fields)) as ServicePublishDraft,service:JSON.parse(JSON.stringify(value.service)) as Service}
}
/** 仅原账号、会话代次和仍存活的编辑上下文可以读取当前预览。 */
export function getServicePreview(owner:string,revision:number):ServicePreview|null {
  return preview?.owner===owner&&preview.revision===revision&&preview.valid()?preview:null
}
/** 只清理指定快照，旧页面的迟到响应不能作废新预览。 */
export function clearServicePreview(value:ServicePreview):void {if(preview===value)preview=null}
