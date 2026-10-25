/** 文件作用：验证通知行动入口的精确页面白名单、管理角色边界及编码绕过防护。 */
import test from 'node:test'
import assert from 'node:assert/strict'
import { safeMessageAction } from './message-actions'

test('无行动字段的旧消息兼容，明确站内业务路径可正常恢复',()=>{
  assert.equal(safeMessageAction({}),null)
  assert.equal(safeMessageAction(null),null)
  for(const path of ['/pages/account/profile','/pages/community/clubs/index','/pages/services/publish?id=service-A','/pages/community/clubs/edit?id=club-A','/pages/community/clubs/detail?id=club-A','/pages/community/clubs/console?id=club-A','/pages/orders/detail?id=order-A'])assert.equal(safeMessageAction({action_path:path})?.path,path)
  assert.deepEqual(safeMessageAction({action_path:'/pages/orders/detail?id=club%3Aorder',action_label:'查看相关订单'}),{path:'/pages/orders/detail?id=club%3Aorder',label:'查看相关订单'})
})

test('明确管理入口允许管理员访问，普通成员不显示管理行动',()=>{
  for(const path of ['/pages/manage/support','/pages/manage/platform','/pages/manage/games','/pages/manage/messages','/pages/manage/content/index','/pages/manage/articles/index','/pages/manage/articles/edit?id=article-A']){
    assert.equal(safeMessageAction({action_path:path},'user'),null)
    assert.equal(safeMessageAction({action_path:path},'clubowner'),null)
    assert.equal(safeMessageAction({action_path:path},'admin')?.path,path)
  }
  assert.equal(safeMessageAction({action_path:'/pages/manage/settings'},'admin'),null)
})

test('外域、任意站内页、附加查询及多重编码均不能成为行动入口',()=>{
  for(const path of ['javascript:alert(1)','https://example.invalid','//example.invalid','__proto__','constructor','/pages/account/profile?next=https://example.invalid','/pages/account/wallet','/pages/manage/moderation','/pages/services/publish?id=foo&next=bar','/pages/services/publish?id=foo#bar','/pages/services/publish?id=foo%0abar','/pages/services/publish?id=%2Fetc','/pages/services/publish?id=%252fetc','/pages/services/publish?id=%E0%A4%A','/pages/services/publish?id=','/pages/services/publish?id=foo+bar',' /pages/account/profile','/pages\\account\\profile'])assert.equal(safeMessageAction({action_path:path},'admin'),null,path)
})
