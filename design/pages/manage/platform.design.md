# 平台设置 · 独立模块设计

文件作用：对应 `frontend/src/pages/manage/platform.vue` 的先行ImageGen设计与实施依据；2026-10-22管理模块拆分。

- 平台管理员专用，仅getAdminSettings/updateSettings公开平台名称与公告；不引入商户密钥、微信登录配置或其他额外开关。普通用户、俱乐部管理员路由拒绝，后端独立校验。
- 共享返回标题“平台设置”，无主底栏；开放基础信息/平台公告分节，16px边距、细分隔、24px节距。名称44px底线输入，公告灰底8px圆角textarea，按实际后端契约名称2—24字、公告0—300字。图中4/20仅样例，运行时计数显示实际24字上限，避免已有合法名称不能保存。
- 保存设置44px唯一主提交，取消修改为文字；保留加载、失败重试、保存防连点、离页/换号旧响应隔离。保存成功以服务端结果回填，随后init(true)刷新全局公开配置；公告空字符串允许清空。
- 冷启动尚未确定身份时，bootstrap失败也必须在权限分支外显示错误和重试；重试重新初始化身份，成功确认管理员后再读取配置，不能只剩顶栏。

- 自然黑绿主题，白色主文案、灰绿辅助、薄荷主色；生成文字误差以真实业务和本说明为准。原生胶囊/状态栏按系统安全区实现，设计中没有胶囊的示例不可据此去掉避让。空态轻量，错误可重试，内容自然滚动。

## 完整生成提示词

Use case: ui-mockup. Create Chinese WeChat Mini Program secondary administrator form,exact390:844flatportrait,no deviceframeoroutside margin. Approved #0a1110 natural darkgreen background,#edf4efwhite,#9dafa3muted,#8de8b6mint. Native topstatusbarandcapsulesafearea, sharedbackarrow title 平台设置. NO mainbottomnav. Open editorial form,16pxhorizontalinsets,24pxsectiongaps,thin separators,officialcenteredoutlineicon,input44pxtransparent with bottomlineonly,textarea lowgray8pxradius. NO giant enclosingpanels,nestedcards,decorativehero/English/illustrations/bluepurplegradient. Formfirstsmallheading 基础信息 withquietconfigurationicon. Field平台名称,smallcounter4/20 at right,input 开机电竞. Thinseparator. Heading平台公告 withquietnotificationicon. Field公告内容, counter0/300,lowgraytextareaabout160pxhighwithplaceholder 输入要向玩家展示的平台公告; no giantfilledfakeannouncement. Smallmutedhelper 公告为空时，玩家页不显示公告. Savefooter inordinaryflowabouty485:singlemint44px保存设置 fullwidth,belowquiettext取消修改. No payment merchant/login/secretsfields onthispage. Read/save realgetAdminSettings/updateSettings only,permissionplatformadmin;runtimeerror/loading/retryandemptykept. Usecalmwhitespace,don't invent extrasettings/toggles/upload. No fake success toast. Pagecan scrollsmallheight. Clear Chinese14pxlabels16pxinput,properalignment.

## 品牌名称更新（2026-10-23）

平台名称输入示例为开机电竞，保持名称可配置、原表单尺寸和保存操作。 本次只局部替换品牌文字，不新增界面、控件、图片或视觉规则。历史图保留原名称。

编辑提示词：Use case: text-localization. Edit only the product brand text in the attached existing 390x844 design. Use exact Chinese brand 开机电竞. 平台名称输入示例为开机电竞，保持名称可配置、原表单尺寸和保存操作。 Keep all unrelated layout, artwork, colors, type sizes, fields, amounts, safe areas and dimensions unchanged. No redesign, stretching, added height or new decorations.
