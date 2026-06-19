import PinyinMatch from 'pinyin-match'

type MatchResult = [number, number] | false;
type MatchFunction = (input: string, keys: string) => MatchResult;

// Handle CJS/ESM interop for pinyin-match
const match = ((): MatchFunction | null => {
  const p = PinyinMatch as unknown;
  if (!p) return null;

  // Check for .default.match (common in some ESM bundles)
  const withDefault = p as { default?: { match?: MatchFunction } };
  if (typeof withDefault.default?.match === 'function') {
    return withDefault.default.match;
  }

  // Check for .match (defined in its typings)
  const withMatch = p as { match?: MatchFunction };
  if (typeof withMatch.match === 'function') {
    return withMatch.match;
  }

  // Check if it's the function itself
  if (typeof p === 'function') {
    return p as MatchFunction;
  }

  // Fallback
  return null;
})();

export interface SearchItem {
  id: string
  title: string
  description: string
  url: string
  category: 'page' | 'feature' | 'setting' | 'admin'
  keywords: string[]
  icon?: string
  matchRange?: [number, number]
}

/**
 * 全局搜索数据源
 * 包含所有可搜索的页面和功能
 */
export const searchData: SearchItem[] = [
  // ==================== 总览 ====================
  {
    id: 'home',
    title: '总览',
    description: '返回控制台总览',
    url: '/',
    category: 'page',
    keywords: ['home', '主页', '首页', 'dashboard', '总览'],
  },

  // ==================== 文档库 ====================
  {
    id: 'docs-api',
    title: '开发接口文档',
    description: '查看 RESTful API 接口规格定义',
    url: '/docs/api',
    category: 'page',
    keywords: ['api', 'docs', '文档', '接口', 'specification'],
  },
  {
    id: 'docs-how-to-use',
    title: '使用帮助文档',
    description: '查看新手教程和集成示例',
    url: '/docs/how-to-use',
    category: 'page',
    keywords: ['docs', '文档', '使用', 'how to', 'tutorial', '教程', 'help'],
  },

  // ==================== 个人设置 ====================
  {
    id: 'settings',
    title: '全局设置',
    description: '配置应用个人偏好选项',
    url: '/settings',
    category: 'setting',
    keywords: ['settings', '设置', '偏好', 'preferences'],
  },
  {
    id: 'settings-profile',
    title: '我的资料',
    description: '编辑昵称、头像和个人属性',
    url: '/settings/profile',
    category: 'setting',
    keywords: ['profile', '资料', '个人', '我的', '信息', 'avatar'],
  },
  {
    id: 'settings-appearance',
    title: '外观设置',
    description: '配置系统显示主题（亮色/暗色）',
    url: '/settings/appearance',
    category: 'setting',
    keywords: ['appearance', '外观', '主题', 'theme', 'dark', 'light'],
  },
  // ==================== 管理员 ====================
  {
    id: 'admin-settings',
    title: '系统设置',
    description: '管理系统登录注册与认证源配置 (管理员专属)',
    url: '/admin/settings',
    category: 'admin',
    keywords: ['admin', '管理员', '系统设置', '安全', 'security', 'oidc', 'login'],
  },
  {
    id: 'admin-system',
    title: '系统配置',
    description: '动态修改平台核心运行时配置 (管理员专属)',
    url: '/admin/system',
    category: 'admin',
    keywords: ['admin', '管理员', '系统', '配置', 'system', 'configurations'],
  },
  {
    id: 'admin-users',
    title: '用户管理',
    description: '管理平台注册用户的活跃状态 (管理员专属)',
    url: '/admin/users',
    category: 'admin',
    keywords: ['admin', '管理员', '用户', '管理', 'users', 'status'],
  },
  {
    id: 'admin-tasks',
    title: '异步任务管理',
    description: '下发与排查后台异步定时任务 (管理员专属)',
    url: '/admin/tasks',
    category: 'admin',
    keywords: ['admin', '管理员', '任务', '异步', 'tasks', 'scheduler', 'worker'],
  },
]

/**
 * 搜索功能
 * @param query 搜索关键词
 * @param isAdmin 是否为管理员
 * @returns 匹配的搜索结果
 */
export function searchItems(query: string, isAdmin: boolean = false): SearchItem[] {
  const trimmedQuery = query.trim()
  
  // 非管理员不能搜索 admin 类别项
  const filteredData = isAdmin 
    ? searchData 
    : searchData.filter(item => item.category !== 'admin')

  if (!trimmedQuery) {
    return filteredData
  }

  return filteredData.map(item => {
    // 优先匹配标题
    const titleMatch = typeof match === 'function' ? match(item.title, trimmedQuery) : null
    if (titleMatch) {
      return { ...item, matchRange: titleMatch as [number, number] }
    }

    // 匹配描述
    if (typeof match === 'function' && match(item.description, trimmedQuery)) {
      return item
    }

    // 匹配关键词
    if (item.keywords.some(keyword => typeof match === 'function' && match(keyword, trimmedQuery))) {
      return item
    }

    return null
  }).filter((item): item is SearchItem => item !== null)
    .sort((a, b) => {
      // 标题匹配优先
      if (a.matchRange && !b.matchRange) return -1
      if (!a.matchRange && b.matchRange) return 1
      
      // 如果都是标题匹配，按匹配位置排序
      if (a.matchRange && b.matchRange) {
        return a.matchRange[0] - b.matchRange[0]
      }
      
      return 0
    })
}
