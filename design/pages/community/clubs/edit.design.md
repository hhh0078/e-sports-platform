# 俱乐部装修 · 开放式分区

文件作用：对应 `frontend/src/pages/community/clubs/edit.vue` 的先行新版设计与实施依据；2026-10-22按二级页面统一任务编辑。旧图/说明已归档 `design/history/20261022-before-secondary-unify/pages/`。

- 整段edit-panel/panel全部去除；主页形象、图文详情、加入规则使用form-section裸底细分隔，24px节距，不另造预览功能。
- 顶部审核恢复提示为小字。封面实际16:9，图中压窄的示例不能作为比例；更换封面轻量入口，仍经官方裁剪压缩和服务端校验。
- 真实字段保持名称2～30字、简介10～1000字、多选覆盖游戏；input透明底线，textarea低对比灰底8px，游戏标签可换行并受容器限制。
- 图文详情保留最多20段/10图/总文字8000字；每段真实上移/下移/删除与添加文字/图片，段落间细分隔。空段用小字提示，不用虚线空框。
- 加入门槛在图文详情之后：押金展示0～100000元且最多两位小数、平台不代收/核验押金、考核最多1000字及门槛变更后的重新确认。界面不得暗示实际代收押金。
- 保存并提交审核44px，取消修改为文字入口。仅负责人可编辑，普通用户保留权限状态；权限、草稿、异步上传/保存和会话隔离逻辑不改。自然滚动，不为首屏塞完压扁图片或输入。

## 统一实施约束

内置ImageGen编辑旧图，机械等比缩为390×844。采用自然黑绿/游戏主题色、16px水平边距、24px段落间距、细分隔，输入控件44px、textarea灰底8px，主提交44px、取消文字。图标固定盒双轴居中。生成文字和局部比例误差以真实源码字段、本说明及系统API安全区为准。设计图不是页面背景，实际验收截图另存output。

## 完整编辑提示词

Use case: ui-mockup. Edit the attached existing Chinese390x844 mini-program design. Approved style: open editorial layout on#0a1110, white#edf4ef, mutedgraygreen#9dafA3, mint#8de8b6, thin separators and24pxsectiongap,16pxhorizontalmargin. Remove large enclosing roundedpanels and nestedcards. Textinput: transparent background and singlebottomhairline,44pxhigh; textarea: subtlegray-greenfilledsurface,8pxradius, noouterformcard. Core submit44px, secondary36px/quiettext, officialiconscentered. NO newfeatures,no Englishdecoration,no genericheadline,no giantbuttons,no neon/blue/purplegradients. Keepordinarysharedback/titleheader, reserve nativestatusbar+WeChatcapsule; NO primarybottomnav. Exact390:844portraitflatUI, no deviceframe,noperspective,no outside margins. Data/examplesareillustrativebutfieldstructuremustmatchrealbusiness. Title俱乐部装修, removeold预览navlinkbecauseactualpagehasnosuchfeature. Shortquiettopnote 修改资料后重新审核，认证在通过后恢复. Openformfirstsection主页形象with16:9coverpreview358x201, quiet更换封面36/textentrybelowright; noouterpanel. Fields俱乐部名称2—30字 transparentunderlinedinput山海电竞; 简介10—1000字 lowcontrast96pxtextarea; 覆盖游戏可多选 compactwrapchips三角洲王者荣耀和平精英 withsmallchecks. Thinseparator24gap then图文详情 withquiet添加文字/添加图片actions, paragraphblock withsmalltoolbar上移下移删除 andeditablebody; NO dashedplaceholderbox, no newpreview. Furtherdownbeyondfirstscreen retain加入规则:押金门槛0—100000元, 平台不代收或核验押金 smalltext;考核要求max1000 textarea. Actualorder fieldsisidentity,richtext,joinrules; don'tputjoinrulesabove richtextasoldimage. One44mint保存并提交审核 andquiet取消修改 atpagebottom; firstscreenmaynaturallyshowpartialrichtext,nevercramallfields. No decorativeillustration, preserveadminonlybusiness.
