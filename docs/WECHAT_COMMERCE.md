# 微信双模式与金币支付

文件作用：说明小程序虚拟支付、公众号普通支付的配置、资金边界和联调验收方法。本文不保存账号密钥或真实交易数据。

## 配置入口

平台管理员在“平台设置”按职责维护：平台基础 → 前端模式；账号与身份 → 微信登录；支付与结算 → 支付方式、微信支付、虚拟支付、商家转账。每个表单只保存本分区字段。

| 配置 | 行为 |
| --- | --- |
| 小程序模式 | 采用小程序 code 登录；支付方式单独选择 |
| 公众号模式 | 微信浏览器 OAuth 登录；当前仅支持微信支付 |
| 微信支付 | 普通微信 JSAPI 付款及人民币钱包，原单绑定原AppID和商户 |
| 虚拟支付 | 小程序金币充值使用 `short_series_coin`，服务消费使用官方扣币接口 |
| 手续费承担方 | 默认平台承担；买家承担时，在确认付款前展示服务金额、额外费用及总金币 |
| 安卓 / iOS 手续费 | 分别配置，单位百分比，最多两位小数，范围为 0%（含）至 100%（不含）；订单固定创建时快照 |
| 金币兑换 | 固定一元一金币；业务商品不需要逐个发布为道具，金币配置本身必须在微信发布 |

旧配置没有独立 `payment_method` 时继续按原模式解释（小程序为虚拟、公众号为微信）；通过新表单保存前端入口后，支付选择被固定并独立维护。旧管理端不得联动覆盖已经拆分的模式；已发起订单、补款及退款继续按原支付事实恢复。

手续费是平台业务配置，不代表微信官方费率。正式费率、类目和活动折扣以该账号当前协议及账单为准。

服务端使用既有 `CONFIG_ENCRYPTION_KEY` 加密整份配置。读取只返回秘密是否存在；秘密输入留空保留已保存值，客户端保存成功或离开后清空。配置未就绪时关闭对应真实能力，不自动切换到另一支付方式。

## 小程序参数

在微信已开通的虚拟支付后台取得 OfferID、正式 AppKey 和沙箱 AppKey，填入平台配置。微信登录仍使用原 `WECHAT_APP_ID` 和 `WECHAT_APP_SECRET`。也可由服务器部署环境提供初始值：

- `WECHAT_COMMERCE_MODE`：`mini_program` 或 `official_account`，默认前者。
- `WECHAT_VIRTUAL_OFFER_ID`、`WECHAT_VIRTUAL_APP_KEY`、`WECHAT_VIRTUAL_SANDBOX_APP_KEY`。
- `WECHAT_VIRTUAL_ENV`：正式 `0`、沙箱 `1`，默认正式。

`session_key` 仅留在服务端短期 Redis 中；付款前用新的微信 code 刷新同一账号的会话，不能用另一个 OpenID 替换付款人。

消息推送接收地址为 `/api/wechat/virtual-payment/callback`（GET校验及POST通知），也可使用已接入同一分流的原小程序消息回调。服务器配置 `WECHAT_MESSAGE_TOKEN`；启用安全模式时配置 `WECHAT_MESSAGE_AES_KEY`，并在微信后台保持Token、EncodingAESKey、XML/JSON格式与服务器一致。没有消息推送Token时不开放新的正式金币付款，旧单查询与恢复仍按原配置处理。签名成功才接收业务通知，现金退款按稳定事件号及单调累计金额去重。

Apple退款问询仅在能够定位已核验充值、其金币确实对应买家验收完成且无进行中售后的服务时提供履约证据建议；证据不足不伪造拒绝理由，最终决定由Apple作出。最终退款通知与问询分开处理，已流转款项冻结来源并保留核账审计。

本版不假设官方支持小数金币。服务金额、含费总金币及需购买金币必须可用整金币精确表示；不符合时明确拒绝，不向上取整、不修改原商品价。例如68元订单由平台承担手续费时可按68金币购买，买家承担1%会产生0.68金币额外费用，当前不能直接付款。后续只有在官方明确单位和真机验证后才可开放小数金币，不能只删除校验。

