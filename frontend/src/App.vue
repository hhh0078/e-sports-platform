<!-- 文件作用：客户端生命周期及全局样式入口。 -->
<script>
import { init } from './lib/store'
export default {
  /** 启动时恢复身份并加载平台游戏配置。 */
  onLaunch() { init().catch(() => {}) },
}
</script>
<style>@import './styles/global.css';</style>
