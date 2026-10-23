/** 文件作用：保留官方uni-load-more，仅把其雪花图片的标签选择器转为微信组件支持的类选择器。 */
/** 转换官方组件的同一图片与规则，不修改node_modules或其它组件。 */
export function classBasedLoadMore(source: string): string {
  return source.replace('<image :src="imgBase64"', '<image class="uni-load-more__snow-image" :src="imgBase64"').replace('.uni-load-more__img--ios-H5 image', '.uni-load-more__img--ios-H5 .uni-load-more__snow-image')
}
