/** 文件作用：校验官方加载状态组件的微信选择器兼容转换真实匹配且可重复构建。 */
import test from 'node:test'
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { classBasedLoadMore } from '../../build/uni-load-more-compat'

test('官方加载状态只类化雪花图片规则且转换幂等', () => {
  const source = readFileSync(new URL('../../node_modules/@dcloudio/uni-ui/lib/uni-load-more/uni-load-more.vue', import.meta.url), 'utf8')
  assert.ok(source.includes('.uni-load-more__img--ios-H5 image'), '官方库已更新时应重审这项精确兼容')
  const transformed = classBasedLoadMore(source)
  assert.ok(transformed.includes('<image class="uni-load-more__snow-image" :src="imgBase64"'))
  assert.ok(transformed.includes('.uni-load-more__img--ios-H5 .uni-load-more__snow-image'))
  assert.equal(transformed.includes('.uni-load-more__img--ios-H5 image'), false)
  assert.equal(classBasedLoadMore(transformed), transformed)
})
