/** 文件作用：执行真实配置组件，验证分区保存、秘密清理、回包隔离及独立支付方式保留。 */
import test from 'node:test'
import assert from 'node:assert/strict'
import vm from 'node:vm'
import {readFileSync} from 'node:fs'
import {ref,reactive,computed} from 'vue'
import {executable} from './sfc-test'
import type {CommerceSettings,CommerceSettingsInput} from '../types/commerce'

/** 加载真实组件脚本，HTTP仅返回不含真实秘密的管理员夹具。 */
function fixture(section:'frontend'|'methods'|'virtual'|'login'){
 const stored:CommerceSettings={mode:'mini_program',payment_method:'wechat',fee_bearer:'buyer',android_fee_bps:100,ios_fee_bps:1200,offer_id:'12345',virtual_env:0,official_app_id:'wx1234567890abcdef',official_redirect_url:'https://example.com/',app_key_set:true,sandbox_app_key_set:false,official_secret_set:true,storage_available:true}
 const state=reactive({user:{id:'admin-A',role:'admin'},commerce:{...stored}}),writes:CommerceSettingsInput[]=[],hooks={hide:()=>{}}
 let finish:((value:CommerceSettings)=>void)|null=null
 const context=vm.createContext({ref,reactive,computed,state,defineProps:()=>({section}),onMounted:()=>{},onBeforeUnmount:()=>{},onShow:()=>{},onHide:(cb:()=>void)=>{hooks.hide=cb},onUnload:()=>{},token:()=>state.user.id,sessionRevision:()=>0,errorMessage:(e:Error)=>e.message,notify:()=>{},getCommerceSettings:async()=>({...stored}),updateCommerceSettings:(body:CommerceSettingsInput)=>{writes.push({...body});return new Promise<CommerceSettings>(resolve=>{finish=resolve})}})
 const script=readFileSync(new URL('../components/CommerceConfig.vue',import.meta.url),'utf8').match(/<script setup lang="ts">([\s\S]*?)<\/script>/)![1]
 vm.runInContext(executable(script)+'\nglobalThis.subject={load,save,draft,secrets,error,loaded}',context)
 return{subject:context.subject,state,writes,hooks,stored,finish:()=>{assert.ok(finish);finish({...stored})}}
}

for(const section of ['frontend','methods','virtual','login'] as const)test(section+'只保存当前分区，秘密成功后清空',async()=>{
 const f=fixture(section);await f.subject.load();f.subject.secrets.app_key='new-test-key-only';f.subject.secrets.official_secret='new-official-test-key'
 const task=f.subject.save()
 assert.equal(f.writes.length,1)
 const keys=Object.keys(f.writes[0]).sort()
 assert.deepEqual(keys,({frontend:['frontend_mode'],methods:['payment_method'],virtual:['android_fee_bps','app_key','fee_bearer','ios_fee_bps','offer_id','sandbox_app_key','virtual_env'],login:['official_app_id','official_redirect_url','official_secret']}[section]).sort())
 f.finish();await task
 assert.equal(f.subject.secrets.app_key,'');assert.equal(f.subject.secrets.official_secret,'');assert.equal(f.state.commerce.payment_method,'wechat')
})

test('切号和隐藏之后的配置回包不能覆盖新账号，秘密输入立即清空',async()=>{
 const f=fixture('virtual');await f.subject.load();f.subject.secrets.app_key='only-test-secret';const task=f.subject.save()
 f.hooks.hide();f.state.user={id:'admin-B',role:'admin'};f.state.commerce.payment_method='virtual';f.finish();await task
 assert.equal(f.subject.secrets.app_key,'');assert.equal(f.state.commerce.payment_method,'virtual')
})

test('当前身份不匹配已加载配置时禁止提交旧账号草稿',async()=>{
 const f=fixture('methods');await f.subject.load();f.state.user={id:'admin-B',role:'admin'};await f.subject.save();assert.equal(f.writes.length,0)
})
