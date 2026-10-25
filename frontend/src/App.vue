<!-- 文件作用：启动时恢复平台状态，准备微信登录票据，并加载统一设计变量。 -->
<script lang="ts">
import { init, initPublic, prepareWechatLogin } from './lib/store'
import { isTimelinePreview } from './lib/sharing'
import { initializeUpdateManager, setUpdateAppVisible } from './lib/update-manager'
export default {
  /** 启动后获取官方临时身份，建立会话仍需用户明确同意协议。 */
  onLaunch(): void {
    // #ifdef MP-WEIXIN
    initializeUpdateManager()
    // #endif
    if (isTimelinePreview()) { initPublic().catch(() => {}); return }
    init().then(() => {
      // #ifdef MP-WEIXIN
      uni.login({ provider: 'weixin', success: result => { if (result.code) prepareWechatLogin(result.code).catch(() => {}) } })
      // #endif
    }).catch(() => {})
  },
  /** 前台才允许提示更新，付款或转账往返不清除尚未结束的资金锁。 */
  onShow(): void {
    setUpdateAppVisible(true)
  },
  /** 后台停止更新提示和重启，保持原单查询恢复能力。 */
  onHide(): void {
    setUpdateAppVisible(false)
  },
}
</script>
<style>
@import './styles/global.css';
</style>
