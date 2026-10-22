/** 文件作用：统一交易曲线的日期范围与整数分坐标计算，不从订单状态猜测成交。 */
import type { AdminTradeDay } from '../types/admin-analytics'
/** 将CSS十六进制颜色转换为微信旧Canvas支持的RGB/RGBA，防止八位透明色被忽略并刷屏告警。 */
export function legacyCanvasColor(color:string):string{
  const value=color.trim(),match=value.match(/^#([0-9a-f]{3,4}|[0-9a-f]{6}|[0-9a-f]{8})$/i)
  if(!match)return /^rgba?\([\d\s.,%]+\)$/.test(value)?value:'#8de8b6'
  const hex=match[1].length<=4?[...match[1]].map(char=>char+char).join(''):match[1]
  const channels=[0,2,4].map(index=>parseInt(hex.slice(index,index+2),16))
  return hex.length===8?`rgba(${channels.join(',')},${Number((parseInt(hex.slice(6,8),16)/255).toFixed(4))})`:`rgb(${channels.join(',')})`
}
/** 用上海自然日生成快捷统计周期，避免用户设备时区改变当天口径。 */
export function adminDateRange(days:number,end=new Date(Date.now()+8*3600000).toISOString().slice(0,10)):{start:string;end:string}{
  const last=new Date(end+'T00:00:00Z');if(!/^\d{4}-\d{2}-\d{2}$/.test(end)||!Number.isFinite(last.getTime())||last.toISOString().slice(0,10)!==end||!Number.isInteger(days)||days<1||days>366)throw new Error('统计周期无效')
  return{start:new Date(last.getTime()-(days-1)*86400000).toISOString().slice(0,10),end}
}
/** 将每天真实金额映射到有边距的画布，单日居中、全零也保留可读纵轴。 */
export function tradeChartGeometry(days:AdminTradeDay[],width:number,height:number){
  const left=48,right=Math.max(left+1,width-12),top=16,bottom=Math.max(top+1,height-28)
  const maximum=Math.max(100,...days.flatMap(day=>[day.paid_cents,day.refund_cents]).filter(Number.isFinite))
  const magnitude=10**Math.floor(Math.log10(maximum)),ceiling=Math.ceil(maximum/magnitude)*magnitude
  /** 无效或负金额仅用于防御图形越界，服务端数据仍原样在文字汇总展示。 */
  const y=(amount:number)=>bottom-Math.min(ceiling,Math.max(0,Number.isFinite(amount)?amount:0))/ceiling*(bottom-top)
  const points=days.map((day,index)=>({x:days.length===1?(left+right)/2:left+index/Math.max(1,days.length-1)*(right-left),paidY:y(day.paid_cents),refundY:y(day.refund_cents),date:day.date}))
  return{left,right,top,bottom,ceiling,points}
}