## 公众号参数与账户

配置独立的公众号 AppID、Secret 和固定 HTTPS 网页授权回跳 URL。回跳 URL 不含 `#`，例如 `https://你的H5域名/`；H5 启动时清除一次性 code / state 并进入登录页。微信后台还需配置网页授权域名、JS接口安全域名、支付授权目录和商户 AppID 绑定。

可选服务器初始字段：`WECHAT_OFFICIAL_APP_ID`、`WECHAT_OFFICIAL_APP_SECRET`、`WECHAT_OFFICIAL_REDIRECT_URL`。

首次单独启用公众号管理账号时，可由私密运维配置 `BOOTSTRAP_ADMIN_OFFICIAL_APP_ID` 与 `BOOTSTRAP_ADMIN_OFFICIAL_OPENID` 精确指定本人官方身份。仅在微信服务端票据匹配时执行一次；持久标记防止降权后再次自动授予。小程序原管理员初始化字段不被公众号复用，客户端不能指定角色。

小程序与公众号分别保存 AppID / OpenID。只有微信服务端返回且唯一的 UnionID 才关联同一内部账号；昵称、客户端 OpenID、未验证手机号不能用于合并资产。尚未绑定同一微信开放平台或旧账号未取得 UnionID 时，公众号可能建立独立账号，原账号资产不会迁移或覆盖。两个已有账号发生冲突时返回明确错误，保留原资产。

公众号是独立使用入口。小程序内不提供去公众号、H5 或其它外部页面完成同一笔虚拟商品付款的导流。

H5 的普通支付使用微信内置桥；提现确认使用服务端同源 URL 签名、官方 JS-SDK 能力检测及 `requestMerchantTransfer`。页面拉起或用户确认不代表资金到账，最终状态仍由商户查单或验签通知确认。

## 手机号短信

公众号网页授权不提供小程序的 `getPhoneNumber` 能力。本版提供腾讯云短信适配，需在服务器配置：

- `TENCENT_SMS_SECRET_ID`、`TENCENT_SMS_SECRET_KEY`。
- `TENCENT_SMS_REGION`，默认 `ap-guangzhou`。
- `TENCENT_SMS_SDK_APP_ID`、`TENCENT_SMS_SIGN_NAME`、`TENCENT_SMS_TEMPLATE_ID`。

短信模板使用一个六位验证码变量。验证码五分钟有效、一次消费，限制账号、号码及来源地址发送频率，五次错误后失效。尚未配置服务时明确提示，不能以模拟验证码满足正式交易门槛。既有经过微信核实的手机号仍有效。

## 资金规则

金币与人民币充值、服务收入、押金独立分账。客户端支付成功只触发原单同步；服务端核对官方现金充值单后才入金币，再用独立稳定业务号执行官方扣币。超时或结果不明保留原单恢复，不生成第二笔消费。

服务订单退款使用官方退币操作，恢复原金币用途；人民币历史订单继续按原支付来源退款。金币充值现金退款是另一业务：微信退款任务受理不等于退款完成，必须核验官方结果。iOS 的现金退款由 Apple 处理，不把退金币当作已退现金。

押金是独立人民币保证资金，不作为购买服务的金币充值入口；退款恢复原押金记录。正式启用前应按微信当前审核口径验证押金适用范围。

后台切换模式不修改旧单的渠道、付款人、AppID、金额及费用快照。普通商户配置仍须保留旧交易所需资质；当前及归档公众号 AppID 仅用于原交易恢复，不作为任意 AppID 签名入口。

## 验收

代码验收使用本地 PostgreSQL / Redis 的独立临时 schema、官方 API 替身及 H5 / 微信构建；替身成功不代表真实商户已获授权。

正式联调分别核验安卓、iOS、公众号：真实登录与账号关联、支付取消、支付后断网恢复、重复通知、订单退款退币、现金退款结果、手续费与实际渠道一致、切换模式后旧单恢复，以及服务收入提现。iOS 需要实际支持的微信客户端、Apple 账户和已开通能力，其真实支付不能由沙箱结果代替。

