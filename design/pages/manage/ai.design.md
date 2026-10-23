# AI 设置 · 管理独立页

文件作用：对应 `frontend/src/pages/manage/ai.vue` 的先行390×844设计与真实业务边界。

- 仅平台管理员可读写，冷启动错误在身份分支外显示重试，已知访客保留权限提示。GET/PATCH /admin/ai-settings返回平铺base_url/model/prompt/has_api_key/ready/mode/configuration_notice；只复制公开字段，密钥永不回显或持久化。
- 默认兼容接口地址https://maas.qianwenaiapi.com/compatible-mode/v1，模型严格为qwen3.8-flash；不声称该域名为千问官方。Base URL最长2048，公网HTTPS标准443，无用户名、查询及片段；model1—120字且无换行；prompt20—12000字；api_key最多4096字节且无换行。图中生成提示字形以此真实契约为准。
- API Key密码框留空保留，保存/取消/隐藏页面清空本次输入；请求秘密在完成后擦除。读取/保存/测试均有同账号和页面版本保护。
- 本地模拟、未配置密钥、已配置AI分别明确显示。ready/mode ai只是配置完整，不能当成真实连通或具备图像审核能力。后端POST /admin/ai-settings/test {}使用已保存配置及固定安全文本，只有真实结果才展示；无密钥不能伪造成功，配置未保存先提示保存。结果全文和失败原因正常换行。
- 主提交保存AI配置44px；测试连接36px辅助，取消修改文字。提示词textarea及长内容自然滚动；input44px透明底线，16px边距细分隔。无管理员发消息表单混在本页。

## 完整生成与局部编辑提示词

Use case: ui-mockup. One professional Chinese WeChat Mini Program UI,exact390:844flatportrait,no frame/nooutside margins. Same established 开机电竞 darknaturalblackgreen #0a1110,white#edf4ef,muted#9dafa3,mint#8de8b6. Sharedsecondarybackarrow/title beneathnativeWeChatstatus/capsulesafearea. NO primarybottomnav. 16pxinsets,24pxsectiongaps,thin separators,officialcenteredicons,body16px/secondary14px,openeditorial layout,44pxprimarysubmit and inputs,36pxcompactactions,textarea lowgray8pxradius. NO nestedpanels,giantcards,dashedemptyframes,illustrations,Englishdecoration orbluepurplegradient. Alltext real UI elements, designisnotbackground. TitleAI 设置. Oneopen adminsettingsform. At topquietstatusrow outlinedchip 本地模拟 withsmallinfoicon,secondary 本地测试不会调用真实模型. Do notclaimrealAIreadywhenkeymissing. Section模型连接: Base URL fieldtransparentbottomline44px contains https://maas.qianwenaiapi.com/compatible-mode/v1 (longURLinputhorizscrollnotwraptooverflowscreen),模型 field qwen3.8-flash. API Key fieldmaskedpassword empty withplaceholder 留空保留已保存密钥;smallright 未配置 status, no actualsecret. Belowrightquiet36px测试连接 action (actualbackendtestingonly,nofakepassed). Thinsectionseparator. Section审核提示词 withsmallinfoicon andlowgraytextareaabout156px showing genericinstruction 仅依据平台规则审核服务内容，明确指出风险与修改建议。 Prompteditable fulltext,safevalidationmax documentedlater. Contentlongnaturalscroll; firstscreenmayendnearbottomoftextarea. Bottomsingle44px保存AI配置 andquiet取消修改text canappearbelowfold ifneeded,donotshrink. Pageonlyconfigandtest,nomanualmessagespanel,nobroadcastfields. Adminonly,localmock/missingkey/productioncapabilitydistinguishedasrealstate. No secretoutput. No giantstatusbanner.

Use case: ui-mockup. Preciselocaledit attached390x844AI设置form. Preserveexactapprovedlayout,theme,allfields/spacing/buttons/header. CorrectONLYactualbusinesslabels: modelplaintextmustbe EXACT qwen3.8-flash withDOTbetween3and8,remove dropdownarrow sinceplaininput. Promptcountermustbe 0 / 12000 not2000,helpersmall 提示词20—12000字. Sectionlabel模型连接改为兼容接口. Bottombutton保存AI配置unchanged. Addtinyhelperbelow测试连接 测试仅使用已保存配置, no inventedofficial千问claims/no realsecret. Nativecapsulewhitespace. Flat390:844,no frame.
