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
    id: 'docs-how-to-use',
    title: '使用帮助文档',
    description: '查看新手教程和集成示例',
    url: 'https://open-flare.pages.dev/',
    category: 'page',
    keywords: ['docs', '文档', '使用', 'how to', 'tutorial', '教程', 'help'],
  },

  // ==================== 业务控制台 ====================
  {
    id: 'console-nodes',
    title: '节点管理',
    description: '管理边缘节点、中继节点与内网穿透通道',
    url: '/nodes',
    category: 'page',
    keywords: ['node', '节点', '边缘节点', '中继', '内网穿透', 'tunnel', '服务器'],
  },
  {
    id: 'console-proxy-routes',
    title: '规则管理',
    description: '配置反向代理、路由匹配规则、WAF 策略与缓存设置',
    url: '/proxy-routes',
    category: 'page',
    keywords: ['route', '规则', '路由', '代理', '反向代理', 'proxy'],
  },
  {
    id: 'console-websites',
    title: '域名列表',
    description: '管理托管域名及证书绑定与监听配置',
    url: '/websites',
    category: 'page',
    keywords: ['website', 'domain', '网站', '域名', '站点'],
  },
  {
    id: 'console-certificates',
    title: 'TLS 证书',
    description: '申请与管理 SSL/TLS 证书，支持自动续期',
    url: '/certificates',
    category: 'page',
    keywords: ['certificate', 'ssl', 'tls', '证书', 'https', '加密'],
  },
  {
    id: 'console-dns-accounts',
    title: 'DNS 账号',
    description: '配置 DNS 服务商 API 凭证以自动申请证书及管理解析',
    url: '/dns-accounts',
    category: 'page',
    keywords: ['dns', 'dns account', '账号', '域名解析', 'cloudflare', 'aliyun', 'tencent'],
  },
  {
    id: 'console-origins',
    title: '源站地址',
    description: '管理反向代理的目标后端服务器与负载均衡组',
    url: '/origins',
    category: 'page',
    keywords: ['origin', '源站', '后端', 'backend', '服务器', '负载均衡'],
  },
  {
    id: 'console-waf',
    title: 'WAF 防火墙',
    description: '配置 Web 应用防火墙规则，阻断恶意请求',
    url: '/waf',
    category: 'page',
    keywords: ['waf', '防火墙', '安全', 'security', '拦截', '规则'],
  },
  {
    id: 'console-ip-groups',
    title: 'IP 组',
    description: '定义 IP 地址列表以在 WAF 或路由中实现黑白名单控制',
    url: '/ip-groups',
    category: 'page',
    keywords: ['ip', 'ip group', 'ip组', '黑名单', '白名单', '访问控制'],
  },
  {
    id: 'console-pages',
    title: 'Pages 静态托管',
    description: '上传或部署静态网页，提供全球 CDN 加速托管',
    url: '/pages',
    category: 'page',
    keywords: ['pages', '静态托管', 'cdn', '网站', '部署', 'static'],
  },
  {
    id: 'console-config-versions',
    title: '版本发布',
    description: '查看、对比、发布与回滚系统配置版本',
    url: '/config-versions',
    category: 'page',
    keywords: ['version', 'config', '版本', '发布', '回滚', '对比', '部署'],
  },
  {
    id: 'console-access-logs',
    title: '访问日志',
    description: '查看并检索全量网站访问请求日志与网络分析数据',
    url: '/access-logs',
    category: 'page',
    keywords: ['log', 'logs', '访问日志', '分析', '流量', '请求'],
  },
  {
    id: 'console-apply-logs',
    title: '应用记录',
    description: '查看节点配置下发、同步与生效的历史记录',
    url: '/apply-logs',
    category: 'page',
    keywords: ['apply', 'log', 'logs', '应用记录', '配置下发', '同步', '部署历史'],
  },
  {
    id: 'console-performance',
    title: '性能调优',
    description: '调优网络连接、代理超时与核心系统性能参数',
    url: '/performance',
    category: 'page',
    keywords: ['performance', '性能', '调优', '优化', '参数', '连接', '超时'],
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
  {
    id: 'admin-files',
    title: '存储管理',
    description: '查看、检索与清理上传到对象存储中的文件 (管理员专属)',
    url: '/admin/files',
    category: 'admin',
    keywords: ['admin', '管理员', '存储', '文件', 'files', 'upload', 's3'],
  },
  {
    id: 'admin-database',
    title: '数据管理',
    description: '监控数据库表大小、分页浏览物理表内容并支持交互式 SQL (管理员专属)',
    url: '/admin/database',
    category: 'admin',
    keywords: ['admin', '管理员', '数据库', 'database', 'sql', 'query', 'gorm'],
  },
  {
    id: 'admin-push',
    title: '通知推送',
    description: '配置与下发邮件、Lark 和 Telegram 渠道通知推送 (管理员专属)',
    url: '/admin/push',
    category: 'admin',
    keywords: ['admin', '管理员', '推送', '通知', 'push', 'mail', 'telegram', 'lark'],
  },
  {
    id: 'admin-logs',
    title: '系统日志',
    description: '查看系统日志与后台异步任务执行日志 (管理员专属)',
    url: '/admin/logs',
    category: 'admin',
    keywords: ['admin', '管理员', '日志', 'logs', 'system log', 'terminal'],
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
