# 滚动稳定性修复与测量

文件作用：记录2026-10-23本轮滚动修复、自动化覆盖及官方微信工具的手势性能复核方法；不以静态检查或程序滚动冒充真机流畅度。

## 实际改动

- 四个常驻主视图继续使用原`scroll-view`和`v-show`，删除`@scroll → reactive位置 → scroll-top`循环。浏览滚动不再向原生容器持续发送位置属性；切标签依靠常驻容器保存原位置。
- 服务卡片共用一个窗口宽度及一个原生窗口监听；同轮组件挂载和页面显示合并读取窗口。最后一个组件卸载后移除监听。
- 封面仅在组件挂载、所属视图激活且页面显示时测量；同轮测量合并。隐藏、离页、后续尺寸请求会使旧异步结果失效。H5容器观察器随可见性暂停。
- 列表封面使用原生图片懒加载，封面几何占位不依赖图片加载完成。原防碰撞算法、字号下限和自定义文字图层保持原逻辑。
- 俱乐部图文按服务端可信`image_width`／`image_height`提前预留高度；没有尺寸的旧块固定16:9占位，以`aspectFit`完整显示并保留点击看原图。图片加载、失败和替换不改变占位。

## 自动化证据

运行：`frontend/node_modules/.bin/tsx --test frontend/src/lib/cover-art.test.ts frontend/src/lib/cover-readable.test.ts frontend/src/lib/shared-responsive.test.ts frontend/src/lib/scroll-performance.test.ts frontend/src/lib/navigation-public.test.ts`

17项通过，0失败。覆盖100个卡片使用者仅一个窗口监听和一次同轮原生读取；隐藏零测量；恢复可见后重新测量；迟到结果丢弃；四个主容器无滚动回写；可信和异常图片尺寸占位；320／390宽的既有文字可读、防重叠及押金预留边界。

## 本轮新增官方SDK程序滚动检查

`scripts/mini-background-smoke.ts`已补充非零位置验收，等待主任务在冻结产物后统一运行：

- 每个主题逐个检查四个主视图，先读真实可滚范围；长内容滚到真实非零位置，然后真实点击相邻底栏再返回。往返期间不重新设置`scrollTop`，检查原Page仍在且位置偏差不超过1px。
- 访客短空态无可滚范围时记录`not_scrollable`，不能冒充非零位置通过；每个主题至少需要一个真实长视图，否则明确失败。
- 响应式服务样本从真实非零列表位置点击服务卡片，详情检查后使用官方`navigateBack()`返回；不能`reLaunch`重建列表，也不能回到列表后先滚到0再检查。
- 报告新增`scroll_retention_measurements`，包含类型、主题/标签、可滚范围、往返前后位置、差值与真实状态；失败保留测量值。新增原图为`*-scroll-retained.png`与`*-service-return-scroll-retained.png`。

复用命令（设备尺寸和9423工程由主任务先准备；分别执行320／390）：

```sh
UI_TEST_PROJECT=/Users/Zhuanz/Desktop/三角洲/frontend/dist/dev/mp-weixin \
UI_TEST_OUTPUT_ROOT=output/post-review-scroll \
UI_TEST_RESPONSIVE=1 UI_EXPECT_WIDTH=390 \
frontend/node_modules/.bin/tsx scripts/mini-background-smoke.ts
```

输出为`output/post-review-scroll/mini-320/report.json`或`mini-390/report.json`及同目录官方原图。该脚本仍只进行公开GET和访客操作；`UI_TEST_PUBLIC_SERVICES=0`会略过服务样本及详情返回检查，报告不能声称该项通过。本节记录脚本能力，不表示本轮已运行成功；它也不测量快速手势惯性或FPS。

## 官方微信复核方法

1. 源码和编译产物冻结后固定同一微信开发者工具工程。分别用320和390宽，记录工具、基础库和机器；原生真机另记设备和系统。
2. 同主题、同排序准备24和72条服务，分别检查空标签和6条长标签。先测冷图，再测缓存图；记录网络条件，不混用H5结果。
3. 官方性能面板开始记录，连续向下快速滑动3次、向上3次，等待惯性停止，再切四个标签并返回。比较逻辑层任务、渲染耗时及跨层数据更新；纯滚动期间不应持续出现`scroll-top`更新。
4. 记录滑动前后位置及原始视频；打开详情再返回、切标签再回来，应保持合理浏览位置且没有主动回顶部。官方SDK的`scrollTo()`可检查几何及返回位置，不能替代真实快速手势或帧率证据。
5. 在俱乐部图文页开启冷图或慢网络，滑过尚未完成加载的图文块；加载前后取同一个下方标题坐标及文档高度，图片本身不应再推动内容。旧图片采用稳定占位，点击原图应仍完整展示。
6. 分开观察内容区中途和顶部／底部：系统边界弹性、输入键盘变化、图片布局移动与滚动反馈需分开归因，不能通过全局关闭正常滚动能力掩盖异常。

当前本模块只完成代码和17项自动化检查，未运行本轮官方SDK、性能面板手势录制、双端构建或真机。全项目验收由主任务统一执行并记录；不能将本记录描述为已测得帧率提升。
