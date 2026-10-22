# 平台管理 · 轻量数据排版

文件作用：对应 `frontend/src/pages/manage/index.vue` 的先行新版设计与实施依据；2026-10-22按二级页面统一任务编辑。旧图/说明已归档 `design/history/20261022-before-secondary-unify/pages/`。

- 新图已去掉指标小卡、整段审核卡、虚构筛选和默认拒绝弹层；裸底四列真实指标+细分隔、紧凑待办提示、横向受控导航、底线搜索形成层级。
- 真实指标为注册用户、全部订单、服务数量、完成交易额；使用服务端数据，不添加演示标签或图中示例数值。
- 真实导航依次服务审核、俱乐部、订单与售后、用户、游戏配置、操作记录、平台设置；横向滚动受最大宽度约束，不能缩窄邻居或溢出。
- 审核列表开放排版，缩略图/标题/状态/作者/价格与必要操作使用正常流，项间细分隔。紧凑操作36px，查看详情文字入口，禁止两枚大按钮占整卡。
- 其余资源页沿用相同语言：用户/俱乐部/订单/游戏使用简洁分隔列表，审计保留日志层级，平台设置使用form-section。游戏主题编辑弹窗不堆外框，图片库与色值沿用真实字段。
- 不按旧图新增不存在的筛选或拒绝理由表单；源代码现有审核、冻结、游戏与支付管理权限和确认流程保持。非管理员仅显示真实权限状态。
- 普通统一返回，无主底栏；搜索/刷新/翻页/切换状态、会话保护与管理数据范围不改。

## 统一实施约束

内置ImageGen编辑旧图，机械等比缩为390×844。采用自然黑绿/游戏主题色、16px水平边距、24px段落间距、细分隔，输入控件44px、textarea灰底8px，主提交44px、取消文字。图标固定盒双轴居中。生成文字和局部比例误差以真实源码字段、本说明及系统API安全区为准。设计图不是页面背景，实际验收截图另存output。

## 完整编辑提示词

Use case: ui-mockup. Edit the attached Chinese390x844 mini-program design to approved OPEN editorial system: #0a1110 darkbackground, #edf4efwhite, #9dafA3muted, #8de8b6mint. NO largeenclosingpanels, no nestedroundedcards, no repeatedboxes, no dashedframes, no decorativehero/headline/English, no illustrationunlessexistingmeaningful. Thinsectiondividers,24pxverticalgaps,16pxhorizontalinsets. Textinputs44px transparentwithONLYbottomhairline;textarea lowcontrastgrayfill8pxradius. Maintransactionsubmit44px,otheractions36pxorquiettext. Officialiconscentered. Ordinarysharedback/titleheader withnativeWeChatstatusbar/capsulesafearea;NO primarybottomnav. Exact390:844flatportraitUI,no deviceframe,nooutside margins. Preserveactualfieldsandflows, no newfeatures. Title平台管理. Thisisrealadminconsole,nogameheroandnoillustration. Removeoldthreetileoverviewandfakedemo/filter/modal. Actual4metrics directonbackground inrefined2x2or4columnstrip:60注册用户,52全部订单,130服务数量,¥2680完成交易额; hairlinedividers, mintfigures, noindividualroundedmetricboxesandNO演示badges. Smallquietpendingcountsline 8服务待审核·2俱乐部申请·1争议. Compacthorizontalunderlinetabs actualorder 服务审核 俱乐部 订单与售后 用户 游戏配置 操作记录 平台设置, allowboundedhorizontalscroll. Underlinedsearch搜索全部名称、用户或订单号 plus smallrefreshicon at right. Actualserviceauditlisttwo-threeentries, OPENseparatedrows notlargecards:thumbnail72x54,title大坝任务协作, pendingstatusright,provider北辰·任务协作, price¥68/小时,shortdescription. 36pxcompact通过审核 and不予通过 asnecessarybusinessactions, 查看详情quiettext. Thinseparatorbetweentheentries. Removeunimplementedthreefilterdropdownsandtheoldrejectionmodalfromshownscreen; existingconfirmationsremaininrealflowbutnotshownbydefault. Preserveallotherresourceadmin tabs/permissions/paginationnotillustrated. No giantactionpair, doNOT showactualpasswordsIDssecrets.

## 钱包管理入口增补

平台设置页在既有协议/支付配置入口附近增加同款文字入口“钱包管理与提现审核”，跳转 `/pages/manage/wallet`；不新增资源API分类、不改摘要与列表结构，复用已先行设计的轻入口规范。钱包管理具体页面参考同目录wallet.png及说明。
