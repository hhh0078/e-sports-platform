# detail 页面设计

文件作用：对应 `frontend/src/pages/orders/detail.vue` 的先行设计图与实现说明。使用 ImageGen 独立生成并缩放到手机首屏，图中示意文案以真实业务规则为准。

- 共用背景 #0a1110、表面 #121d18、主色 #8de8b6；默认16px水平内边距，8/12/16/24px间距，12px卡片圆角。
- 顶部统一组件左上返回、居中标题和必要右侧操作，避让微信胶囊；无底部主导航，无无用英文或重复标题。
- 主按钮44px、次按钮36px，官方图标固定盒居中；输入框44px，长内容正常换行；不挤压横向布局。
- 实现内容严格参考 detail.png 和下方原始设计指令；图片使用本项目官方素材，生成图中插图仅示意布局。
- 加载、错误、空态、权限拒绝、确认弹层沿用共享组件。身份、退款、成员审批等依旧由后端验证，不能为匹配图而删业务保护。
- 页面超过一屏时自然滚动；固定业务操作区只承载当前页面主动作并预留安全区。

## 本页布局与内容指令

Use case: ui-mockup. Asset: one premium production-feasible Chinese WeChat Mini Program screen,390x844 logical portrait viewport aspect390:844. Fill entire raster flat UI only,no phone device,no perspective,no outside margins,readable Chinese. Brand 集结 game services marketplace. Unified system background#0a1110 panel#121d18 fine borders#25352c primary mint#8de8b6 white#edf4ef gray#9dafA3. No blue/purple gradients. 16px horizontal margin,8/12/16/24 spacing,12px cards,44px primary buttons,36px secondary. Official thin icons all centered. Native statusbar/capsule safe area. ONE shared compact top nav left chevron back centered page title. NO bottom main navigation, no decorative English,no slogan hero. Restrained premium utilitarian hierarchy. Title 订单详情. First quiet status heading 服务进行中 and small order number202610220018. Compact order summary card 三角洲 · 大坝任务协作 ¥68.00, 老板 云间旅人 / 打手 北辰, requirements 完成任务后提交战绩截图. Secondary outlined36 申请退款 and 主要下一步 buttons based status. Heading 订单聊天室 with small 证据留存 mint badge and caption 老板 · 打手 · 俱乐部 · 平台. Conversation card fills main remaining screen: centered gray system 订单已支付, seller bubble 北辰 已准备好，可以集合, buyer mint bubble 我们晚上8点开始，任务目标已写在订单里. Each bubble has compact role tag andtime, no overlaps. Bottom composer image icon,input 发送消息或图片证据,mint36发送. A normal bottom safe area. No big summary painting,no main tabs. Refund stage alternate adds quiet amber status box andclear agree/refuse/intervention buttons.
