# 发布服务 · 游戏选择弹框

文件作用：对应 `frontend/src/pages/services/publish.vue` 的共享游戏选择打开状态变体；正常态仍读取 `publish.png`。本图先保存并检查后才允许对应页面迁移视觉。

- 复用已有首页批准的官方UniPopup底部弹层：圆角18px、主题surface、16px边距、右上44px关闭、44px搜索、3列54px游戏图标与名称、12px网格间隔、选中薄主题边框。页面背景覆盖rgba(0,0,0,0.6)，禁止原生白色滚轮游戏选择；游戏图片必须使用现有官方素材，不能从生成图裁资产。
- 支持当前13个游戏及未来真实配置，网格固定三列并在限定高度内纵向滚动，不把13项全部压进首屏。滚动区约330—360px/max55vh；弹层最高85vh， footer与底部安全区不随列表被挤走。320/390px长名称至多两行且不挤窄相邻格，搜索无结果诚实空态。
- 该PNG仅是同页面游戏选择打开状态；保留同名前缀正常首屏PNG与全部既有业务，不能把设计图当运行背景。生成文字、示意封面及局部压缩误差以此说明和真实数据为准。

- 发布服务“选择游戏”44px字段打开共享单选弹层，标题“选择游戏”，点击即更新本地服务游戏并关闭；不得写首页游戏缓存。原服务切游戏后的分类、随机对应底图及封面文字安全规则继续按业务处理，取消不清表单。
- 不新增确认按钮或恢复原生wheel。标题、封面预览、叠加文字、价格、单位、分类、挂靠及提交审核等全部既有能力不变。

## 完整生成提示词

Use case: ui-mockup. Targeted UI STATE VARIANT of image1 approvedexisting390x844page, image2 is visualreference ONLY for the ALREADYAPPROVED homepage ALLGAMES bottomsheet. Preserveimage1 pageunderneath exactly; adduniformcustomofficialUniPopupbottomsheet,NO nativewhitewheelpicker. DoNOTexplorenewstyle. Naturaldarkblackgreen#0a1110/#111e19, white#edf3ef,muted#91a299,currentmint#83e7b7;rgba(0,0,0,0.6) fullscreenscrim, never eight-digithexnativeoverlay. Fullwidthpanelwith18pxTOPcorners,subtle1pxborder,16pxinsets,safeareabottompadding,19pxheadingleftand44pxcloseiconright. Searchfield44pxhwithsearchicon,placeholder搜索游戏名称,subtlegraydarkfillandthinoutline10pxradius. THREEcolumnresponsivegamegrid,54pxSQUAREofficialgameiconcrop10pxradiusabove13-14pxname,max2lines,12pxgap. Gridinnerverticalscroll about330-360px,max55vh;13gamesareREALoptionsincluding三角洲行动/王者荣耀/和平精英/暗区突围/无畏契约/穿越火线手游/金铲铲之战/英雄联盟手游/使命召唤手游/蛋仔派对/第五人格/无畏契约手游/永劫无间手游,showfirst9+partialnextrow/naturalverticalscrollandno tinyfonttocramall13. Imageiconartonlyplacementreference;actualruntimeusesprojectofficialassets,notgeneratedcroppedart. Selectedtilethinmintoutlineandminttext; othersnoborderorlowcontrastborder. No newfeatures/giantillustrations/Englishbanners/bluepurplegradient/no extrafakeonlinecounts. FlatportraitEXACT390:844,no deviceframe/nooutside margin. Pageunderneathisexisting发布服务form,triggerits选择游戏field. Single-selectsheettitle选择游戏,三角洲行动selected. TapitemupdatesONLYserviceformlocalgameandcloses; NOconfirmfooterforSINGLEselect. Existingcoverpreviewandformunchangedbehindrgbaoverlay,nohomebottomnav. Searchemptyinitial,not蛋仔fromreference. Nofalsefieldsorremovingeditableoverlaytext.
