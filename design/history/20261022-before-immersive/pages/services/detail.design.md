# detail 页面设计

文件作用：对应 `frontend/src/pages/services/detail.vue` 的先行设计图与实现说明。使用 ImageGen 独立生成并缩放到手机首屏，图中示意文案以真实业务规则为准。

- 共用背景 #0a1110、表面 #121d18、主色 #8de8b6；默认16px水平内边距，8/12/16/24px间距，12px卡片圆角。
- 顶部只使用统一组件，左上返回和居中标题，避让微信胶囊；无底部主导航，无无用英文或重复标题。
- 主按钮44px、次按钮36px，官方图标固定盒居中；输入框44px，长内容正常换行；不挤压横向布局。
- 实现内容严格参考 detail.png 及下方原始设计指令；图片使用本项目官方素材，生成图中的插图不可冒充正式游戏封面。
- 加载、错误、空态和权限拒绝都沿用共享组件，保留重试及返回；登录和手机号授权必须由用户主动确认。
- 页面超过一屏时用自然滚动；固定业务底栏只保留当前页主操作，内容预留安全区域。弹层用一致的标题/关闭/内容/操作层级。

## 本页布局与内容指令

Use case: ui-mockup. Asset: one premium production-feasible Chinese WeChat Mini Program screen, 390x844 logical viewport, portrait aspect 390:844. Fill entire image with flat UI only, no phone device, no perspective, no outside margins, sharp readable Chinese. Brand 集结 gaming service marketplace. Unified system background #0a1110, panel #121d18, fine border #25352c, mint primary #8de8b6, white #edf4ef, secondary #9dafA3. No blue or purple gradients. 16px screen margin, spacing 8/12/16/24, card radius12, primary button44px, secondary36px. Official style thin icons and all icon/text centered. Native WeChat statusbar/capsule top safe area. Secondary page: one consistent compact top navigation with left chevron back and centered title; NO bottom four-tab navigation. Restrained professional clean high-end visual. Title 服务详情. Unified header left back. First 16:9 tactical game cover screenshot with independent two mint/black caption tags 位于左下 耐心沟通 稳定撤离 (image represents source artwork only). Below cover, separated normal-flow title 三角洲 · 大坝任务协作 and club certification 山海电竞认证, large mint price ¥68 /小时. Seller panel circular avatar 北辰 已完成28单 查看主页 chevron. 服务说明 section concise paragraphs, three tag pills 新手友好 / 可预约 / 全程沟通. 下单信息 form 数量 stepper 1, 预约与要求 textarea. Fixed primary bottom action row ¥68 合计 plus mint44 立即下单. No image/body text overlap. No repeated headings or slogans.
