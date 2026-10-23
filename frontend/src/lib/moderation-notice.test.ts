/** 文件作用：验证后置审核回执边界和零延时非阻塞提示，不把AI排队解释为审核通过。 */
import test from 'node:test'
import assert from 'node:assert/strict'
import { moderationReceipt, queueModerationNotice } from './moderation-notice'

/** 确保已调度的零延时提示执行完毕再恢复全局运行环境。 */
async function drainNotices():Promise<void>{await new Promise(resolve=>setTimeout(resolve,0))}

test('审核回执仅接受明确待审状态和正安全整数任务数',()=>{
  for(const value of [null,undefined,'ai_pending',{},[],{status:'allowed',pending_count:1},{status:'ai_processing',pending_count:1},{status:'manual_pending',pending_count:0},{status:'ai_pending',pending_count:-1},{status:'ai_pending',pending_count:1.5},{status:'ai_pending',pending_count:'1'},{status:'ai_pending',pending_count:Number.MAX_SAFE_INTEGER+1}])assert.equal(moderationReceipt(value),null)
  assert.deepEqual(moderationReceipt({status:'ai_pending',pending_count:2,estimated_seconds:999}),{status:'ai_pending',pending_count:2,estimated_seconds:20})
  assert.deepEqual(moderationReceipt({status:'manual_pending',pending_count:1}),{status:'manual_pending',pending_count:1,estimated_seconds:20})
})

test('同批连续保存合并最后有效提示，人工待审不承诺20秒且不阻塞业务',async()=>{
  const notices:UniApp.ShowToastOptions[]=[],platform=globalThis as unknown as {uni:Partial<typeof uni>},original=platform.uni
  platform.uni={showToast:(options:UniApp.ShowToastOptions)=>{notices.push(options)}} as Partial<typeof uni>
  try{queueModerationNotice({status:'ai_pending',pending_count:1},()=>true);queueModerationNotice({status:'manual_pending',pending_count:2},()=>true,true);assert.equal(notices.length,0);await drainNotices();assert.equal(notices.length,1);assert.equal(notices[0].title,'图片已上传，等待人工审核');assert.equal(notices[0].duration,3000);assert.equal(notices[0].icon,'none')}finally{await drainNotices();platform.uni=original}
})

test('轻提示能力异常不抛出，不将已保存成功的业务误报失败',async()=>{
  const platform=globalThis as unknown as {uni:Partial<typeof uni>},original=platform.uni
  platform.uni={showToast:()=>{throw new Error('原生提示不可用')}} as Partial<typeof uni>
  try{assert.doesNotThrow(()=>queueModerationNotice({status:'ai_pending',pending_count:1},()=>true));await drainNotices()}finally{await drainNotices();platform.uni=original}
})
