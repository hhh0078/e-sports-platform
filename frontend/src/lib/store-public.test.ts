/** 文件作用：执行真实store初始化，验证朋友圈公共浏览不读取私有身份且不会阻断正常登录恢复。 */
import test from 'node:test'
import assert from 'node:assert/strict'
import vm from 'node:vm'
import { readFileSync } from 'node:fs'
import { reactive,ref,computed } from 'vue'
import { executable } from './sfc-test'
/** 构建带可控网络边界的真实共享store，不替换业务初始化逻辑。 */
function harness(){let boot=0,me=0;const storage=new Map([['jijie_token','token-A']]),context=vm.createContext({reactive,ref,computed,uni:{getStorageSync:(key:string)=>storage.get(key),removeStorageSync:(key:string)=>storage.delete(key)},token:()=>storage.get('jijie_token')||'',errorStatus:(e:{status?:number})=>e.status,getBootstrap:async()=>{boot++;return{games:[],dev_mode:false}},getMe:async()=>{me++;return{id:'A',nickname:'玩家'}}});vm.runInContext(executable(readFileSync(new URL('./store.ts',import.meta.url),'utf8'))+'\nglobalThis.subject={init,initPublic,state}',context);return{subject:context.subject,storage,counts:()=>({boot,me}),context}}
test('朋友圈公共初始化即使本机有令牌也不读取本人身份',async()=>{const h=harness();await Promise.all([h.subject.init(false,true),h.subject.initPublic()]);assert.deepEqual(h.counts(),{boot:1,me:0});assert.equal(h.subject.state.ready,true);assert.equal(h.subject.state.user,null);assert.equal(h.storage.get('jijie_token'),'token-A')})
test('公共配置就绪后正常初始化仍恢复原会话，不把ready误当身份已恢复',async()=>{const h=harness();await h.subject.initPublic();await Promise.all([h.subject.init(),h.subject.init()]);assert.deepEqual(h.counts(),{boot:1,me:1});assert.equal(h.subject.state.user.id,'A');await h.subject.init();assert.equal(h.counts().me,1)})
test('公共刷新不会因私有本人接口不可用而失败或清除原令牌',async()=>{const h=harness();h.context.getMe=async()=>{throw Object.assign(new Error('私有接口失效'),{status:401})};await h.subject.init(false,true);assert.equal(h.storage.get('jijie_token'),'token-A');await h.subject.init();assert.equal(h.storage.has('jijie_token'),false);assert.equal(h.subject.state.user,null)})
