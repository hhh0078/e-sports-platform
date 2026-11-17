/** 文件作用：用微信官方SDK检查隔离测试包的设置分类、返回和基础模式保存，不连接生产业务接口。 */
const automator=require('/Users/Zhuanz/Desktop/三角洲/frontend/node_modules/miniprogram-automator');
const assert=require('node:assert/strict');
const MiniProgram=require('/Users/Zhuanz/Desktop/三角洲/frontend/node_modules/miniprogram-automator/out/MiniProgram').default;
/** 当前工具Tool.getInfo只返回IDE版本，改为读取真实微信系统基础库版本再校验兼容性。 */
MiniProgram.prototype.checkVersion=async function(){const info=await this.callWxMethod('getSystemInfoSync');assert.ok(info.SDKVersion,'微信基础库未就绪');console.log('SDKVersion',info.SDKVersion)};
async function main(){
 const mini=await automator.connect({wsEndpoint:'ws://127.0.0.1:9425'});
 try{
  await mini.mockWxMethod('request',function(options){
   const path=options.url.split('/api')[1]||'';let data={};
   if(path==='/bootstrap')data={games:[{id:'delta',name:'三角洲行动',primary_color:'#8de8b6',bg_color:'#101614',surface_color:'#17221c',text_color:'#f2f6f3'}],site_name:'开机电竞',dev_mode:false};
   else if(path==='/me')data={id:'fixture-admin',role:'admin',status:'active',is_minor:false,phone_verified:true,nickname:'测试管理员'};
   else if(path==='/admin/identity-settings'){
    if(options.method==='PUT'){globalThis.__identityMode=options.data.mode;globalThis.__identitySaved={mode:options.data.mode,keyEmpty:options.data.api_key===''}};
    data={mode:globalThis.__identityMode||'strict',has_api_key:false,configured:globalThis.__identityMode==='basic',storage_available:false,api_url:'https://v2.xxapi.cn/api/idCardCheck'};
   }else if(path==='/me/identity')data={verified:true,is_minor:false,verification_mode:globalThis.__identityMode||'strict'};
   else if(path.includes('subscription'))data={templates:[],preferences:[]};
   const response={data:{data},statusCode:200,header:{}};return response;
  });
  await mini.callWxMethod('setStorageSync','jijie_token','fixture-token');
  let page=await mini.reLaunch('/pages/manage/settings/index');await page.waitFor(1800);
  let shell=await page.$('components\\/AppShell');assert.ok(shell,'缺少原生AppShell');
  await mini.screenshot({path:'/tmp/kaiji-admin-settings/output/mini-debug.png'});console.log('TEXT',await (await page.$('view')).text());let entries=await page.$$('.settings-entry');assert.equal(entries.length,9);
  await entries[4].trigger('tap');await page.waitFor(2500);page=await mini.currentPage();shell=await page.$('components\\/AppShell');entries=await page.$$('.settings-entry');console.log('PAGE',page.path,'STACK',await mini.evaluate(()=>getCurrentPages().map(p=>({route:p.route,options:p.options}))));await mini.screenshot({path:'/tmp/kaiji-admin-settings/output/mini-payment-debug.png'});assert.equal(entries.length,3);
  await mini.screenshot({path:'/tmp/kaiji-admin-settings/output/admin-mini-payments.png'});
  await mini.navigateBack();page=await mini.currentPage();await page.waitFor(2000);shell=await page.$('components\\/AppShell');assert.equal((await page.$$('.settings-entry')).length,9);
  page=await mini.navigateTo('/pages/manage/identity');await page.waitFor(2000);shell=await page.$('components\\/AppShell');const modes=await page.$$('.identity-mode');assert.equal(modes.length,2);await modes[1].trigger('tap');await page.waitFor(500);assert.equal((await page.$$('.identity-config-key')).length,0);await (await page.$('.identity-config-save')).trigger('tap');await page.waitFor(1500);
  const saved=await mini.evaluate(()=>globalThis.__identitySaved);assert.equal(saved.mode,'basic');assert.equal(saved.keyEmpty,true);
  await mini.screenshot({path:'/tmp/kaiji-admin-settings/output/admin-mini-identity.png'});
  console.log(JSON.stringify({groups:9,payments:3,backRestored:true,basicSaved:saved.mode,externalAPI:false,realDevice:false}));
 }finally{await mini.restoreWxMethod('request');await mini.callWxMethod('removeStorageSync','jijie_token');await mini.disconnect()}
}
main().catch(e=>{console.error(e);process.exitCode=1});
