# 订单支付确认 · 双渠道弹层

文件作用：对应 `frontend/src/pages/orders/detail.vue` 中支付确认弹层的先行局部变体；普通订单详情仍参考detail.png，本变体不重做正文。

- 仅买家真实待付款订单打开支付确认，图中背景状态只是暗化版式参照，不授权进行中订单再次付款。
- 弹层展示实际应付金额；微信支付/钱包余额为互斥单选，确认前不扣款。可用余额读取服务端spendable_cents，不用可提现余额代替、不发假余额。
- 充值余额仅可消费，不能提现；余额不足时该渠道不可执行或明确提示，不做部分余额+微信混合支付。
- 核心确认支付44px，取消文字入口；微信支付结果仍以服务端核验为准，钱包扣款由后端事务处理，连点/旧会话/隐藏页面不重复提交。
- 明细、聊天室与其他状态操作不变，所有真实权限和手机号门槛保留。原生状态栏与底部安全区按实际设备处理。

## 完整局部编辑提示词

Use case: ui-mockup. PreciseLOCALeditofattachedChinese390x844design. Preserveallunrelatedstyle/fields/layout/gameart/safeareas,approved#0a1110darknaturalgreenand#8de8b6mint, thinseparatorsandcentredofficialicons. No newfeatures, no giantcards, no Englishdecoration/no bluepurplegradient. AllnewtextindependentUI,nobakedgameart. Exact390:844portraitflatUI,noframe/nooutside margins. AddONLYaPAYMENTCONFIRMATIONmodalasavariant, keeporderpagebehinddarktranslucentmaskwithoutredesign. Modalreadablecentered/bottomsheetcharcoalwith12pxcornersand16pxinsets. Title确认支付 andamount¥68.00. Twoclear mutually-exclusive officialradiolinerows: 微信支付 selected; 钱包余额 unselectedwithsmall 可用余额¥0.00 (ifinsufficientdisablewithclear余额不足,donotfakefunds). Smallhelper 充值余额仅可消费，不能提现. No amountinput,paylater,bankcards,newcredits,orcombinedpartialpayment. One44pxmint 确认支付 button andquiet取消text, avoidsafebottomarea. Actualmodalonlyforpending-paymentbuyer;backgroundstatusisdimmedreference,notpermissiontopayaworkingorder. RadioselectiondoesNOTpayuntilconfirm; balanceisbackendspendableamount notimaginarygrant. Keepallotherpagevisualpartsunchanged.


## 2026-10-30 双模式支付

本轮先行布局及状态规则见 [detail-payment.commerce.design.md](detail-payment.commerce.design.md) 与同名前缀 `.commerce.png`。金币、人民币及费用分别展示；公众号手机号短信和无服务配置提示复用登录设计变体。
