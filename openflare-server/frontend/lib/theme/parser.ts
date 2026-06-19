import type {Theme} from "./types"
import themesData from "./themes.json"

/**
 * 获取所有可用主题
 * 从预生成的 themes.json 中直接读取主题列表和颜色配置
 * @returns 主题列表，已按默认主题优先且其余按名称排序
 */
export async function getAvailableThemes(): Promise<Theme[]> {
  return themesData as Theme[];
}
