# detail 页面设计

文件作用：对应 `frontend/src/pages/community/players/detail.vue` 的先行设计图与实现说明。使用 ImageGen 独立生成并缩放到手机首屏，图中示意文案以真实业务规则为准。

- 共用背景 #0a1110、表面 #121d18、主色 #8de8b6；默认16px水平内边距，8/12/16/24px间距，12px卡片圆角。
- 顶部统一组件左上返回、居中标题和必要右侧操作，避让微信胶囊；无底部主导航，无无用英文或重复标题。
- 主按钮44px、次按钮36px，官方图标固定盒居中；输入框44px，长内容正常换行；不挤压横向布局。
- 无用户主页封面时参考 detail.png；用户已经设置 profile_cover 时参考 detail-cover.png。两状态均使用统一顶栏，封面与身份区必须使用正常文档流。
- 有封面状态：统一顶栏下先展示16:9用户封面，16px水平边距、12px圆角；封面下方保留16px正向间距后放头像昵称身份行，再依次展示统计、认证俱乐部与服务列表。禁止负margin、浮动身份层或任何内容与封面重叠。
- 图片使用本项目官方素材或用户实际上传内容，生成图中插图仅示意布局；不能在运行页面中强行显示设计图的山景或虚构认证。
- 加载、错误、空态、权限拒绝、确认弹层沿用共享组件。身份、退款、成员审批等依旧由后端验证，不能为匹配图而删业务保护。
- 页面超过一屏时自然滚动；固定业务操作区只承载当前页面主动作并预留安全区。

## 本页布局与内容指令

Use case: ui-mockup. Asset: one premium production-feasible Chinese WeChat Mini Program screen,390x844 logical portrait viewport aspect390:844. Fill entire raster flat UI only,no phone device,no perspective,no outside margins,readable Chinese. Brand 集结 game services marketplace. Unified system background#0a1110 panel#121d18 fine borders#25352c primary mint#8de8b6 white#edf4ef gray#9dafA3. No blue/purple gradients. 16px horizontal margin,8/12/16/24 spacing,12px cards,44px primary buttons,36px secondary. Official thin icons all centered. Native statusbar/capsule safe area. ONE shared compact top nav left chevron back centered page title. NO bottom main navigation, no decorative English,no slogan hero. Restrained premium utilitarian hierarchy. Title 玩家主页. Back nav plusshare icon. Compact profile row circular avatar,name 北辰20px, small 俱乐部认证 badge, bio 耐心沟通，认真履约. Three stats 已完成28 / 好评5.0 / 在售3. Single row 山海电竞 已认证 member chevron. Heading发布的服务,2-column cards each upper16:9 dark tactical cover with independently positionedmint label耐心沟通; below cover normal-flow title 大坝任务协作 / 排位陪练,认证,price¥68/小时 and完成数. Absolutely no content overlap. No decorative hero background,no follow button feature,no mainbottomnav.
