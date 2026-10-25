# 微信新版本准备完成

文件作用：在已批准的首页设计上先行生成微信原生更新弹框状态，约390×844；图片由内置ImageGen局部编辑，再机械等比缩小，不是运行验收截图。

首页与业务页面原版式不改变。版本检查仅微信小程序启动时绑定一次官方getUpdateManager；H5不调用。发现更新由微信下载，下载成功且当前前台、资金业务结束、没有其它系统弹框时，使用微信原生showModal，标题“发现新版本”，正文“新版本已准备好，重启后即可使用。当前资金操作结束后再更新。”，唯一按钮“立即更新”，showCancel:false。确认后再次核验资金任务与前台状态，再调用applyUpdate。

资金任务覆盖请求发起、原生微信付款／商家转账、按原单查款、退款与审核转账请求。原生支付引发隐藏时不得提前释放尚未结束任务；未知结果保留既有服务端原单及本地业务号，不删除任何恢复缓存。结束后仅调度下一事件循环检查更新，不在付款回调中同步重启。

公告和更新通过同一系统弹框锁协调：已有公告关闭后再提示更新；更新已准备且公告尚未打开时，优先更新。下载、弹框或applyUpdate异常使用轻提示并释放业务协调状态，不锁住正常浏览、发布或交易。后台不弹更新或重启，回前台重新检查；同一下载事件不得反复绑定或重复弹框。

验收重点：ready期间正在付款／退款时无更新弹框；原生回调之后仍在查原单时不重启；用户确认后若又有资金任务开始则延期；公告已打开不覆盖；后台不弹、不重启；下载失败不锁业务；H5没有原生更新调用。微信真实版本下载、applyUpdate重启需真机新版本体验验证，mock和构建不能替代。

生成提示词：Use case ui-mockup, precise local UI state edit of approved player.content-v2 home screenshot. Preserve the complete dark natural-green home layout and official game artwork. Add only a standard native WeChat white update modal over neutral translucent black dimming; Chinese title发现新版本, content新版本已准备好，重启后即可使用。当前资金操作结束后再更新。, one full-width green text button立即更新, no cancel, close icon or illustration, no blue-purple gradient; flat390×844.
