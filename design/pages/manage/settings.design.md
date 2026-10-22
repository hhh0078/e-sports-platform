# 平台配置 · 开放式协议与支付表单

文件作用：对应 `frontend/src/pages/manage/settings.vue` 的先行新版设计与实施依据；2026-10-22按二级页面统一任务编辑。旧图/说明已归档 `design/history/20261022-before-secondary-unify/pages/`。

- 新图去掉两协议的大圆角外框及大分段按钮；协议配置/微信支付改轻量下划线导航，细分隔和24px节距区分文档。普通统一返回，无主底栏。
- 仅input透明底线、textarea低对比灰底8px；公开标题2～80字、版本号最多64字、正文30～8000字、更新时间与查看当前发布版本均保留，禁止照搬图中假正文/统计。
- 发布协议版本44px；取消修改为文字动作。隐私政策和用户协议共用同套开放字段布局，内容长时自然滚动，不压缩控件挤首屏。
- 微信支付页直接复用同一设计语言，无需再加装饰图：顶部配置状态为小图标+状态/说明开放行；商户信息与密钥证书通过细分隔分区，不再整段套panel。
- 支付公开字段保持商户号、商户API证书序列号、微信支付公钥ID、支付回调HTTPS地址、退款回调HTTPS地址；输入44px底线，长URL允许正常输入且不能撑宽页面。
- 秘密字段为APIv3密钥、商户私钥PEM、微信支付平台公钥PEM；已配置/未配置状态不回显秘密，留空保留，输入秘密掩码/PEM用低对比textarea；保存成功或离开页面清空。绝不能在图、日志或设计里放实际密钥。
- 本地测试支付提示、服务端加密存储可用状态、权限校验、加载/失败重试、版本冲突保护均保留；保存支付配置44px，取消文字。
- 初始化失败在身份分支外显示明确的“重新加载”，重新执行init和领域配置读取；已知访客保持原权限说明，尚未初始化的网络失败不得只剩顶栏。

## 统一实施约束

内置ImageGen编辑旧图，机械等比缩为390×844。采用自然黑绿/游戏主题色、16px水平边距、24px段落间距、细分隔，输入控件44px、textarea灰底8px，主提交44px、取消文字。图标固定盒双轴居中。生成文字和局部比例误差以真实源码字段、本说明及系统API安全区为准。设计图不是页面背景，实际验收截图另存output。

## 完整编辑提示词

Use case: ui-mockup. Edit the attached Chinese390x844 mini-program design to approved OPEN editorial system: #0a1110 darkbackground, #edf4efwhite, #9dafA3muted, #8de8b6mint. NO largeenclosingpanels, no nestedroundedcards, no repeatedboxes, no dashedframes, no decorativehero/headline/English, no illustrationunlessexistingmeaningful. Thinsectiondividers,24pxverticalgaps,16pxhorizontalinsets. Textinputs44px transparentwithONLYbottomhairline;textarea lowcontrastgrayfill8pxradius. Maintransactionsubmit44px,otheractions36pxorquiettext. Officialiconscentered. Ordinarysharedback/titleheader withnativeWeChatstatusbar/capsulesafearea;NO primarybottomnav. Exact390:844flatportraitUI,no deviceframe,nooutside margins. Preserveactualfieldsandflows, no newfeatures. Title平台配置. Preserveactualtop2tabs协议配置/微信支付, usecompactleftalignedunderlinetabs NOTbigfilledsegmentedblock. Firsttabactive. A smallunboxedhelper 发布协议时请填写新版本号. UseragreementsectionisOPEN onbackground notouterroundedpanel: smallofficialdocumenticon+用户服务协议18px, tinyupdateddate right. Fields公开标题2—80字 transparentunderlined44pxinput集结用户服务协议;版本号 transparentunderlined44pxinput2026-10-22.2;完整正文30—8000字 withcounteratright,lowcontrast8pxtextareaabout160-200px. BodyreadableChineselegaltext,no fake180/180limit. Below quiet查看当前已发布版本link. Thinseparator24gap then隐私政策sectionbeginswithsamefields,pagetakesnaturalscrollratherthancrammingfulltext. One44pxmint发布协议版本 atbottomand取消修改quiettext; no duplicate支付配置shortcutbarbecauserealpaymenttabhandlesit. Paymentalternate(notshown)usestheseexactopenformstylesformerchantfields,knownsecretstatusandmaskedinputs/PEMtextareas; neveractualkeydata. No illustrationorbanner. Keepactualadminonlyguardsandpublictitlefieldmissingfromoldimage.
