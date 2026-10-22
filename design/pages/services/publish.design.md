# 发布服务 · 开放式编辑

文件作用：对应 `frontend/src/pages/services/publish.vue` 的先行新版设计与实施依据；2026-10-22按二级页面统一任务编辑。旧图/说明已归档 `design/history/20261022-before-secondary-unify/pages/`。

- 已移除选择游戏、封面文字、服务信息等整段外层panel；以裸背景、24px节距和细分隔形成分区。input透明底线、textarea低对比灰底8px，单个真实控件可保留自身边界。
- 普通统一顶部返回，无主底栏。游戏选择为44px轻量单行，换底图为预览标题右侧文字入口。
- 选择游戏字段统一打开共享GameSelector自定义搜索/三列图标弹层，单选即选即回，取消保留草稿。先行打开状态见[publish-game-picker.design.md](publish-game-picker.design.md)与同名PNG；不再使用原生游戏滚轮，服务分类/单位/挂靠等其他picker不在本次范围。
- 发布预览复用同一ServiceCard的cover-only模式，源图16:9、约4:3显示裁切；图底从透明渐变到主题背景，标签在上、标题在下独立悬浮。此处不显示价格/成交正文，替代此前预览价成交规则；列表仍正常展示价成交。
- 自定义文案保持独立图层，与标签/标题预留区避让；最多6条，每条按实际工具规范化为最多12个字；字号实际12～24。生成图“最多6个字”、16～32滑块等示例误差不得照搬。
- 保留全部真实编辑能力：游戏底图缩略图、添加/删除、字色/背景色和色值、九宫格位置、字号；每条编辑器通过细分隔区别，不套重复大框。首屏放不下自然滚动，不压扁预览。
- 下方真实字段为标题40字、分类、价格/单位、说明1000字、最多4个标签、可选已认证俱乐部。保存/审核流程与手机号门槛保持不变。
- 提交审核为44px唯一主操作；不新增图中示意字段，也不删除源码已有字段。图片比例、文字越界/碰撞及320px宽度仍需实际验收。

## 统一实施约束

内置ImageGen编辑旧图，机械等比缩为390×844。采用自然黑绿/游戏主题色、16px水平边距、24px段落间距、细分隔，输入控件44px、textarea灰底8px，主提交44px、取消文字。图标固定盒双轴居中。生成文字和局部比例误差以真实源码字段、本说明及系统API安全区为准。设计图不是页面背景，实际验收截图另存output。

## 完整编辑提示词

Use case: ui-mockup. Edit the attached Chinese390x844 mini-program design to approved OPEN editorial system: #0a1110 darkbackground, #edf4efwhite, #9dafA3muted, #8de8b6mint. NO largeenclosingpanels, no nestedroundedcards, no repeatedboxes, no dashedframes, no decorativehero/headline/English, no illustrationunlessexistingmeaningful. Thinsectiondividers,24pxverticalgaps,16pxhorizontalinsets. Textinputs44px transparentwithONLYbottomhairline;textarea lowcontrastgrayfill8pxradius. Maintransactionsubmit44px,otheractions36pxorquiettext. Officialiconscentered. Ordinarysharedback/titleheader withnativeWeChatstatusbar/capsulesafearea;NO primarybottomnav. Exact390:844flatportraitUI,no deviceframe,nooutside margins. Preserveactualfieldsandflows, no newfeatures. Title发布服务. Removeoldouterboxesaround选择游戏/封面预览/封面文字/服务信息; sectionsdirectlyondarkbackground. Gamepickerbare44pxunderlinedrow三角洲行动, notonepanelinsideanother. Previewheading封面预览 withquiet换一张底图 right. PreviewusesSAMEunifiedServiceCard inCOVER-ONLYmode: gameartsource16:9,displaycropabout4:3, bottomtransparent-to-opaque#121d18gradient, independentlabelsabove independentboldtitleINSIDEimage. Usercustomwords耐心沟通稳定撤离 safelayoutabovefixedmetadata, NOT bakedintoasset. Cover-onlymeansNO price/成交bodyinpreview,remove¥68/已成交0fromoldimage. Afterpreviewthinseparator/24gap then封面文字 header withsmall添加文字textaction; supportinghintonequietline. Showcompact16:9bottomcoveroptionsasthumbnailsifspace, then叠加文字1/6andremovequietlink,underlinedinput耐心沟通. Colorswatches/backgroundselection/position/字号slider inopenrows withnofullroundedlabel-editorbox. Preservefullactualeditorbelowviewport, don'tcramallintofirstscreen. 服务信息 continuesfurtherdownwithtitle,category,price/unit,description,tags,optionalclubpickerusingthesameunderlinedfieldstyle. Sole44pxmint提交审核actionatbottom. No fakehero,newfeatures,orbigplaceholderparagraph. Naturalworkingformwithclearhierarchy.

## 2026-10-22自愿平台押金局部增补

发布预览保持cover-only，不恢复虚假的¥0/成交0。当前打手真实provider_deposit_cents>0时图下另起小行显示平台押金标识；预览下增加轻量自愿说明“缴纳后展示平台押金标识，不影响发布”和“我的钱包”文字入口。未缴不阻止发布，不新增强制条件，不作担保/赔付承诺；图中OCR差异不得照搬。

完整局部编辑提示词：

Use case: ui-mockup. PreciseLOCALeditofattachedChinese390x844design. Preserveallunrelatedstyle/fields/layout/gameart/safeareas,approved#0a1110darknaturalgreenand#8de8b6mint, thinseparatorsandcentredofficialicons. No newfeatures, no giantcards, no Englishdecoration/no bluepurplegradient. AllnewtextindependentUI,nobakedgameart. Exact390:844portraitflatUI,noframe/nooutside margins. ChangeONLYdepositvisibilitysupplement: cover-onlyServiceCardpreviewmayhaveoneSMALLindependentnormal-flowrowbelowtheimage 平台押金 ifthecurrentproviderhaspositivedeposit. DoNOTrestorefake¥0/成交0body. DoNOTputbadgeincustomcovertext/clubcertificationarea. Immediatelybelowpreviewaddquiet2-linevoluntaryhint 缴纳后展示平台押金标识，不影响发布 plus small我的钱包→textentry. Notmandatory,nocheckbox,no blockedsubmit,noguarantee/compensationpromise. Keepallactualcovereditor/formfields,controlsand44px提交审核; allowremainingfieldsscrollbelowviewport,nocrammingorremovingfields. Allotherlayoutunchanged.
