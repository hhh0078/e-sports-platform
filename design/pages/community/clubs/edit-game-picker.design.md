# 俱乐部装修 · 多选游戏弹框

文件作用：对应 `frontend/src/pages/community/clubs/edit.vue` 的共享游戏选择打开状态变体；正常态仍读取 `edit.png`。本图先保存并检查后才允许对应页面迁移视觉。

- 复用已有首页批准的官方UniPopup底部弹层：圆角18px、主题surface、16px边距、右上44px关闭、44px搜索、3列54px游戏图标与名称、12px网格间隔、选中薄主题边框。页面背景覆盖rgba(0,0,0,0.6)，禁止原生白色滚轮游戏选择；游戏图片必须使用现有官方素材，不能从生成图裁资产。
- 支持当前13个游戏及未来真实配置，网格固定三列并在限定高度内纵向滚动，不把13项全部压进首屏。滚动区约330—360px/max55vh；弹层最高85vh， footer与底部安全区不随列表被挤走。320/390px长名称至多两行且不挤窄相邻格，搜索无结果诚实空态。
- 该PNG仅是同页面游戏选择打开状态；保留同名前缀正常首屏PNG与全部既有业务，不能把设计图当运行背景。生成文字、示意封面及局部压缩误差以此说明和真实数据为准。

- 俱乐部装修覆盖游戏字段打开共享多选弹层，标题“覆盖游戏”，显示真实已选项数；选中主题细边与小勾。先复制game_ids为弹层临时草稿，点击仅修改草稿。
- 底部“取消”为文字，“确认选择”44px；关闭、点遮罩或取消均丢弃临时修改，父表单原选择不变。确认才更新本地game_ids，不提交装修，不更新主游戏偏好。该业务最终至少一个游戏，空选时按统一业务校验提示或禁用确认。
- 保留装修全部图文、封面、规则、草稿、权限及异步上传保护；弹层关闭后回到原表单滚动位置。图中额外拖拽条不作为新手势需求。

## 完整生成提示词

Use case: ui-mockup. Targeted UI STATE VARIANT of image1 approvedexisting390x844page, image2 is visualreference ONLY for the ALREADYAPPROVED homepage ALLGAMES bottomsheet. Preserveimage1 pageunderneath exactly; adduniformcustomofficialUniPopupbottomsheet,NO nativewhitewheelpicker. DoNOTexplorenewstyle. Naturaldarkblackgreen#0a1110/#111e19, white#edf3ef,muted#91a299,currentmint#83e7b7;rgba(0,0,0,0.6) fullscreenscrim, never eight-digithexnativeoverlay. Fullwidthpanelwith18pxTOPcorners,subtle1pxborder,16pxinsets,safeareabottompadding,19pxheadingleftand44pxcloseiconright. Searchfield44pxhwithsearchicon,placeholder搜索游戏名称,subtlegraydarkfillandthinoutline10pxradius. THREEcolumnresponsivegamegrid,54pxSQUAREofficialgameiconcrop10pxradiusabove13-14pxname,max2lines,12pxgap. Gridinnerverticalscroll about330-360px,max55vh;13gamesareREALoptionsincluding三角洲行动/王者荣耀/和平精英/暗区突围/无畏契约/穿越火线手游/金铲铲之战/英雄联盟手游/使命召唤手游/蛋仔派对/第五人格/无畏契约手游/永劫无间手游,showfirst9+partialnextrow/naturalverticalscrollandno tinyfonttocramall13. Imageiconartonlyplacementreference;actualruntimeusesprojectofficialassets,notgeneratedcroppedart. Selectedtilethinmintoutlineandminttext; othersnoborderorlowcontrastborder. No newfeatures/giantillustrations/Englishbanners/bluepurplegradient/no extrafakeonlinecounts. FlatportraitEXACT390:844,no deviceframe/nooutside margin. Pageunderneathisexisting俱乐部装修form,trigger覆盖游戏field. MULTIPLE-selectsheettitle覆盖游戏,smallsubline已选3项. 三角洲行动/王者荣耀/和平精英selectedwithmintborderandtinycheckincorner. SelectionisDRAFTuntilconfirm. FooterbelowINDEPENDENTscrollgrid:quiet取消textleft,one44pxmint确认选择rightwithadequategapandbottomsafearea. CloseX/scrim/canceldiscardtemporaryselectionandpreserveparentform;confirmupdateslocalgame_idsONLY,neverhomepreference. Maintainbottomsheetmax85vhnaturalscrollfor13items,no crowdedpage,noshrink. No nativewheel,pagetitleabove remainsdim.
