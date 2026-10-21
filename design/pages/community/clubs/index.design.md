# 俱乐部目录 · 开放式列表

文件作用：对应 `frontend/src/pages/community/clubs/index.vue` 的先行新版设计与实施依据；2026-10-22按二级页面统一任务编辑。旧图/说明已归档 `design/history/20261022-before-secondary-unify/pages/`。

- 去掉包裹每个俱乐部的完整圆角panel，仅保留图片自身圆角；封面、身份、简介与游戏/状态在正常流，项间细分隔和24px节距。
- 搜索使用44px底线输入，发现俱乐部/我的俱乐部用轻量下划线切换，右侧创建文字入口；保持源码命名，不照搬生成图中的全部俱乐部差异。无新增筛选。
- 封面16:9、下方名称/认证/成员数/主理人、最多两行简介、受控游戏标签与真实状态。图片只是构图示意，使用俱乐部实际封面或游戏封面；禁止复制假数据或无来源认证。
- 普通统一顶部返回，无主底栏。访客可浏览，创建/我的按原权限处理；创建弹窗沿用底线input/低对比textarea与多选游戏，不改业务。
- 空态用少量文字与轻入口即可，不必再复用每页同一插画。搜索、切换账号和筛选后的旧响应不能覆盖新列表。

## 统一实施约束

内置ImageGen编辑旧图，机械等比缩为390×844。采用自然黑绿/游戏主题色、16px水平边距、24px段落间距、细分隔，输入控件44px、textarea灰底8px，主提交44px、取消文字。图标固定盒双轴居中。生成文字和局部比例误差以真实源码字段、本说明及系统API安全区为准。设计图不是页面背景，实际验收截图另存output。

## 完整编辑提示词

Use case: ui-mockup. Edit the attached existing Chinese390x844 mini-program design. Approved style: open editorial layout on#0a1110, white#edf4ef, mutedgraygreen#9dafA3, mint#8de8b6, thin separators and24pxsectiongap,16pxhorizontalmargin. Remove large enclosing roundedpanels and nestedcards. Textinput: transparent background and singlebottomhairline,44pxhigh; textarea: subtlegray-greenfilledsurface,8pxradius, noouterformcard. Core submit44px, secondary36px/quiettext, officialiconscentered. NO newfeatures,no Englishdecoration,no genericheadline,no giantbuttons,no neon/blue/purplegradients. Keepordinarysharedback/titleheader, reserve nativestatusbar+WeChatcapsule; NO primarybottomnav. Exact390:844portraitflatUI, no deviceframe,noperspective,no outside margins. Data/examplesareillustrativebutfieldstructuremustmatchrealbusiness. Title俱乐部. Preserveactualsearch,发现俱乐部/我的俱乐部tabs andsmall创建textentry. Searchunderlinedrow, tabs compactwithmintunderline, create atfar right oftabrow safelybelowcapsule. DoNOTusebigfilledsegmentblock. Clubdiscoverylist is open editorialstack, NOT borderedsamecards:16:9coverimage358x201with8pxcorners; BELOW image in normalflow smallmonogram+bold山海电竞+verifiedicon, muted24位成员·山海主理人主理, two-lineconciseintro. Footer smallgamechips三角洲王者荣耀和平精英 withquiet开放加入/平台认证state,chevron. Thinseparator and24pxgap thennext云上竞技cover partiallyvisible. Nofake申请statusornewfilters notpresentinactualcode. Preserveguestbrowsing;创建protectsloginandopensexistingformdialog. Noillustrationneededonpopulatedlist, no decorativebanner, no giantCTA. Alltextwrap, widthbounded.
