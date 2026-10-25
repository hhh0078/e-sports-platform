/** 文件作用：执行真实钱包页面脚本并编译其展示模板，核验合并余额文案及充值不可提现的展示边界。 */
import test from 'node:test'
import type { TestContext } from 'node:test'
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import vm from 'node:vm'
import { compile,computed,effectScope,reactive,ref,watch } from 'vue'
import type { VNode } from 'vue'
import { executable,pageScript } from './sfc-test'
import type { Wallet } from '../types/wallet'

/** 两页仅展示层的真实模板路径。 */
type BalancePage='main/me/me.vue'|'account/wallet.vue'

/**
 * 执行真实页面脚本，保留Vue响应式值并将网络、导航与生命周期隔离为无副作用边界。
 * @param t - 自动回收Vue作用域的测试上下文。
 * @param page - 实际页面路径。
 * @param balance - 模拟服务端完整或异常资金响应，不在测试中补造余额。
 * @param loggedIn - 是否已登录。
 * @returns 页面脚本公开的金额方法、实际钱包与身份状态。
 */
function pageContext(t:TestContext,page:BalancePage,balance:Partial<Wallet>|null,loggedIn=true):Record<string,unknown>{
  const state=reactive({user:loggedIn?{id:'A'}:null}),wallet=ref(balance),scope=effectScope()
  const context=vm.createContext({ref,computed,watch,state,defineProps:()=>({active:false}),defineExpose:()=>{},sessionRevision:()=>0,useMainVisibility:()=>{},useNativePageRefresh:()=>{},useWallet:()=>({wallet})})
  scope.run(()=>vm.runInContext(executable(pageScript(page))+';globalThis.display={walletAmount}',context))
  t.after(()=>scope.stop())
  if(page==='main/me/me.vue')vm.runInContext('wallet.value=globalThis.balance',Object.assign(context,{balance}))
  return {...context.display,state,wallet:balance,walletError:'',busy:false,protectedGo:()=>{}}
}

/**
 * 从真实SFC中提取钱包摘要，编译为Vue节点，避免仅用源码字符串断言掩盖模板绑定错误。
 * @param page - 实际页面路径。
 * @param context - 页面金额函数及真实响应数据。
 * @returns Vue实际生成的展示节点。
 */
function renderBalance(page:BalancePage,context:Record<string,unknown>):VNode{
  const source=readFileSync(new URL('../pages/'+page,import.meta.url),'utf8')
  const block=source.match(page==='main/me/me.vue'?/(<view class="me-wallet"[\s\S]*?)\n\t\t<view class="space-grid">/:/(<view class="balance-section">[\s\S]*?)\n    <view class="deposit-section">/)
  assert.ok(block,'缺少真实钱包摘要模板')
  const render=compile(block[1],{isCustomElement:()=>true,prefixIdentifiers:true}) as unknown as (context:Record<string,unknown>,cache:unknown[])=>VNode
  return render(context,[])
}

/** 按Vue节点收集可见文字，不将属性或事件源码当成页面内容。 */
function visibleText(node:unknown):string{if(typeof node==='string'||typeof node==='number')return String(node);if(Array.isArray(node))return node.map(visibleText).join('');return node&&typeof node==='object'&&'children' in node?visibleText(node.children):''}

/** 按真实class找到金额或说明节点，不依赖它在模板里的固定顺序。 */
function classText(node:unknown,name:string):string{if(Array.isArray(node))return node.map(item=>classText(item,name)).join('');if(!node||typeof node!=='object')return '';const vnode=node as VNode;if(String(vnode.props?.class||'').split(/\s+/).includes(name))return visibleText(vnode);return classText(vnode.children,name)}

for(const page of ['main/me/me.vue','account/wallet.vue'] as const){
  const mainClass=page==='main/me/me.vue'?'wallet-main-value':'balance-value',noteClass=page==='main/me/me.vue'?'wallet-balance-note':'balance-availability'
  test(page+'：展示账户合计及可提现子额，移除重复充值金额行',t=>{
    const data={spendable_cents:7001,available_cents:1000,recharge_cents:6001,ongoing_cents:2500,settling_cents:3000,withdrawal_frozen_cents:500}
    const rendered=renderBalance(page,pageContext(t,page,data))
    assert.equal(classText(rendered,mainClass),'¥70.01')
    assert.equal(classText(rendered,noteClass),'其中可提现 ¥10.00，充值仅用于消费')
    assert.match(visibleText(rendered),/账户余额/)
    assert.match(visibleText(rendered),/进行中¥25.00结算中¥30.00提现中¥5.00/)
    assert.equal(classText(rendered,'wallet-recharge'),'')
    assert.doesNotMatch(visibleText(rendered),/充值余额 ¥|充值余额（仅消费）/)
    assert.equal(data.available_cents,1000,'展示不能更改可提现资金分区')
  })
  test(page+'：主额只取权威spendable字段，不从缓存分区拼造',t=>{
    const rendered=renderBalance(page,pageContext(t,page,{spendable_cents:6051,available_cents:50,recharge_cents:600000}))
    assert.equal(classText(rendered,mainClass),'¥60.51')
    assert.match(classText(rendered,noteClass),/可提现 ¥0.50/)
  })
  test(page+'：登录后缺失或无效合计显示未知，不能当成零或用充值推造',async t=>{
    for(const data of [null,{available_cents:1000,recharge_cents:6000},{spendable_cents:NaN},{spendable_cents:1.5}])await t.test(String(data?.spendable_cents),child=>{
      const rendered=renderBalance(page,pageContext(child,page,data))
      assert.equal(classText(rendered,mainClass),'—')
    })
  })
  test(page+'：访客始终零值，不显示上个账号缓存',t=>{
    const rendered=renderBalance(page,pageContext(t,page,{spendable_cents:999999,available_cents:999999},false))
    assert.equal(classText(rendered,mainClass),'¥0.00')
    assert.equal(classText(rendered,noteClass),'其中可提现 ¥0.00，充值仅用于消费')
  })
}
