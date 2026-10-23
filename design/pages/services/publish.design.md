# services/publish · 本轮设计

文件作用：对应前端同路径页面的本轮先行设计与实现依据。使用内置 ImageGen 对已认可设计局部编辑并机械等比缩小至约390×844；不以验收截图替代设计图。旧图和旧说明保存在 `design/history/20261023-before-deposit-ui/`。

统一要求：自然黑绿背景、当前游戏主题、16px水平边距、真实安全区；押金徽标独立采用黑金色，不随游戏主题变化。图中示例用户、金额、订单角标均不是运行数据，运行时只取服务端。生成文字误差以本说明及真实接口为准。正文不缩小来塞首屏，长内容自然滚动。

共享服务卡最新约定：标题在上、认证/游戏/分类标签在下，均为独立封面图层，渐变底部与卡片底色一致；价格在图下紧凑正常流。押金贴右上边缘，不留外边距，黑金色、真实实心盾牌，仅左下圆角；平台短标“押金”、用户向俱乐部实缴短标“俱乐部”，金额版写明对应押金和元。0/未知/退款中不显示。卡片自定义文字至少10px，详情至少14px；六条长文字可换行并适度增高，不缩成几像素或隐藏，保留九宫格意图并避开系统信息和押金角标。

## 页面布局与状态

发布预览使用同一ServiceCard，包括标题/标签新顺序和黑金角标。自定义文字与底图始终分离；原选择游戏、文案颜色/位置/字号及审核流程保留。预览可覆盖0、1、6条文字和双押金状态；任何草稿不得伪造已缴押金。

## 生成提示词与实施边界

Use case: ui-mockup. Precise local edit of approved Chinese 集结 WeChat mini-program screenshot. Preserve natural charcoal green #0a1110, panel #121d18, mint #8de8b6, white #edf4ef, 16px insets, centered icons, native safe areas. Flat390:844portrait, no frame, no outside margin, no blue/purple gradient. Change only the page requirements described above; keep game imagery and unrelated content. Runtime text and badges stay separate UI layers. Exact Chinese text and amounts follow the page requirements; sample values are illustrative only.

验收：320/390宽度分别核对短/长名称、0/正数/双押金、六条最长中英表情文字和九宫格位置；无碰撞、无水平溢出。真实微信头像、支付/转账到账仍须单独真机与商户验收。

## 实缴预览刷新

新建和同账号返回页面时刷新getWallet，平台押金仅取deposit_cents；挂靠选择后getClubDeposits仅取membership.amount_cents，不取platform或required_cents。两类金额为只读服务端状态，不属于表单保存内容。切号/切俱乐部/隐藏的迟到响应不得恢复旧徽标；支付或押金更新后返回不重置用户草稿。预览包含价格与成交区，不能只显示封面。

## 2026-10-23后置审核

沿用现有布局，主提交文案为“发布服务”。正常发布后才进入AI或人工图文审核，无AI不阻塞发布；拒绝原因通过消息告知。原订单约定不受修改影响。

## 2026-10-24 微信订阅入口

沿用已批准布局，仅加入共享紧凑铃铛文字入口。真实授权弹层遵循 `design/pages/account/subscriptions.png` 与同名说明；不修改主交易控件，不异步自动弹微信授权。服务发布成功仅提示打手已付款新单，打手订单页可订阅退款申请；买家仅接单结果，俱乐部负责人仅成员申请/旗下争议，平台仅新俱乐部申请。消息中心增加本人微信提醒设置，平台控制台增加独立配置入口。普通聊天、钱包、调价和图文审核不发微信订阅提醒，仍保留站内消息。
