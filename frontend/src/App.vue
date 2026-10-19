<!-- 文件作用：客户端生命周期及全局样式入口。 -->
<script>
import { init, prepareWechatLogin } from './lib/store'
export default {
  /** 启动时恢复身份并加载平台游戏配置。 */
  onLaunch() { init().then(() => {
    // #ifdef MP-WEIXIN
    // 静默获取微信临时身份；不创建账号、不跳过协议、不影响访客浏览。
    uni.login({ provider: 'weixin', success: result => { if (result.code) prepareWechatLogin(result.code).catch(() => {}) } })
    // #endif
  }).catch(() => {}) },
}
</script>
<style>@import './styles/global.css';</style>
