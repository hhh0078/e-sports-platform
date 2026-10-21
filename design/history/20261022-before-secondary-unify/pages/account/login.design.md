# login 页面设计

文件作用：对应 `frontend/src/pages/account/login.vue` 的先行设计图与实现说明。使用 ImageGen 独立生成并缩放到手机首屏，图中示意文案以真实业务规则为准。

- 共用背景 #0a1110、表面 #121d18、主色 #8de8b6；默认16px水平内边距，8/12/16/24px间距，12px卡片圆角。
- 顶部只使用统一组件，左上返回和居中标题，避让微信胶囊；无底部主导航，无无用英文或重复标题。
- 主按钮44px、次按钮36px，官方图标固定盒居中；输入框44px，长内容正常换行；不挤压横向布局。
- 实现内容严格参考 login.png 及下方原始设计指令；图片使用本项目官方素材，生成图中的插图不可冒充正式游戏封面。
- 加载、错误、空态和权限拒绝都沿用共享组件，保留重试及返回；登录和手机号授权必须由用户主动确认。
- 页面超过一屏时用自然滚动；固定业务底栏只保留当前页主操作，内容预留安全区域。弹层用一致的标题/关闭/内容/操作层级。

## 本页布局与内容指令

Use case: ui-mockup. Asset: one premium production-feasible Chinese WeChat Mini Program screen, 390x844 logical viewport, portrait aspect 390:844. Fill entire image with flat UI only, no phone device, no perspective, no outside margins, sharp readable Chinese. Brand 集结 gaming service marketplace. Unified system background #0a1110, panel #121d18, fine border #25352c, mint primary #8de8b6, white #edf4ef, secondary #9dafA3. No blue or purple gradients. 16px screen margin, spacing 8/12/16/24, card radius12, primary button44px, secondary36px. Official style thin icons and all icon/text centered. Native WeChat statusbar/capsule top safe area. Secondary page: one consistent compact top navigation with left chevron back and centered title; NO bottom four-tab navigation. Restrained professional clean high-end visual. Title 登录集结. Top left back, quiet skip 先逛逛 below if needed. Bold meaningful hero text 热爱游戏，自由接单, short subtext 无需挂靠，凭实力找到队友. Restrained narrow three-game artwork montage in middle taking only 140px height; no invented logo. Three small icons 自由接单, 订单留证, 俱乐部认证. Bottom strong full-width mint button 微信一键登录, below unchecked consent checkbox inline 我已阅读并同意《用户协议》和《隐私政策》. Optional profile note can be omitted. No need phone input/login password. No huge English slogan. Leave sufficient breathing room.
