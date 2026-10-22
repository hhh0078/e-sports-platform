# 发布需求 · 游戏选择弹框

文件作用：对应 `frontend/src/pages/requests/publish.vue` 的共享游戏选择打开状态变体；正常态仍读取 `publish.png`。本图先保存并检查后才允许对应页面迁移视觉。

- 复用已有首页批准的官方UniPopup底部弹层：圆角18px、主题surface、16px边距、右上44px关闭、44px搜索、3列54px游戏图标与名称、12px网格间隔、选中薄主题边框。页面背景覆盖rgba(0,0,0,0.6)，禁止原生白色滚轮游戏选择；游戏图片必须使用现有官方素材，不能从生成图裁资产。
- 支持当前13个游戏及未来真实配置，网格固定三列并在限定高度内纵向滚动，不把13项全部压进首屏。滚动区约330—360px/max55vh；弹层最高85vh， footer与底部安全区不随列表被挤走。320/390px长名称至多两行且不挤窄相邻格，搜索无结果诚实空态。
- 该PNG仅是同页面游戏选择打开状态；保留同名前缀正常首屏PNG与全部既有业务，不能把设计图当运行背景。生成文字、示意封面及局部压缩误差以此说明和真实数据为准。

- 发布需求“选择游戏”44px字段打开共享单选弹层，标题“选择游戏”；点击即更新本地gameId并关闭，取消保留原游戏与需求草稿。不改写首页偏好或触发订单提交。
- 需求标题、目标验收、预算、手机号与支付门槛继续沿用原流程，游戏选择统一不涉及排序/日期/单位等其他原生picker。

## 完整生成提示词

Use case: ui-mockup. Targeted UI STATE VARIANT of image1 approvedexisting390x844page, image2 is visualreference ONLY for the ALREADYAPPROVED homepage ALLGAMES bottomsheet. Preserveimage1 pageunderneath exactly; adduniformcustomofficialUniPopupbottomsheet,NO nativewhitewheelpicker. DoNOTexplorenewstyle. Naturaldarkblackgreen#0a1110/#111e19, white#edf3ef,muted#91a299,currentmint#83e7b7;rgba(0,0,0,0.6) fullscreenscrim, never eight-digithexnativeoverlay. Fullwidthpanelwith18pxTOPcorners,subtle1pxborder,16pxinsets,safeareabottompadding,19pxheadingleftand44pxcloseiconright. Searchfield44pxhwithsearchicon,placeholder搜索游戏名称,subtlegraydarkfillandthinoutline10pxradius. THREEcolumnresponsivegamegrid,54pxSQUAREofficialgameiconcrop10pxradiusabove13-14pxname,max2lines,12pxgap. Gridinnerverticalscroll about330-360px,max55vh;13gamesareREALoptionsincluding三角洲行动/王者荣耀/和平精英/暗区突围/无畏契约/穿越火线手游/金铲铲之战/英雄联盟手游/使命召唤手游/蛋仔派对/第五人格/无畏契约手游/永劫无间手游,showfirst9+partialnextrow/naturalverticalscrollandno tinyfonttocramall13. Imageiconartonlyplacementreference;actualruntimeusesprojectofficialassets,notgeneratedcroppedart. Selectedtilethinmintoutlineandminttext; othersnoborderorlowcontrastborder. No newfeatures/giantillustrations/Englishbanners/bluepurplegradient/no extrafakeonlinecounts. FlatportraitEXACT390:844,no deviceframe/nooutside margin. Pageunderneathisexisting发布需求form,trigger选择游戏field. Single-selectsheettitle选择游戏,三角洲行动selected. TapitemupdatesONLYrequestlocalgameandcloses,doesnotchangehomepreference. NOconfirmfooterforSINGLEselect. Existingtitle/requirements/budget/submitunchangedbehindrgbaoverlay,secondaryheaderandnohomebottomnav.
