# 订单详情 · 开放摘要与证据沟通

文件作用：对应 `frontend/src/pages/orders/detail.vue` 的先行新版设计与实施依据；2026-10-22按二级页面统一任务编辑。旧图/说明已归档 `design/history/20261022-before-secondary-unify/pages/`。

- 去掉订单摘要外层大panel和图中旧缩略图（当前源码没有摘要封面），状态/订单号/单行状态说明，标题与金额、老板/打手、下单服务约定、额外要求使用开放排版与细分隔。
- 标题和长约定允许正常换行，金额可不压缩但不能撑宽；实际单价×单位×数量、交付说明保留，不照搬假数据。
- 图为买家进行中示例，仅有36px申请退款，不显示虚构的等待服务完成主按钮；其他状态/身份按真实actions给出当前可执行动作，核心提交交付44px，辅助36px/文字，不把所有动作堆在一起。
- 聊天、退款/平台裁决等保留必要语义容器。订单聊天室保留通告、历史、消息、角色/时间、图片证据及输入发送；用低对比表面而非多层框。普通顶部返回，无主导航底栏。
- 退款理由/回复、协商暂停验收结算、卖方拒绝/买方介入、管理员裁决、完成后评价等全部保留；退款确认或评价表单沿用底线input/低对比textarea。
- 四方权限仍由服务端限定：老板、打手、相关俱乐部管理员与平台管理员；视觉不扩大聊天可见范围。长文本、滚动、图片证据刷新、会话/定时器保护不改。

## 统一实施约束

内置ImageGen编辑旧图，机械等比缩为390×844。采用自然黑绿/游戏主题色、16px水平边距、24px段落间距、细分隔，输入控件44px、textarea灰底8px，主提交44px、取消文字。图标固定盒双轴居中。生成文字和局部比例误差以真实源码字段、本说明及系统API安全区为准。设计图不是页面背景，实际验收截图另存output。

## 完整编辑提示词

Use case: ui-mockup. Edit the attached existing Chinese390x844 mini-program design. Approved style: open editorial layout on#0a1110, white#edf4ef, mutedgraygreen#9dafA3, mint#8de8b6, thin separators and24pxsectiongap,16pxhorizontalmargin. Remove large enclosing roundedpanels and nestedcards. Textinput: transparent background and singlebottomhairline,44pxhigh; textarea: subtlegray-greenfilledsurface,8pxradius, noouterformcard. Core submit44px, secondary36px/quiettext, officialiconscentered. NO newfeatures,no Englishdecoration,no genericheadline,no giantbuttons,no neon/blue/purplegradients. Keepordinarysharedback/titleheader, reserve nativestatusbar+WeChatcapsule; NO primarybottomnav. Exact390:844portraitflatUI, no deviceframe,noperspective,no outside margins. Data/examplesareillustrativebutfieldstructuremustmatchrealbusiness. Title订单详情. Keepactualorderstatusheading 服务进行中 withsmallordernumber JJ202610220018 andoneshortmutedstagecaption. Reworksummaryfromoldboxedthumbnailcard intoOPEN typography: no largeenclosingcard, NO servicethumbnail becauseactualpagehasnone. Boldtitle三角洲·大坝任务协作 andmint¥68.00 right, allowwrapping; nextsmall老板云间旅人 / 打手北辰. 下单时的服务约定 smallerheading, conciseactualdescriptionand¥68/小时×1, then额外要求/交付说明 normalmultilineflow, notboxedagain. Separatewithhairline. Showbuyerworkingcase: ONLY36pxoutlined申请退款 action, NO fake等待服务完成primarybutton. Otherroles/states haveactualactions(e.g.provider提交交付44) butnotallshownsimultaneously. Belowheading订单聊天室, tinycaption老板·打手·俱乐部·平台 and证据留存 badge. Chat is valid semanticcontainer: understated darksurface with8pxcorners, compactnotice, centered systemnotification订单已支付, leftsellergraybubble 北辰(打手) 已准备好，可以集合; rightbuyermintbubble 云间旅人(老板) 晚上8点开始，目标已写在订单里; role/timevisible. Smallhistorylink andnormalcomposer withimageevidenceicon,input,36发送. Chatheightmanageableandnaturalouter-scroll; no largeillustrationorhero, no bottomtabnav. Refundstate wouldshowonlyrelevantreason/reply/intervention semanticblock, doNOTdeleteflow. Clearorderagreement/evidencehierarchywithoutcardstack.

## 支付弹层变体

待付款买家确认支付时先参照 `detail-payment.png` 与 `detail-payment.design.md`，支持微信支付/钱包余额单选和真实可用余额；充值余额仅可消费。普通正文仍使用本图，不按暗化背景示例放开其他状态付款。
