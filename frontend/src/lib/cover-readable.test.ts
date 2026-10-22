/** 文件作用：验证卡片和详情真实像素文案可读、完整、防碰撞及系统区域避让。 */
import test from 'node:test'
import assert from 'node:assert/strict'
import {layoutReadableCover} from './cover-readable'
import {POSITIONS} from './cover'
test('320/390下六条长文案保持最小字号并避开角标标题，必要时增高换行',()=>{
  for(const width of [136,171,288,320,358,390])for(const [position] of POSITIONS){
    const minFont=width<200?10:14,top=width<200?44:96,bottom=width<200?52:188
    const result=layoutReadableCover(Array.from({length:6},(_,id)=>({id,text:id%2?'最长文案十二字一起开心玩':'WWWWWWWWWWWW',position,font_size:24})),width,width/(4/3),bottom,top,minFont)
    assert.equal(result.labels.length,6)
    for(const label of result.labels){assert.ok(label.font_size*width/360>=minFont-.001);assert.ok(label.top/100*result.height>=top+7.99);assert.ok((label.top+label.height)/100*result.height<=result.height-bottom-7.99);assert.ok(label.left>=0&&label.left+label.width<=100.01)}
    for(let i=0;i<result.labels.length;i++)for(let j=i+1;j<result.labels.length;j++){const a=result.labels[i],b=result.labels[j];assert.ok(a.x+a.w+3.99<=b.x||b.x+b.w+3.99<=a.x||a.y+a.h+3.99<=b.y||b.y+b.h+3.99<=a.y)}
  }
})
test('简单文案保留位置与原文，空图保持比例且不修改输入',()=>{
  const labels=[{text:'耐心沟通',position:'top-right',font_size:16}],original=JSON.stringify(labels)
  const result=layoutReadableCover(labels,171,128,52,22)
  assert.equal(result.labels.length,1);assert.equal(result.labels[0].text,'耐心沟通');assert.equal(result.labels[0].position,'top-right');assert.equal(JSON.stringify(labels),original)
  assert.equal(layoutReadableCover([],171,128).height,128)
})
