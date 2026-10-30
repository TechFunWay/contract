// 下拉选项的解析（文本 → 选项数组）与回显（选项数组 → 文本），
// 字段设计器的选项输入框只用这两个函数。
//
// 分隔符：顿号「、」、半角/全角逗号、任意空白（空格 / 换行 / 制表符）。
// 选项本身要含这些字符时用引号包住：半角 "..." '...' 或中文输入法的 “...”。
//
// 背景（2026-10-30 粉丝反馈「下拉选择，填写没法空格和填逗号」）：旧实现在每个
// 按键后都拿 options.join('、') 回填输入框，分隔符一敲进去就被吃掉——
// 敲 "A, B, C" 最后输入框里只剩 "ABC"。所以调用方必须把用户敲的原始文本留在
// 输入框里，只把这里的解析结果向上抛，绝不能反过来用回显值覆盖输入中态。
//
// 两个函数必须满足往返恒等：splitOptions(joinOptions(x)) 与 x 等价，
// 否则「组件本地文本 ↔ 外部选项数组」的同步判等会误触发、把用户敲的文本覆盖掉。

/** 把用户输入的原始文本解析成选项数组；空白段与多余分隔符自然丢弃。 */
export function splitOptions(raw: string): string[] {
  if (!raw) return []
  const out: string[] = []
  // 引号包住的一段整体算一个选项（保留其中的空格与逗号）；
  // 其余片段遇到分隔符即切开。用 exec 循环保留书写顺序。
  const re = /"([^"]*)"|'([^']*)'|“([^”]*)”|[^、,，\s]+/g
  let m: RegExpExecArray | null
  while ((m = re.exec(raw)) !== null) {
    const quoted = m[1] ?? m[2] ?? m[3]
    const text = quoted !== undefined ? quoted : m[0]
    if (text.trim()) out.push(text)
  }
  return out
}

const HAS_SEPARATOR = /[、,，\s]/

/** 把选项数组拼回输入框里的回显文本（分隔符统一用顿号）。 */
export function joinOptions(options?: string[] | null): string {
  return (options || []).map(quoteIfNeeded).join('、')
}

// 选项本身含分隔符时必须加引号包起来，否则回显的文本下次再解析会被切开
// （选项「A、B」若原样回显成 A、B，重开编辑器一保存就裂成两个选项）。
function quoteIfNeeded(option: string): string {
  if (!HAS_SEPARATOR.test(option)) return option
  const pairs: [string, string][] = [
    ['"', '"'],
    ['“', '”'],
    ["'", "'"],
  ]
  const pair = pairs.find(([open, close]) => !option.includes(open) && !option.includes(close))
  // 三对引号都被占满的极端选项原样回显：解析行为与旧版一致，只影响显示。
  return pair ? `${pair[0]}${option}${pair[1]}` : option
}
