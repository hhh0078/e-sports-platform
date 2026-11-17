/** 文件作用：集中定义管理员业务入口与九类设置，分类只能跳转到明确登记的内部页面。 */
/** 具体业务或配置入口。 */
export interface AdminEntry { label:string;icon:string;route:string;description?:string }
/** 设置分类；单项直接打开，多项显示二级列表。 */
export interface AdminSettingsGroup {id:string;label:string;icon:string;description:string;entries:AdminEntry[]}
export const ADMIN_BUSINESS_ENTRIES:AdminEntry[]=[
  {label:'用户管理',icon:'user',route:'/pages/manage/users'},
  {label:'服务审核',icon:'receipt',route:'/pages/manage/services'},
  {label:'俱乐部管理',icon:'club',route:'/pages/manage/clubs'},
  {label:'订单与售后',icon:'orders',route:'/pages/manage/orders'},
  {label:'提现申请',icon:'wallet',route:'/pages/manage/wallet'},
  {label:'资金来源审核',icon:'shield',route:'/pages/manage/finance/index'},
]
export const ADMIN_CONTENT_ENTRIES:AdminEntry[]=[
  {label:'公共信息',icon:'bell',route:'/pages/manage/content/index'},
  {label:'文章管理',icon:'receipt',route:'/pages/manage/articles/index'},
  {label:'图文审核',icon:'shield',route:'/pages/manage/moderation'},
  {label:'发送平台消息',icon:'bell',route:'/pages/manage/messages'},
]
export const ADMIN_SETTINGS_GROUPS:AdminSettingsGroup[]=[
  {id:'platform',label:'平台基础',icon:'settings',description:'平台名称与基础资料',entries:[{label:'平台资料',icon:'settings',route:'/pages/manage/platform'}]},
  {id:'games',label:'游戏配置',icon:'game',description:'游戏目录、分类与主题',entries:[{label:'游戏管理',icon:'game',route:'/pages/manage/games'}]},
  {id:'identity',label:'账号与身份',icon:'shield',description:'身份校验、操作审计',entries:[{label:'身份校验',icon:'shield',route:'/pages/manage/identity',description:'严格API核验或宽松基础校验'},{label:'操作审计',icon:'receipt',route:'/pages/manage/audit',description:'查看管理员操作记录'}]},
  {id:'agreements',label:'协议管理',icon:'receipt',description:'用户协议、隐私政策',entries:[{label:'协议管理',icon:'receipt',route:'/pages/manage/settings?tab=agreements'}]},
  {id:'payments',label:'支付与结算',icon:'weixin',description:'支付模式、微信支付、商家转账',entries:[{label:'支付模式',icon:'settings',route:'/pages/manage/settings?tab=commerce',description:'小程序与公众号的支付方式'},{label:'微信支付',icon:'weixin',route:'/pages/manage/settings?tab=payment',description:'商户信息、密钥与回调'},{label:'商家转账',icon:'wallet',route:'/pages/manage/wallet?view=transfer',description:'提现通道、场景与报备字段'}]},
  {id:'finance',label:'费率与押金',icon:'wallet',description:'平台抽成与两类押金门槛',entries:[{label:'交易规则',icon:'wallet',route:'/pages/manage/finance/settings'}]},
  {id:'review',label:'内容与审核',icon:'shield',description:'AI审核服务与规则',entries:[{label:'AI审核配置',icon:'settings',route:'/pages/manage/ai'}]},
  {id:'notifications',label:'消息与订阅',icon:'bell',description:'微信订阅模板与提醒',entries:[{label:'微信订阅',icon:'bell',route:'/pages/manage/subscriptions'}]},
  {id:'support',label:'客服支持',icon:'headphones',description:'二维码、电话与微信号',entries:[{label:'客服设置',icon:'headphones',route:'/pages/manage/support'}]},
]
