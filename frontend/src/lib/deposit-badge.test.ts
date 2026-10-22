/** 文件作用：执行真实押金标识脚本，验证服务端有效金额、退款后隐藏以及列表/详情文案的区别。 */
import test from 'node:test'
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import vm from 'node:vm'
import { computed, reactive } from 'vue'
import { executable } from './sfc-test'
import { money } from './format'

/** Props使用真实Vue响应式，仅替代编译期宏。 */
function harness(){
  const source=readFileSync(new URL('../components/DepositBadge.vue',import.meta.url),'utf8').match(/<script setup lang="ts">([\s\S]*?)<\/script>/)?.[1]
  assert.ok(source)
  const props=reactive<{amountCents:number|undefined;showAmount:boolean;kind:'user_platform'|'user_club'|'club_platform'}>({amountCents:undefined,showAmount:false,kind:'user_platform'})
  const context=vm.createContext({computed,money,defineProps:()=>props,withDefaults:(value:object)=>value})
  vm.runInContext(executable(source)+'\nglobalThis.subject={hasDeposit,badgeText}',context)
  return{props,subject:context.subject}
}

test('押金标识只接受正整数分，未知、零、退款后金额及异常金额不显示',()=>{
  const subject=harness()
  for(const value of [undefined,0,-1,NaN,Infinity,1.5,Number.MAX_SAFE_INTEGER+1]){
    subject.props.amountCents=value
    assert.equal(subject.subject.hasDeposit.value,false)
  }
  subject.props.amountCents=20050
  assert.equal(subject.subject.hasDeposit.value,true)
  subject.props.amountCents=0
  assert.equal(subject.subject.hasDeposit.value,false)
})

test('列表使用短标识，详情按真实整数分金额格式展示且没有赔付承诺',()=>{
  const subject=harness();subject.props.amountCents=20050
  assert.equal(subject.subject.badgeText.value,'押金')
  subject.props.showAmount=true
  assert.equal(subject.subject.badgeText.value,'平台押金200.5元')
  subject.props.amountCents=10000
  assert.equal(subject.subject.badgeText.value,'平台押金100元')
  subject.props.kind='user_club'
  assert.equal(subject.subject.badgeText.value,'俱乐部押金100元')
  subject.props.showAmount=false
  assert.equal(subject.subject.badgeText.value,'俱乐部')
  subject.props.kind='club_platform'
  subject.props.showAmount=true
  assert.equal(subject.subject.badgeText.value,'平台押金100元')
})
