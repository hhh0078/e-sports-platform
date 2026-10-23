/** 文件作用：验证朋友圈单页的统一返回与跳转不会调用微信禁止的页面导航能力。 */
import test from 'node:test'
import assert from 'node:assert/strict'
import {navigate,back} from './navigation'
test('1154公共单页返回和内部跳转只说明打开完整小程序，不发起导航',()=>{const runtime=globalThis as unknown as {wx?:unknown;uni?:unknown},oldWX=runtime.wx,oldUni=runtime.uni;const calls:string[]=[];runtime.wx={getEnterOptionsSync:()=>({scene:1154})};runtime.uni={showToast:()=>calls.push('hint'),navigateTo:()=>calls.push('navigate'),navigateBack:()=>calls.push('back'),reLaunch:()=>calls.push('relaunch')};try{navigate('/pages/account/login');back();assert.deepEqual(calls,['hint','hint'])}finally{runtime.wx=oldWX;runtime.uni=oldUni}})