## 官方依据

- [微信虚拟支付接入](https://developers.weixin.qq.com/miniprogram/dev/platform-capabilities/business-capabilities/virtual-payment.html)
- [虚拟支付客户端接口](https://developers.weixin.qq.com/miniprogram/dev/api/payment/wx.requestVirtualPayment.html)
- [微信团队虚拟支付运营指南](https://developers.weixin.qq.com/community/minihome/doc/00002cf077cd4810fee42f4b865c01)
- [腾讯 CloudBase 虚拟支付说明](https://github.com/TencentCloudBase/skills/blob/main/skills/cloudbase-wechat-integration/references/virtual-payment.md)
- [腾讯 CloudBase 维护者对自建回调与退款终态的说明](https://github.com/TencentCloudBase/CloudBase-AI-Toolkit/issues/1132)
- [普通商户 JSAPI 用户确认收款](https://pay.wechatpay.cn/doc/v3/merchant/4012716430)
- [腾讯云短信 TC3 签名](https://cloud.tencent.com/document/product/382/55981)
- [腾讯云发送短信接口](https://cloud.tencent.com/document/api/382/52072)

## 2026-11-17 充值平台字段兼容

`POST /wallet/topups` 接收可选 `platform: android|ios`，未传按旧协议处理。此前前端已发送该字段，后端严格JSON请求结构遗漏，导致创建充值前返回400“请求内容格式不正确”。本次仅补齐明确字段及枚举校验，保留未知字段拒绝、原金额、用途、通道与幂等记账；同一请求切换平台不能另建充值或重复入账。

隔离数据库先复现用户载荷的400，再执行完整 `TEST_INTEGRATION=true go test -race ./... -count=1` 全部通过（internal/app139.583秒）；覆盖Android、iOS、旧载荷、幂等重试、非法平台和未知字段。未执行真实充值或微信扣款。

### 2026-11-17 现场发布与消息配置状态

充值字段修复3689df8已部署，并包含于后续钱包新版9458c60；生产容器健康，开发模式关闭。用户后续请求进入虚拟付款配置门槛，返回“虚拟支付消息验签尚未配置”。只读检查确认容器及现有部署环境未设置WECHAT_MESSAGE_TOKEN，已知旧部署也无可沿用项；安全模式另需匹配WECHAT_MESSAGE_AES_KEY。保持验签保护，未伪造Token、复用AppKey或发起真实扣款，待管理员确认微信后台消息推送配置后接通。

### 消息回调服务端配置完成

管理员已提供消息配置并明确选择明文模式、JSON。服务器私密环境已配置消息Token，AESKey保持未启用；当前API原镜像重载后健康。公网地址 `https://game.ai.58it.cn/api/wechat/virtual-payment/callback` 的有效签名GET返回正确echostr，错误签名JSON通知返回403。Token/AES值不进入文档、代码或日志。私密原配置保留于服务器受限目录以便恢复。

微信后台仍由管理员保存URL、原Token、明文模式和JSON格式。回调状态已就绪，但虚拟支付商业配置公开状态仍不完整，应继续检查OfferID及当前环境AppKey；未执行真实付款或官方实际通知验收。


## 现金原单状态传播与退款

官方查单的0/1/3/6状态不生成金币，保留原付款单等待同步；已付2/4状态但凭据未传播齐全也不提前入账。只有原单、面额、实付、类型、交易号和时间全部核对通过才入账，重复同步保持幂等。已退款5/8独立处理，充值未入账前已退款时记录现金退还，不能因迟到成功快照再次补币。真正金额、类型、原单与时间冲突有独立错误。日志只记录数值、状态和匹配标志，不记录OpenID或密钥。

本轮用户原单只读查单返回原价100分、实付100分、类型0、退款状态5；没有保留退款前官方响应，因此不能将原故障断言为手续费或某个特定前置状态。当前处理以可信官方查询为准，没有手工补币、改账或另发起退款。
