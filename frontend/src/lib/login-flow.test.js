/** 文件作用：执行真实登录页脚本，验证协议门槛、微信失败提示和页面离开时的登录竞态。 */
import test from 'node:test'
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import vm from 'node:vm'
import { ref, reactive } from 'vue'
const source=readFileSync(new URL('../pages/login/index.vue',import.meta.url),'utf8').match(/<script setup>([\s\S]*?)<\/script>/)[1].replace(/^[ \t]*\/\/ #ifdef H5[\s\S]*?^[ \t]*\/\/ #endif/gm,'').replace(/^import .*$/gm,'')
/** 建立可控微信回调，直接执行生产页面代码而不复制其判断。 */
function harness(){const calls=[],notices=[],hooks={},agreements={user_agreement:{version:'u1'},privacy_policy:{version:'p1'}},state={devMode:false,user:null};let wxCallbacks;const context=vm.createContext({ref,reactive,state,sessionRevision:()=>0,token:()=>'',uni:{login(options){wxCallbacks=options;calls.push('wx.login')}},init:async()=>{},api:async()=>agreements,prepareWechatLogin:async()=>{calls.push('prepare');return{ticket:'one-time',agreements}},wechatLogin:async()=>{calls.push('login');return{id:'new-user'}},notify:message=>notices.push(message),onLoad:()=>{},onMounted:()=>{},onHide:callback=>{hooks.hide=callback},onUnload:callback=>{hooks.unload=callback}});vm.runInContext(source+'\nglobalThis.page={realLogin,loginFailed,accepted,agreements,error,submitting,profileOffer}',context);context.page.agreements.value=agreements;return{page:context.page,calls,notices,hooks,agreements,wx:()=>wxCallbacks}}

test('未勾选协议时不调用微信也不建立会话',async()=>{const subject=harness();await subject.page.realLogin();assert.deepEqual(subject.calls,[]);assert.ok(subject.notices[0].includes('勾选'))})
test('真实微信登录失败显示开发者工具登录态过期，并恢复按钮',async()=>{const subject=harness();subject.page.accepted.value=true;const pending=subject.page.realLogin();subject.wx().fail({errMsg:'login:fail INVALID_LOGIN,access_token expired [20261018 14:45:43]'});await pending;assert.match(subject.page.error.value,/开发者工具登录态已过期/);assert.equal(subject.page.submitting.value,'');assert.deepEqual(subject.calls,['wx.login']);assert.equal(subject.page.profileOffer.value,false)})
test('登录期间离开页面后迟到的票据不会建立会话或弹出资料层',async()=>{const subject=harness();subject.page.accepted.value=true;const pending=subject.page.realLogin();subject.hooks.hide();subject.wx().success({code:'temporary'});await pending;assert.equal(subject.calls.includes('login'),false);assert.equal(subject.page.profileOffer.value,false);assert.equal(subject.page.submitting.value,'')})
test('当前登录按微信票据后再建立会话的顺序显示可选资料层',async()=>{const subject=harness();subject.page.accepted.value=true;const pending=subject.page.realLogin();subject.wx().success({code:'temporary'});await pending;assert.deepEqual(subject.calls,['wx.login','prepare','login']);assert.equal(subject.page.profileOffer.value,true)})
