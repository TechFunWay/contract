// 人民币小写金额 → 中文大写（壹贰叁… / 拾佰仟万亿 / 角分整）。
// 用于合同填写页「金额大写」字段的自动转换：输入小写金额即时生成大写。

const DIGITS = ['零', '壹', '贰', '叁', '肆', '伍', '陆', '柒', '捌', '玖']
const SMALL_UNITS = ['', '拾', '佰', '仟']
const BIG_UNITS = ['', '万', '亿', '兆']

// 0 ≤ n < 10000 的一段数字转大写
function sectionToCn(n: number): string {
  let rest = n
  let out = ''
  let unit = 0
  let pendingZero = false
  while (rest > 0) {
    const d = rest % 10
    if (d === 0) {
      if (out !== '') pendingZero = true
    } else {
      if (pendingZero) {
        out = DIGITS[0] + out
        pendingZero = false
      }
      out = DIGITS[d] + SMALL_UNITS[unit] + out
    }
    rest = Math.floor(rest / 10)
    unit += 1
  }
  return out
}

// 正整数转大写，按 4 位分节（个/万/亿/兆），段间不足四位补「零」
function intToCn(n: number): string {
  if (n === 0) return DIGITS[0]
  const groups: number[] = []
  let rest = n
  while (rest > 0) {
    groups.push(rest % 10000)
    rest = Math.floor(rest / 10000)
  }
  let out = ''
  for (let i = groups.length - 1; i >= 0; i -= 1) {
    const g = groups[i]
    if (g === 0) {
      // 中间整段为 0：后面还有非零段时补一个「零」
      if (out !== '' && !out.endsWith(DIGITS[0]) && groups.slice(0, i).some((x) => x > 0)) {
        out += DIGITS[0]
      }
      continue
    }
    // 非首段且该段不足四位：先补「零」再接段值
    if (i !== groups.length - 1 && out !== '' && g < 1000 && !out.endsWith(DIGITS[0])) {
      out += DIGITS[0]
    }
    out += sectionToCn(g) + BIG_UNITS[i]
  }
  return out.replace(/零{2,}/g, DIGITS[0]).replace(/零+$/, '') || DIGITS[0]
}

/**
 * 把小写金额转成中文大写。
 * - 允许逗号、空格、￥/¥/元 前后缀，超过 2 位小数按四舍五入取到分
 * - 返回示例：100 → 壹佰元整；100.5 → 壹佰元伍角整；100.05 → 壹佰元零伍分
 * - 非法输入、负数或超出可转换范围（≥ 1 兆）返回空串，由调用方决定如何处理
 */
export function amountToCnyUppercase(input: string | number): string {
  const raw = String(input ?? '')
    .trim()
    .replace(/[,，\s￥¥元]/g, '')
  if (!raw || !/^\d+(\.\d+)?$/.test(raw)) return ''
  const num = Number(raw)
  if (!Number.isFinite(num)) return ''
  const cents = Math.round(num * 100)
  if (cents <= 0) return cents === 0 ? `${DIGITS[0]}元整` : ''
  if (cents >= 1e16) return ''

  const yuan = Math.floor(cents / 100)
  const jiao = Math.floor((cents % 100) / 10)
  const fen = cents % 10
  const base = yuan > 0 ? `${intToCn(yuan)}元` : ''

  if (jiao === 0 && fen === 0) return `${base}整`
  if (jiao === 0) return fen > 0 ? `${base}${DIGITS[0]}${DIGITS[fen]}分` : ''
  if (fen === 0) return `${base}${DIGITS[jiao]}角整`
  return `${base}${DIGITS[jiao]}角${DIGITS[fen]}分`
}
