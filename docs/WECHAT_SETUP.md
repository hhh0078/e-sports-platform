# 微信登录与支付接入指南

文件作用：记录真实微信所需配置、上线前联调步骤和本地测试的边界，不包含任何真实密钥。2026-10-18。

## 当前可用方式

本地使用 `DEV_MODE=true`、游客 AppID 和五个体验身份，能够完整走订单、支付状态、退款和四方沟通。微信适配代码已经提供官方 `code2session`、常规小程序 JSAPI 支付、RSA 签名与验签、AES-GCM 通知解密、付款查单、全额退款、退款查询、退款原单号重试。

自动测试使用一次性测试密钥和受控通道验证协议与异常恢复，不会真实扣款。当前尚无正式 AppID、商户资质和域名，所以没有冒充真实登录、商户联调或审核上线通过。

## 准备资料与配置位置

由小程序与商户负责人在微信官方后台准备资料，填入部署环境或服务器配置文件；不要放在小程序包里，也不要粘贴进聊天。

| 配置 | 来源与含义 |
| --- | --- |
| `WECHAT_APP_ID` | 正式微信小程序 AppID；同时替换 `frontend/src/manifest.json` 中 `mp-weixin.appid` |
| `WECHAT_APP_SECRET` | 小程序 AppSecret，仅服务端使用 |
| `WECHAT_PAY_MCH_ID` | 已开通小程序支付并与 AppID 关联的商户号 |
| `WECHAT_PAY_SERIAL` | 商户 API 证书序列号，供请求签名标识使用 |
| `WECHAT_PAY_PRIVATE_KEY_PATH` | 服务端商户 RSA 私钥文件绝对路径，至少 2048 位，PKCS#8 或 PKCS#1 PEM |
| `WECHAT_PAY_PUBLIC_KEY_ID` | 微信支付公钥 ID，对应通知和应答的 `Wechatpay-Serial`；不能误填成商户证书序列号 |
| `WECHAT_PAY_PUBLIC_KEY_PATH` | 微信支付验签公钥 PEM 文件的绝对路径；必须与上述公钥 ID 对应 |
| `WECHAT_PAY_API_V3_KEY` | 商户 APIv3 密钥，32 字节，用于解密通知资源 |
| `WECHAT_PAY_NOTIFY_URL` | `https://你的API域名/api/payments/wechat/notify` |
| `WECHAT_PAY_REFUND_NOTIFY_URL` | `https://你的API域名/api/payments/wechat/refund-notify` |
| `VITE_API_BASE` | 前端 `.env.local` 中配置 `https://你的API域名/api`，修改后重新构建 |

示例见 `backend/.env.example`。正式部署使用 `APP_ENV=production` 和 `DEV_MODE=false`；系统会拒绝生产环境打开开发模式。启动脚本只面向本地开发，不能当作生产进程管理器。

## 域名、部署与首位管理员

1. 使用独立生产 PostgreSQL 和 Redis，创建本项目专用数据库与账号；不要把本地演示库直接作为真实交易库。建立备份，图片证据也在 PostgreSQL 内。
2. Go 服务放在 HTTPS 反向代理之后，开放两个微信通知接口。小程序后台配置合法 `request`、`uploadFile` 和 `downloadFile` 域名；证据图片与 API 使用相同域名时也应完整配置这些域名项。
3. 服务器配置微信密钥与正式 AppID；前端替换 AppID、API 地址并重新构建。正式版本启用域名校验，不保留游客调试设置。
4. 首位运营人员在正式小程序微信登录一次，服务器会创建 `wx_...` 用户。管理员身份只由可信运维在数据库中授予，前端不能自报角色。先核对该人的用户 ID，再通过数据库管理工具将这一条用户记录的 `data.role` 改为 `admin`。只更改确认的单条记录，保留操作记录；生产种子不会自动建立可随意登录的测试管理员。
5. 在小程序内用管理员账号完成服务类目、俱乐部、主题和公告检查。按微信平台实际要求准备主体、服务类目、隐私说明与审核材料。

## 真实渠道验收顺序

使用商户允许的低金额测试订单，先在允许的测试范围内联调，再开放真实用户。

- 微信登录：正常 code 兑换、重复/过期 code、退出与会话失效、被冻结用户拒绝访问。
- 支付：服务器金额与下单快照一致；客户端支付成功之后，以验签回调或查单为准；用户取消拉起支付不会显示已付款。
- 并发与中断：同时发起预支付和取消；付款通知重复/延迟；服务重启后查单恢复；错误商户、OpenID、金额、签名和过期时间均拒绝。
- 退款：接单前取消、卖家同意退款、平台裁决退款；请求超时后仍冻结交易；按原退款单号查询或重试，确认微信成功才显示退款完成。
- 异常退款：`ABNORMAL`、`CLOSED` 或网络未知结果不能由买家撤回后直接结算。管理员在商户侧核对资金状态后同步；本系统不会将不明资金结果自动当作成功。
- 真机：切到移动网络测试下单、图片上传、聊天历史、通知返回和断网重试。分别使用玩家、打手、俱乐部管理员与平台管理员账号核对权限。

接口只做全额退款。当前的收入字段是订单聚合统计，不是商户向打手实际出款；上线前还需按商户资质确定资金结算、分账或提现方式，并单独联调，不应把本地收入数字当作完成付款给打手。

## 失败定位

缺少正式资料时服务端明确返回配置错误，不会静默切换成测试付款。检查 `.run/api.log` 或生产服务日志，不要把密钥、APIv3 Key、私钥内容写入错误报告。重点核对 AppID/商户绑定、公钥 ID 与公钥配对、服务器时钟、回调 HTTPS 可达性、数据库和 Redis 状态。

官方入口：[微信公众平台](https://mp.weixin.qq.com/)、[微信支付商户平台](https://pay.weixin.qq.com/)、[微信支付开发文档](https://pay.wechatpay.cn/doc/v3/)。采用常规 `/v3/pay/transactions/jsapi`；不能因文档出现灰度中的新能力，就假定当前商户已经有权限。

## 复现微信模拟器 UI 验收

已安装官方 `miniprogram-automator` 为前端开发依赖。构建后在微信开发者工具中开启自动化端口：

```sh
/Applications/wechatwebdevtools.app/Contents/MacOS/cli auto --project "$PWD/frontend/dist/build/mp-weixin" --auto-port 9420 --trust-project
node scripts/mini-e2e.cjs
```

若游客工具出现 AppID 提示，使用游客模式打开项目；构建过程中 `dist` 会先重建，不要把短暂的 app.json 缺失当作最终产物缺失，构建完成后重新编译即可。脚本只允许本地 DEV_MODE=true，实际点击五个身份，创建测试单并完成付款、接单、交付、四方聊天、退款拒绝、平台介入、裁决退款与无关人拒绝，截图和结果位于 `output/playwright/mini-*`。它会保留新测试订单与证据，不改旧订单。
