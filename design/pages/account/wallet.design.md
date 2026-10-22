# 我的钱包 · 余额与自愿平台押金

文件作用：对应 `frontend/src/pages/account/wallet.vue` 的先行设计及用户最新局部要求；2026-10-22生成/编辑。已有页面上一版归档在 `design/history/20261022-before-game-refinements/pages/`，钱包为本次新增，无旧版。

- 新增钱包先行图；统一顶部返回“我的钱包”，无主导航底栏，采用开放分节、细分隔、16px边距、24px节距，不额外叠加游戏插画或信用卡风格大卡。
- 最突出可提现余额，旁为待结算，均显示服务端真实数据；图中0.00仅表示合法空账户示例，不能当默认赠款或伪造余额。未登录/读取中显示“登录后查看”/占位，不把未知数当真实余额。
- 充值/提现为紧凑36px入口；金额操作弹层或同页表单沿用底线金额输入44px、单个44px确认和文字取消。金额采用整数分校验，提现不得超过真实可提现额，零余额禁用或清楚提示。
- 平台押金独立区明确“自愿”，展示实际持有金额和自愿缴纳/按条件申请退还轻入口；接单不需要缴纳平台押金，不与俱乐部规则押金混同，不显示无来源授信或收益承诺。
- 有实际押金且没有进行中/售后订单时才提供可执行退还；有相关履约/争议时说明暂不可退原因，不能仅靠前端隐藏通过条件。无押金不显示可执行的假退还操作。
- 交易明细仅展示真实服务端记录及充值/提现/押金的真实状态、金额、时间和关联信息；可按类型过滤，暂无记录用轻量空态。处理中的单据不在客户端提前记作到账或成功。
- 微信支付/提现能力配置不足时，针对本次操作显示明确错误（例如未配置微信支付），不模拟扣款或到账，不把客户端确认当支付回调；余额/历史仍可查看。支付已提交、取消、处理中、成功、失败等以服务端及实际支付结果为准。
- 不新增银行卡输入、外部收款账户、假信用额度或默认充值数据；秘密与支付凭据不进入图、日志或持久客户端缓存。提现具体渠道与费用以真实后端能力为准，不承诺即时到账。

## 生成与验收边界

390×844，使用内置ImageGen。既有页面仅局部编辑，钱包新建；输出仅机械等比缩放。生成示例文字/数字/局部比例以真实业务、本说明和系统安全区为准，不能作为真实资金凭证。实际SDK与交易验收由root执行，本图不代表支付/真机通过。

## 完整生成或编辑提示词

Use case: ui-mockup. Create one NEW Chinese WeChat mini-program wallet UI design, exact390:844portrait logicalviewport, flatUIonly,no deviceframe,no outside margins. Match集结 approvednatural darkeditorialstyle: background#0a1110,white#edf4ef,mutedgraygreen#9dafA3,mint#8de8b6,16pxhorizontalinsets,24pxsectiongaps,thin separators. OPEN sections,NO repeatedlargecards,no neon/bluepurplegradients,no decorativeillustration,no fakecredit/grantedfunds/bankcardinputs. Ordinarysharedtopbackheader 我的钱包 withnativeWeChatstatus/capsulesafearea,NO primarybottomtabnav. Topmostprominentbalance section: label可提现余额, large¥0.00 (zeroillustrativeemptyaccount; runtimealwaysrealserverbalance), besideorbelowquiet待结算¥0.00. Compact36px充值 and提现 entries, NOTtwohugefullwidthbuttons. Underthinseparatorindependent平台押金 section withsmall 自愿 badge, currentamount¥0.00, smallmint 自愿缴纳→link. Quiethelper 接单无需缴纳平台押金 and 无进行中或售后订单时可申请退还. DoNOTimplydepositismandatoryorclubdeposit. Atzeroheldamountdonotshowactivefakerefundbutton; paidstatecanexpose申请退还lightentry. Next交易明细 headingwithboundedcompactfilters 全部 充值 提现 押金; emptybodyquiet 暂无资金记录,no fakeledger/dates/money. Operations/formsnotshownmainstate:recharge/withdraw/depositopensame-stylecompactamountsheetwithtransparentunderlined44pxamountinputandone44pxconfirm,取消quiettext; aftersubmissiondisplayrealpending/success/failurestatus,neverfakepaymentdone. MissingWeChatconfigerrorstateisinlineclear 暂未配置微信支付，当前无法发起此操作，资金与明细仍可查看, notfakeauthorizecheckout. Keepreadablecalmprofessionalfinancialhierarchy, statusbymodelnotinvented. No unnecessaryslogan, no loan/credit/bankdetails, zerofiguresareempty placeholdersnotrealbalanceclaims.
