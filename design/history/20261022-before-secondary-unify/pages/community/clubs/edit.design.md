# edit 页面设计

文件作用：对应 `frontend/src/pages/community/clubs/edit.vue` 的先行设计图与实现说明。使用 ImageGen 独立生成并缩放到手机首屏，图中示意文案以真实业务规则为准。

- 共用背景 #0a1110、表面 #121d18、主色 #8de8b6；默认16px水平内边距，8/12/16/24px间距，12px卡片圆角。
- 顶部统一组件左上返回、居中标题和必要右侧操作，避让微信胶囊；无底部主导航，无无用英文或重复标题。
- 主按钮44px、次按钮36px，官方图标固定盒居中；输入框44px，长内容正常换行；不挤压横向布局。
- 实现内容严格参考 edit.png 和下方原始设计指令；图片使用本项目官方素材，生成图中插图仅示意布局。
- 加载、错误、空态、权限拒绝、确认弹层沿用共享组件。身份、退款、成员审批等依旧由后端验证，不能为匹配图而删业务保护。
- 页面超过一屏时自然滚动；固定业务操作区只承载当前页面主动作并预留安全区。

## 本页布局与内容指令

Use case: ui-mockup. Asset: one premium production-feasible Chinese WeChat Mini Program screen,390x844 logical portrait viewport aspect390:844. Fill entire raster flat UI only,no phone device,no perspective,no outside margins,readable Chinese. Brand 集结 game services marketplace. Unified system background#0a1110 panel#121d18 fine borders#25352c primary mint#8de8b6 white#edf4ef gray#9dafA3. No blue/purple gradients. 16px horizontal margin,8/12/16/24 spacing,12px cards,44px primary buttons,36px secondary. Official thin icons all centered. Native statusbar/capsule safe area. ONE shared compact top nav left chevron back centered page title. NO bottom main navigation, no decorative English,no slogan hero. Restrained premium utilitarian hierarchy. Title 俱乐部装修. Compact nav back plusquiet 预览. Section封面图片,16:9 tactical cover crop area withcamera icon andsecondary更换封面 button overlay only aseditorcontrol, hint 16:9 · 自动裁剪压缩. Form grouped 俱乐部名称 山海电竞, 介绍 textarea 欢迎认真履约的游戏爱好者, 游戏范围 three selected chips 三角洲 王者荣耀 和平精英. Section加入规则: 押金规则（元） input200.00, 考核要求 textarea 需提交近期战绩，负责人审核. Next 图文详情 blockheader with 添加文字 / 添加图片 secondary controls. Bottom fixed44保存装修 mint. No decoration banner, no bottomnav.
