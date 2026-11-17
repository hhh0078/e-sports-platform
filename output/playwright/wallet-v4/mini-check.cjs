/** 文件作用：使用官方SDK检查四页钱包布局和规则，所有API均为隔离夹具，不连接生产支付。 */
const automator=require('/Users/Zhuanz/Desktop/三角洲/frontend/node_modules/miniprogram-automator');
const MiniProgram=require('/Users/Zhuanz/Desktop/三角洲/frontend/node_modules/miniprogram-automator/out/MiniProgram').default;
const assert=require('node:assert/strict');
/** 当前工具只返回IDE版本，改为读取真实微信基础库版本确认SDK可用。 */
MiniProgram.prototype.checkVersion=async function(){const info=await this.callWxMethod('getSystemInfoSync');assert.ok(info.SDKVersion);console.log('SDK',info.SDKVersion)};
/** 通过原生入口打开规则，并核验正文存在。 */
async function checkGuide(page,must){const guide=await page.$('components\\/FinanceGuide');assert.ok(guide,'规则组件未挂载');await (await guide.$('.finance-guide-link')).trigger('tap');await page.waitFor(800);const sections=await guide.$$('.finance-guide-section');assert.ok(sections.length>=2,'规则段落缺失');const text=(await Promise.all(sections.map(row=>row.text()))).join('');assert.ok(text.includes(must),'完整规则缺失');return guide}
/** 冷启动与页面跳转都只使用SDK公开操作。 */
async function main(){const mini=await automator.connect({wsEndpoint:'ws://127.0.0.1:9426'});try{
 await mini.mockWxMethod('request',function(options){
  const p=(options.url.split('/api')[1]||'').split('?')[0];let data=[];
  const wallet={virtual_coin_cents:0,spendable_cents:200,recharge_cents:200,available_cents:0,pending_cents:0,ongoing_cents:0,settling_cents:0,withdrawal_frozen_cents:0,deposit_cents:0,deposit_refunding_cents:0,deposit_required_cents:3000,deposit_shortfall_cents:3000,deposit_qualified:false,settlement_hold_days:7,currency:'CNY',dev_mode:false};
  const deposit={amount_cents:0,refunding_cents:0,required_cents:3000,shortfall_cents:3000,qualified:false};
  if(p==='/bootstrap')data={games:[{id:'delta',name:'三角洲行动',primary_color:'#e6bc6b',bg_color:'#15110c',surface_color:'#242017',text_color:'#f4f3ed',categories:[],covers:[],enabled:true}],site_name:'开机电竞',dev_mode:false,commerce:{mode:'mini_program'}};
  else if(p==='/me')data={id:'fixture-admin',role:'admin',nickname:'界面验收',status:'active',phone_verified:true,is_minor:false};
  else if(p==='/me/identity')data={verified:true,is_minor:false,verification_mode:'strict'};
  else if(p==='/wallet'||p.endsWith('/wallet'))data=wallet;
  else if(p==='/clubs/fixture-club')data={club:{id:'fixture-club',name:'先锋俱乐部',owner_id:'fixture-admin',deposit_cents:3000,game_ids:['delta'],status:'active'},can_manage:true,members:[],services:[],my_membership:null};
  else if(p.endsWith('/deposits'))data={platform:{...deposit,can_manage:true},membership:deposit};
  else if(p==='/admin/financial-settings')data={free_platform_rate_bps:500,club_platform_rate_bps:300,personal_platform_deposit_cents:3000,club_creation_deposit_cents:3000};
  else if(p.includes('subscription'))data={templates:[],preferences:[]};
  return{data:{data},statusCode:200,header:{}};
 });
 await mini.callWxMethod('setStorageSync','jijie_token','offline-wallet-fixture');
 let page=await mini.reLaunch('/pages/account/wallet');await page.waitFor(2500);
 const card=await page.$('.coin-card');assert.ok(card);const size=await card.size();assert.ok(size.width>250&&size.width<450);assert.ok((await card.text()).includes('0.00'));
 await mini.screenshot({path:'/tmp/kaiji-wallet-visual/output/playwright/wallet-v4/mini-wallet.png'});
 await checkGuide(page,'Apple');await mini.screenshot({path:'/tmp/kaiji-wallet-visual/output/playwright/wallet-v4/mini-wallet-rules.png'});
 for(const spec of [{route:'/pages/community/clubs/funds?id=fixture-club',must:'存在认证成员',name:'funds'},{route:'/pages/community/clubs/deposit?id=fixture-club',must:'自行扣罚',name:'member'},{route:'/pages/manage/finance/settings',must:'旧订单保留',name:'settings'}]){
  page=await mini.reLaunch(spec.route);await page.waitFor(2500);await mini.screenshot({path:'/tmp/kaiji-wallet-visual/output/playwright/wallet-v4/mini-'+spec.name+'.png'});await checkGuide(page,spec.must);
 }
 console.log(JSON.stringify({walletCard:true,rules:4,provider:'mock',externalPayment:false,realDevice:false}));
}finally{await mini.restoreWxMethod('request');await mini.callWxMethod('removeStorageSync','jijie_token');await mini.disconnect()}}
main().catch(error=>{console.error(error);process.exitCode=1});
