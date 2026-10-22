# 个人资料 · 静态接单身份与钱包

文件作用：对应 `frontend/src/pages/account/profile.vue` 的先行设计及用户最新局部要求；2026-10-22生成/编辑。已有页面上一版归档在 `design/history/20261022-before-game-refinements/pages/`，钱包为本次新增，无旧版。

- 仅把接单身份改为静态只读行：标题只显示一次“接单身份”，右侧“默认开启”，不再有NativeSwitch、下拉箭头或“开启后…”文案，能力默认具备且无法关闭。
- 手机号行之后增加“我的钱包”轻入口，使用同款官方图标/细分隔/箭头，跳转钱包，无假余额和本页充值操作。
- 保留用户头像1:1裁剪压缩、昵称2～24字、简介300字、手机号验证、个人16:9封面、保存/取消、公开主页/退出登录；输入仍透明底线、textarea低对比8px，不恢复大外框。
- 访客资料为空且不可编辑，接单说明可为“登录后即可接单”；默认身份不绕过冻结、手机号与发布简介等原校验。
- 普通统一返回，无主底栏；内容自然滚动，不能为在首屏塞完新增钱包入口而压扁16:9封面。图中局部图片/文字示意以真实比例和本说明为准。

## 生成与验收边界

390×844，使用内置ImageGen。既有页面仅局部编辑，钱包新建；输出仅机械等比缩放。生成示例文字/数字/局部比例以真实业务、本说明和系统安全区为准，不能作为真实资金凭证。实际SDK与交易验收由root执行，本图不代表支付/真机通过。

## 完整生成或编辑提示词

Use case: ui-mockup. PRECISE LOCAL EDIT of attachedexistingChinese390x844design. Preserveallunrelatedapprovedlayout, naturaldarkgreen#0a1110,mint#8de8b6,thinsectiondividers,16pxinsets,officialcenteredicons,existingfieldsandnativeWeChatsafearea. DoNOTinventfeatures,newart,headlines,English,bluepurplegradients,orreplacegameartsource. Onlyedittheexplicitprovideridentitypoint. Exact390:844flatportrait,no deviceframe,nooutside margins. TextisindependentUI, notbakedintoimages. Keepordinary个人资料sharedbackheaderandallactualprofilefields/avatar/phoneverification/cover/save/cancel/publichome/logoutunchanged. ChangeONLYproviderrow: removegreenONswitchentirelyandremoveold我可以接单/开启后他人可向我发起服务邀请copy. ReplacewithcleanSTATICread-onlyrow, smallofficialworkiconleft, title 接单身份,quietsubline 默认拥有接单能力, smallmint 默认开启 at right. No toggle, dropdown, disableoption,switchtrackorthumb. Gueststatecanread登录后即可接单withnoneditableprofilefields, nocapabilityenableaction. Preservephoneverificationrule, frozenaccountcontrols,andrequiredprofileintroforexistingpublishflow; defaultidentitydoesNOTbypasschecks. Inputstylesremaintransparentbottomlines44px/textareaquiet8px; doNOTaddnewoutercardsorfields. Keepwholelayoutandgamecoverphotofromtarget,allothercontentidentical.

钱包轻入口局部追加：
Use case: ui-mockup. PRECISELOCALedit attached390x844profileeditor. Preserveallordinary个人资料backheader/avatar/name/bio/phone/cover/save/cancel/publichome/logout, naturaldarkgreenstyle,formsandsafearea. AddONLYquiet44px 我的钱包 row withofficialwalletoutlineiconandchevron afterphoneverificationandbeforestaticproviderrow, thinseparatorssameasexistingrows. No balance/no fakefunds/no bigwalletcard/no rechargebuttonhere. ProvideridentityremainsoneSTATICread-onlyrow 接单身份 with 默认开启 at right, no switchtrack/thumb/dropdownorenableaction; removeanyduplicateextra接单身份headingabovethisrowsoitappearsjustonce. Ifnewrowpusheslowercover/savebelowfirstscreen,allow naturalscrollratherthancompressingallfieldsorimage. Inputtransparentbottomline44px/textareaquiet8px, doNOTreintroducebigouterformpanel. Keepactualbusinessfieldsandallotherartunchanged. Flat390:844portrait,no deviceframe,nooutside margins,no purplebluegradient.

删除重复小标题的最终单点修正：
Use case: precise-object-edit / ui-mockup. The attached390x844profileUI is ALMOST FINAL. Make EXACTLYONE small deletion: remove the SMALL section-caption text 接单身份 at x20 y535 (the one just below我的钱包row and just ABOVE the provider-iconrow). Keep the OTHER 接单身份 label INSIDE the providerrow, nearx60 y569, and keep its subtext 默认拥有接单能力 and righttext 默认开启. There must be onlyONE occurrence of接单身份onthepage. Replace deletedcaptionarea with matching plain#0a1110 background. DoNOT delete the providerrow itself. DoNOT moveorchangeanyotherpixelarea, walletrow, avatar, cover, fields, save/cancel buttons, text, colorsorlayout. No switch. No otheredits. Keepflat390:844portrait,no deviceframe.
