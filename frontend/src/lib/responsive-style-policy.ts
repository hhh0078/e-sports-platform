/** 文件作用：使用现有PostCSS解析器拦截业务样式中的固定设计px，允许原生安全区、细线、触控下限及真实位图边界。 */
import postcss, { type Rule } from 'postcss'

/** 不符合响应式单位约定的CSS声明。 */
export interface FixedPixelIssue { property: string; value: string; line: number }

/**
 * 检查常规视觉声明是否仍使用固定px，不检查真实媒体断点及注释。
 * @param styles - 独立CSS或Vue样式块正文。
 * @returns 可定位的固定像素声明，合法的设备/绘图边界不返回。
 */
export function fixedVisualPixels(styles: string): FixedPixelIssue[] {
  const issues: FixedPixelIssue[] = []
  postcss.parse(styles).walkDecls(declaration => {
    const property = declaration.prop.toLowerCase()
    const selector = declaration.parent?.type === 'rule' ? (declaration.parent as Rule).selector : ''
    if (/\.(?:poster|friend|timeline)-canvas\b/.test(selector) && ['width', 'height'].includes(property)) return
    const value = declaration.value.replace(/var\(--(?:status-bar-height|capsule-space|capsule-right-inset|floating-top)[^)]*\)/g, '')
    const invalid = [...value.matchAll(/(?<![\w.])(-?(?:\d+(?:\.\d*)?|\.\d+))px\b/g)].some(match => {
      const number = Number(match[1])
      if (Math.abs(number) === 1 && ((/^(?:border|outline)/.test(property) && !property.includes('radius')) || ['height', 'width', 'box-shadow', 'text-shadow'].includes(property))) return false
      if (['min-height', 'min-width'].includes(property) && number === 44) return false
      if (property === 'max-width' && number >= 440) return false
      if (['top', 'left', 'right', 'bottom'].includes(property) && Math.abs(number) >= 1000) return false
      return true
    })
    if (invalid) issues.push({ property, value: declaration.value, line: declaration.source?.start?.line || 1 })
  })
  return issues
}
