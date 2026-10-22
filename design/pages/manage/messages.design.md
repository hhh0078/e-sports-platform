# 发送平台消息 · 管理独立页

文件作用：对应 `frontend/src/pages/manage/messages.vue` 的先行390×844设计与真实业务边界。

- 管理员发送平台消息独立页面，共享返回、无主底栏。接收人ID可空，空字符串广播给active真实账号；非空定向。标题1—60字，正文1—5000字，recipient_id最长128字节。图中计数按此真实契约，不保留最初80字示例。
- 仅调用POST /admin/messages {recipient_id?,title,body}，响应{recipient_count}。空接收人发送前通过官方确认弹框明确全站广播及标题，定向也确认对象；取消不发请求，提交防连点。成功按真实recipient_count提示，不能伪造送达/已读结果。
- 冷启动失败可重试init；普通玩家及俱乐部管理员无表单、无发送权限。隐藏/切账号作废旧确认与响应并清除草稿，迟到成功不能覆盖新账号或继续发出请求。
- 开放式分节、44px透明底线输入、灰底8px正文textarea、16px边距、细分隔；44px发送消息唯一主提交，取消修改为文字。长正文与长用户ID不能水平撑开。

## 完整生成与局部编辑提示词

Use case: ui-mockup. One professional Chinese WeChat Mini Program UI,exact390:844flatportrait,no frame/nooutside margins. Same established 集结 darknaturalblackgreen #0a1110,white#edf4ef,muted#9dafa3,mint#8de8b6. Sharedsecondarybackarrow/title beneathnativeWeChatstatus/capsulesafearea. NO primarybottomnav. 16pxinsets,24pxsectiongaps,thin separators,officialcenteredicons,body16px/secondary14px,openeditorial layout,44pxprimarysubmit and inputs,36pxcompactactions,textarea lowgray8pxradius. NO nestedpanels,giantcards,dashedemptyframes,illustrations,Englishdecoration orbluepurplegradient. Alltext real UI elements, designisnotbackground. Title发送平台消息. Adminonlyopenform. Firstsection接收对象 smallusericon,field用户ID（选填）,transparent44pxinput placeholder 留空发送给全部用户. Muted14pxhelper 留空为全站广播，请确认内容后发送. No fakeuserselectororsuggestedpeople. Thinseparator. Nextsection消息内容 smallnotificationicon. Field标题,small0/80counter,input44pxplaceholder 输入消息标题. Field正文,0/5000counter,lowgraytextarea180px placeholder 输入完整消息内容. Largeenough16pxtext. Beneathsingle44pxmint预览并发送 or发送消息 mainaction peractualconfirmationflow, use发送消息. Underquiet取消修改text. Sendingrequiresnativeconfirmationwithrecipient/broadcastscopeandtitle;modalnotshown. Serverreturnsactualrecipient_count,successshownrealcountonly,notprebakedtoast. No AIsettings/keyfields,no historylistwithoutAPI,no fakecountsorbalance. Canverticalscroll, maxwidthbounded,neutralcleanhierarchy.

Use case: ui-mockup. Targetededit attached390x844发送平台消息adminform. Preserveexactpagecontent,header,statusbar,blacknaturalgreen/mintpalette,16pxinsets,sectionspacing,icons,textarea,44px发送消息andquiet取消修改,alllabelsandcounters. ChangeONLYthreevisualdetails:用户IDand标题textinputsbecome44pxtransparentOPENunderlinedinputs,removefullroundedboxfillandtop/sideborders. Textareakeepssubtlegray8pxroundedfill. No enclosingouterpanels,no otherchanges. Nativecapsulesafespace. Flat390:844,no frame/nooutside margins.

Use case: ui-mockup. Preciselocaleditattached390x844发送平台消息form. PreserveEXACTalllayout,blackgreenmintcolors,16pxinsets,underlinedinputs,textarea,labels,headerandsubmit. ChangeONLYtitlecounterfrom0/80to0/60,actualbackendlimit60Chinesecharacters. Bodycounter0/5000unchanged. Noothernewfeaturesorchanges. Flat390:844,no frame.
