<!-- 文件作用：启动时恢复平台状态，准备微信登录票据，并加载统一设计变量。 -->
<script lang="ts">
import { init, initPublic, prepareWechatLogin } from './lib/store'
import { isTimelinePreview } from './lib/sharing'
export default {
  /** 启动后获取官方临时身份，建立会话仍需用户明确同意协议。 */
  onLaunch(): void {
    if (isTimelinePreview()) { initPublic().catch(() => {}); return }
    init().then(() => {
      // #ifdef MP-WEIXIN
      uni.login({ provider: 'weixin', success: result => { if (result.code) prepareWechatLogin(result.code).catch(() => {}) } })
      // #endif
    }).catch(() => {})
  },
}
</script>
<style>
@import './styles/global.css';
</style>
