# 钱包管理 · 提现审核与公开转账配置

文件作用：对应新增 `frontend/src/pages/manage/wallet.vue` 的先行设计与真实后端契约依据；本图先生成保存，随后才实现页面。

- 统一二级返回“钱包管理”，无主底栏；开放分节与细分隔。提现列表默认待审核，无数据用轻空态；图中不伪造用户或资金记录。
- 接口只支持pending_review/processing/wait_user_confirm/success/failed/rejected或all。对应六个受控筛选：待审核/处理中/待确认/成功/失败/已拒绝，后三个是终态，不能向服务端发送虚构terminal筛选。
- 列表使用真实user_id（没有昵称不能编造），实际整数分金额、状态/时间与失败原因。仅pending_review显示审核拒绝（原因2～500字）及审核并发起；其他处理中状态只可按原申请同步，终态不可再操作。
- 所有发起/同步转账执行前弹层展示真实用户ID和金额摘要；开发模式才显示模拟成功/失败选择并发送dev_outcome，生产不出现该参数或假成功。外部未知结果保持冻结，不由本地按钮解冻。
- 管理公开配置严格为withdrawal_enabled、transfer_scene_id、transfer_scene_report_infos[{info_type,info_content}]、transfer_notify_url、transfer_remark、user_recv_perception。字段使用底线44px/低对比textarea，报备行最多10项、名称/内容各1～32字，场景编号最多32字，说明32字、收款感知10字，通知URL为无查询参数HTTPS。
- GET/PATCH配置均返回{settings,transfer_ready,configuration_notice}，保存草稿不意味着微信商户场景已获批。只有界面真实就绪状态由返回值决定。
- 支付密钥/证书沿用既有支付配置，只给跳转入口，不读取/缓存/展示秘密、OpenID、实名密文或收款确认package。
- 管理员身份、页面隐藏、切换筛选/账号的旧读取和旧写入回调需隔离；禁用进行中按钮和重复提交，错误保留可修复草稿，取消恢复已保存公开配置。
- 冷启动身份初始化失败时在权限分支外显示错误和“重新加载”，从真实init重试；管理员列表/公开配置错误仍分别在原位置重试，不把未知身份误判为已知访客。

## 完整生成提示词

Use case: ui-mockup. NEWChineseWeChatminiprogrampagewalletmanagement,390x844logicalportrait,flatUIonly,noframe/nooutside margins. Matchapproved集结editorialdark#0a1110white#edf4efmutedgraygreen#9dafA3mint#8de8b6,16pxhorizontalinsets,24pxopensections,thinlines,no largeoutercards,nobox-on-box,no artwork/no Englishdecor/no purplebluegradients. Ordinarysharedbackheader 钱包管理, nativeWeChatstatus/capsulesafearea,no mainbottomtabs. Firstsection提现申请 withquiet刷新text. Boundedhorizontalfilterchips 待审核 处理中 待确认 成功 失败 已拒绝, firstactive; final3areactualterminalfilters,notfakeaggregateAPI. Emptydefaultstate 暂无待审核提现 withsmallhelp用户提交申请后在此审核,nomockpayoutrows,nofakecashfigures. Populatedactualrowswouldshowuserid(notinventnickname),integer-centsformattedamount,status/time,onlyuninitiatedpendingcan拒绝;executionconfirmationmustshowactualuser/amountbeforeexternaltransfer. Belowthinline商家转账配置 withsmall未就绪statusfromserver. Staticnote 保存配置不代表微信场景已获批. FormOPENnotouterpanels:启用真实提现 switch(legitimateadminconfiguration);商家转账场景编号transparentunderlined44pxinputplaceholder填写获批场景编号,NOinventedofficialcode. 转账场景报备字段 simplekey/valuepairrows/info_type+info_content,quiet添加字段/删除,max10rows. Below转账通知HTTPS地址underlinedinput,转账备注max32,收款感知描述max10, maycontinuebelowfirstscreen. Quiet 商户密钥与证书沿用支付配置→link,neverkeys. One44px保存转账配置and取消修改quiettextatpageend. No bankaccount/newcreditfields,no falseapprovebuttonswhenempty. Transferfailure/configinsufficientshowhonestinlineerror;neverclient-marksuccess/locallyunfreezeexternalpendingfunds.
