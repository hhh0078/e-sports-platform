# 俱乐部沉浸主页新版设计

文件作用：对应 `frontend/src/pages/community/clubs/detail.vue` 的先行设计图与实施约束；2026-10-22 根据用户最新要求编辑。旧版保存在 `design/history/20261022-before-immersive/pages/`。

- 展示型二级页面取消整条应用顶栏和“俱乐部主页”重复标题。用户封面从顶部沉浸开始，16:9等比裁切展示；原生状态栏/胶囊安全区保留。
- 左上使用36px圆形细边返回按钮，浮在封面上。分享入口避开胶囊，可放下方或正文，不能与系统控件重叠；无主导航底栏。
- 封面下16px正向间距后正常流展示俱乐部Logo、名称、认证、主理人与统计，禁止头像负margin/身份侵入封面。
- 支持游戏单行受控展示。门槛条区分考核、押金规则及平台不代收押金说明；申请加入/分享维持真实权限与流程，控件44/36px。
- 详细信息/成员列表切换，正文为管理员上传的图文，成员为已认证打手。说明文字自然排版，减少多层套卡；不虚构认证或自动代收押金。
- 上传封面和文章图片仅使用用户实际资产；生成图仅示意构图。

## 生成与实施边界

使用内置ImageGen编辑同路径旧图后等比缩放至390×844。图片用于可读首屏设计，所有界面需实现为真实组件；生成文字误差以本说明和真实业务为准。图中省略的微信胶囊仍必须按系统API真实避让。实际验收截图单独放output，不得替换设计图。

## 完整编辑指令

Use case: ui-mockup. Edit the attached 390x844 Chinese WeChat mini-program design. Output one flat 390:844 portrait UI, no device frame, no outside margins. Preserve brand 集结, dark charcoal-green #0a1110, white #edf4ef, muted graygreen #9dafA3, mint accent #8de8b6, fine restrained dividers, 16px horizontal insets and legible Chinese. Native statusbar and top-right WeChat capsule safe area are real constraints. Avoid generic AI dashboard appearance: no repeated big rounded boxes, no dashed placeholders, no decorative giant buttons, no blue/purple gradients. Official icons aligned centrally. Any game artwork is design-reference only. Primary tabs have only ONE persistent bottom nav 玩家 打手 订单 我的. This is an image-led SECONDARY club profile, so NO primary bottom tabnav. Remove top navbar entirely including centered 俱乐部主页. A fullwidth16:9 scenic tactical community COVER begins at y0 behind native status and capsule, no border/margins and heightabout220px. Overlay a circular36px subtle fineborder backbutton at x16 y52 on cover; official share control below capsule if needed but no fullbar. After cover leave16px NORMAL POSITIVE flow gap, then clubidentity with circular64 logo 山海, 山海电竞 and small平台认证, smallhostline. No negative margin, identity must not overlap cover. Stats24认证成员 3覆盖游戏 in same compactrow. Onegame chiprow 三角洲 王者荣耀 和平精英 constrainedwidth. Joining conditionrow with shield, 加入门槛, 需考核·押金规则¥200, tiny 平台不代收押金 explanatorylink. Main44申请加入 and secondary36分享主页 aligned and tasteful, not oversized. Detail/membertabs in normalflow, 詳细信息 active underline. The article 关于我们 uses simple readable richtext and a landscape uploadedimage; avoid giant redundant panels. Cover-focused immersive rendering with no header title, back clear and capsule avoidance. Maintain existing content and polishedspacing.
