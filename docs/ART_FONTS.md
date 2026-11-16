# 艺术字体与按字加载

文件作用：记录轮播、商品/服务封面艺术字体的授权来源、裁剪部署要求、缓存及验收方法。

## 支持范围

- 管理轮播与发布/编辑服务的每条独立文案均可选字体；缺少 `font_family` 沿用系统默认。
- ServiceCard、个人主页、发布预览、详情通过共享 CoverArt 使用相同配置；首页独立文字轮播使用同一加载器。
- 系统标题、价格、认证和正文保持系统字体。文案仍为独立文本元素，不烘焙入图片。
- 草稿保存白名单包含合法字体编号；未知字体在前端回退，保存接口拒绝非白名单编号。

## 原文件和授权

六款字体为站酷快乐体、站酷庆科黄油体、站酷小薇体、Ma Shan Zheng、Long Cang、Zhi Mang Xing，均取自 Google Fonts 的 OFL 目录。

`backend/internal/artfonts/catalog.json` 是前后端共享目录，登记上游固定提交、原文件地址及 SHA256。每款 `sources/<id>/OFL.txt` 保留原作者版权和 SIL OFL 1.1 全文。子集的 OpenType name 表也携带原版权与完整许可证；字体不得单独售卖，修改与再分发继续遵守 OFL。

字体源文件仅嵌入服务端二进制，不注册源字体静态路由，不进入 H5 或小程序资源包。前端只导入小型JSON目录。更新源字体、依赖版本或裁剪规则时须递增目录 `version`，使浏览器缓存失效。

## 部署和本地运行

后端继续使用 Go，额外通过无 Shell 的固定 Python 子进程调用 FontTools。Dockerfile 已使用国内镜像安装 `backend/font-requirements.txt` 的固定版本依赖，配置 `FONT_SUBSET_PYTHON=/opt/font-subset/bin/python`。容器保持非root和只读根文件系统，不需要写字体缓存目录。

本机开发可复用独立虚拟环境，避免污染系统Python：

```sh
python3 -m venv .run/art-font-venv
.run/art-font-venv/bin/pip install -i https://pypi.tuna.tsinghua.edu.cn/simple -r backend/font-requirements.txt
export FONT_SUBSET_PYTHON="$PWD/.run/art-font-venv/bin/python"
```

再按已有方式启动Go。若不设置环境变量，后端使用PATH中的 `python3`；需确保该解释器已安装对应FontTools。发布时须同步升级后端镜像与前端，旧后端没有字体路由。

## 加载协议

匿名GET `/api/fonts/<version>/<id>.woff?text=<实际去重字形>` 返回 WOFF，兼顾微信与旧iOS；不从外部字体CDN加载，不携带登录令牌。响应包含 `font/woff`、公开跨域、ETag、一年不可变缓存。

- 每个封面按选中字体合并当前文案字形；系统字体、空文案不请求。列表卡片进入视口附近才加载；隐藏页面不发起新加载。
- 轮播只准备当前和下一张字幕，单次最多144码点，包含两张各六条十二字文案；前后端排序去重，同字集复用同一资源。
- 子集只包含用到的Unicode字形、字体格式必需表、字形依赖及授权信息。不能把“按字加载”理解为文件中完全没有格式或授权开销。
- JS请求索引最多256条；浏览器与微信使用资源缓存。服务端最多缓存256项、合计16MiB；两个裁剪进程，最多16个待处理任务，20秒超时。资源不足返回可重试503，错误响应不缓存。
- 加载完成才切换艺术字；加载中、网络失败、未知字体或字体不收录的罕见字/表情使用系统字体。旧文字或旧选择的迟到结果不会恢复到当前组件。
- 字体文件需从正式HTTPS地址正常访问，公开跨域头不能被反向代理移除。微信真机字体显示、正式域名和不同系统版本需独立验证，构建通过不代表真机验收。

## 验证

按项目规范执行类型检查、规范检查、前端测试、H5与微信构建。后端执行：

```sh
cd backend
FONT_SUBSET_PYTHON="$PWD/../.run/art-font-venv/bin/python" go test ./...
```

字体专项测试对六款真实字体分别请求“限时优惠”，重开输出字体检查cmap恰为这四个码点、保留授权、文件显著缩小、ETag返回304，并拒绝无文案、超长字集及任意原文件地址。浏览器验收截图在 `output/playwright/art-fonts/`；接口夹具与真实字体请求需明确区分，不得伪装成生产或微信真机结果。
