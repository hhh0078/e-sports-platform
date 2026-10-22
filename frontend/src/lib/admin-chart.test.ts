/** 文件作用：验证自定义周期跨月闰年、单日与全零曲线和极端金额不越界。 */
import test from 'node:test'
import assert from 'node:assert/strict'
import { adminDateRange,tradeChartGeometry,legacyCanvasColor } from './admin-chart'
/** 构造每日真实类型的坐标测试记录。 */
function day(date:string,paid:number,refund=0){return{date,paid_cents:paid,refund_cents:refund,paid_orders:paid?1:0,refunded_orders:refund?1:0}}
test('管理员快捷周期包含末日并正确跨过闰日',()=>{assert.deepEqual(adminDateRange(7,'2024-03-01'),{start:'2024-02-24',end:'2024-03-01'});assert.deepEqual(adminDateRange(1,'2026-10-22'),{start:'2026-10-22',end:'2026-10-22'});for(const days of [0,367,1.5])assert.throws(()=>adminDateRange(days))})
test('全零和单日曲线可读且保持画布边界',()=>{const chart=tradeChartGeometry([day('2026-10-22',0)],288,196);assert.equal(chart.points[0].x,(chart.left+chart.right)/2);assert.equal(chart.points[0].paidY,chart.bottom);assert.ok(chart.ceiling>0)})
test('交易退款共用纵轴，巨额及异常输入不越界',()=>{const chart=tradeChartGeometry([day('2026-10-21',999999999,10),day('2026-10-22',-1,Number.NaN)],288,196);for(const p of chart.points){assert.ok(p.x>=chart.left&&p.x<=chart.right);assert.ok(p.paidY>=chart.top&&p.paidY<=chart.bottom);assert.ok(p.refundY>=chart.top&&p.refundY<=chart.bottom)}assert.equal(chart.points[1].paidY,chart.bottom)})

test('旧微信Canvas颜色边界将八位与四位透明色转为rgba，六位主题色保持rgb',()=>{assert.equal(legacyCanvasColor('#ffffff12'),'rgba(255,255,255,0.0706)');assert.equal(legacyCanvasColor('#0000'),'rgba(0,0,0,0)');assert.equal(legacyCanvasColor('#00e59b'),'rgb(0,229,155)');assert.equal(legacyCanvasColor('#abc'),'rgb(170,187,204)');assert.equal(legacyCanvasColor('rgba(255,255,255,0.2)'),'rgba(255,255,255,0.2)');assert.equal(legacyCanvasColor('invalid'),'#8de8b6')})
