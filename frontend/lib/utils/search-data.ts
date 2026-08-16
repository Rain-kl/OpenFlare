import PinyinMatch from 'pinyin-match';

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

export interface SearchItemSource {
  id: string;
  titleKey: string;
  descriptionKey: string;
  url: string;
  category: 'page' | 'feature' | 'setting' | 'admin';
  keywords: string[];
  icon?: string;
}

export interface SearchItem extends SearchItemSource {
  title: string;
  description: string;
  matchRange?: [number, number];
}

/**
 * 全局搜索数据源
 * 包含所有可搜索的页面和功能
 */
export const searchData: SearchItemSource[] = [
  // ==================== 总览 ====================
  {
    id: 'home',
    titleKey: 'nav.dashboard',
    descriptionKey: 'search.items.homeDesc',
    url: '/',
    category: 'page',
    keywords: ['home', '主页', '首页', 'dashboard', '总览'],
  },

  // ==================== 文档库 ====================
  {
    id: 'docs-how-to-use',
    titleKey: 'nav.usageDocs',
    descriptionKey: 'search.items.usageDocsDesc',
    url: 'https://openflare.fyrn.link/',
    category: 'page',
    keywords: ['docs', '文档', '使用', 'how to', 'tutorial', '教程', 'help'],
  },

  // ==================== 业务控制台 ====================
  {
    id: 'console-nodes',
    titleKey: 'nav.nodes',
    descriptionKey: 'search.items.nodesDesc',
    url: '/nodes',
    category: 'page',
    keywords: [
      'node',
      '节点',
      '边缘节点',
      '中继',
      '内网穿透',
      'tunnel',
      '服务器',
    ],
  },
  {
    id: 'console-proxy-routes',
    titleKey: 'nav.proxyRoutes',
    descriptionKey: 'search.items.proxyRoutesDesc',
    url: '/proxy-routes',
    category: 'page',
    keywords: ['route', '规则', '路由', '代理', '反向代理', 'proxy'],
  },
  {
    id: 'console-websites',
    titleKey: 'nav.websites',
    descriptionKey: 'search.items.websitesDesc',
    url: '/websites',
    category: 'page',
    keywords: ['website', 'domain', '网站', '域名', '站点'],
  },
  {
    id: 'console-certificates',
    titleKey: 'nav.certificates',
    descriptionKey: 'search.items.certificatesDesc',
    url: '/certificates',
    category: 'page',
    keywords: ['certificate', 'ssl', 'tls', '证书', 'https', '加密'],
  },
  {
    id: 'console-dns-accounts',
    titleKey: 'nav.dnsAccounts',
    descriptionKey: 'search.items.dnsAccountsDesc',
    url: '/dns-accounts',
    category: 'page',
    keywords: [
      'dns',
      'dns account',
      '账号',
      '域名解析',
      'cloudflare',
      'aliyun',
      'tencent',
    ],
  },
  {
    id: 'console-origins',
    titleKey: 'nav.origins',
    descriptionKey: 'search.items.originsDesc',
    url: '/origins',
    category: 'page',
    keywords: ['origin', '源站', '后端', 'backend', '服务器', '负载均衡'],
  },
  {
    id: 'console-responses',
    titleKey: 'nav.responses',
    descriptionKey: 'search.items.responsesDesc',
    url: '/responses',
    category: 'page',
    keywords: [
      '响应页面',
      '错误页',
      '联系页',
      '源站',
      '502',
      '503',
      '5xx',
      'Service Worker',
      '离线兜底',
      'error page',
    ],
  },
  {
    id: 'console-waf',
    titleKey: 'nav.waf',
    descriptionKey: 'search.items.wafDesc',
    url: '/waf',
    category: 'page',
    keywords: ['waf', '防火墙', '安全', 'security', '拦截', '规则'],
  },
  {
    id: 'console-ip-groups',
    titleKey: 'nav.ipGroups',
    descriptionKey: 'search.items.ipGroupsDesc',
    url: '/ip-groups',
    category: 'page',
    keywords: ['ip', 'ip group', 'ip组', '黑名单', '白名单', '访问控制'],
  },
  {
    id: 'console-rate-limits',
    titleKey: 'nav.rateLimits',
    descriptionKey: 'search.items.rateLimitsDesc',
    url: '/rate-limits',
    category: 'page',
    keywords: [
      '限流',
      'rate limit',
      'limit_conn',
      'limit_rate',
      '并发',
      '带宽',
    ],
  },
  {
    id: 'console-pages',
    titleKey: 'nav.pages',
    descriptionKey: 'search.items.pagesDesc',
    url: '/pages',
    category: 'page',
    keywords: ['pages', '静态托管', 'cdn', '网站', '部署', 'static'],
  },
  {
    id: 'console-config-versions',
    titleKey: 'nav.configVersions',
    descriptionKey: 'search.items.configVersionsDesc',
    url: '/config-versions',
    category: 'page',
    keywords: ['version', 'config', '版本', '发布', '回滚', '对比', '部署'],
  },
  {
    id: 'console-access-logs',
    titleKey: 'nav.accessLogs',
    descriptionKey: 'search.items.accessLogsDesc',
    url: '/access-logs',
    category: 'page',
    keywords: ['log', 'logs', '访问日志', '分析', '流量', '请求'],
  },
  {
    id: 'console-apply-logs',
    titleKey: 'nav.applyLogs',
    descriptionKey: 'search.items.applyLogsDesc',
    url: '/apply-logs',
    category: 'page',
    keywords: [
      'apply',
      'log',
      'logs',
      '应用记录',
      '配置下发',
      '同步',
      '部署历史',
    ],
  },
  {
    id: 'console-performance',
    titleKey: 'nav.performance',
    descriptionKey: 'search.items.performanceDesc',
    url: '/performance',
    category: 'page',
    keywords: ['performance', '性能', '调优', '优化', '参数', '连接', '超时'],
  },

  // ==================== 个人设置 ====================
  {
    id: 'settings',
    titleKey: 'search.items.globalSettings',
    descriptionKey: 'search.items.settingsDesc',
    url: '/settings',
    category: 'setting',
    keywords: ['settings', '设置', '偏好', 'preferences'],
  },
  {
    id: 'settings-profile',
    titleKey: 'user.profile',
    descriptionKey: 'search.items.profileDesc',
    url: '/settings/profile',
    category: 'setting',
    keywords: ['profile', '资料', '个人', '我的', '信息', 'avatar'],
  },
  {
    id: 'settings-appearance',
    titleKey: 'search.items.appearance',
    descriptionKey: 'search.items.appearanceDesc',
    url: '/settings/appearance',
    category: 'setting',
    keywords: ['appearance', '外观', '主题', 'theme', 'dark', 'light'],
  },
  {
    id: 'admin-settings',
    titleKey: 'nav.adminSettings',
    descriptionKey: 'search.items.adminSettingsDesc',
    url: '/admin/settings',
    category: 'admin',
    keywords: [
      'admin',
      '管理员',
      '系统设置',
      '安全',
      'security',
      'oidc',
      'login',
    ],
  },
  {
    id: 'admin-system',
    titleKey: 'nav.system',
    descriptionKey: 'search.items.systemDesc',
    url: '/admin/system',
    category: 'admin',
    keywords: ['admin', '管理员', '系统', '配置', 'system', 'configurations'],
  },
  {
    id: 'admin-users',
    titleKey: 'nav.users',
    descriptionKey: 'search.items.usersDesc',
    url: '/admin/users',
    category: 'admin',
    keywords: ['admin', '管理员', '用户', '管理', 'users', 'status'],
  },
  {
    id: 'admin-tasks',
    titleKey: 'nav.tasks',
    descriptionKey: 'search.items.tasksDesc',
    url: '/admin/tasks',
    category: 'admin',
    keywords: [
      'admin',
      '管理员',
      '任务',
      '异步',
      'tasks',
      'scheduler',
      'worker',
    ],
  },
  {
    id: 'admin-files',
    titleKey: 'nav.storage',
    descriptionKey: 'search.items.storageDesc',
    url: '/admin/files',
    category: 'admin',
    keywords: ['admin', '管理员', '存储', '文件', 'files', 'upload', 's3'],
  },
  {
    id: 'admin-database',
    titleKey: 'nav.database',
    descriptionKey: 'search.items.databaseDesc',
    url: '/admin/database',
    category: 'admin',
    keywords: ['admin', '管理员', '数据库', 'database', 'sql', 'query', 'gorm'],
  },
  {
    id: 'admin-push',
    titleKey: 'nav.push',
    descriptionKey: 'search.items.pushDesc',
    url: '/admin/push',
    category: 'admin',
    keywords: [
      'admin',
      '管理员',
      '推送',
      '通知',
      'push',
      'mail',
      'telegram',
      'lark',
    ],
  },
  {
    id: 'admin-logs',
    titleKey: 'nav.logs',
    descriptionKey: 'search.items.logsDesc',
    url: '/admin/logs',
    category: 'admin',
    keywords: ['admin', '管理员', '日志', 'logs', 'system log', 'terminal'],
  },
];

/**
 * 搜索功能
 * @param query 搜索关键词
 * @param isAdmin 是否为管理员
 * @param t 文案解析函数（layout 命名空间）
 * @returns 匹配的搜索结果
 */
export function searchItems(
  query: string,
  isAdmin: boolean = false,
  t: (key: string) => string,
): SearchItem[] {
  const trimmedQuery = query.trim();

  // 非管理员不能搜索 admin 类别项
  const filteredData = isAdmin
    ? searchData
    : searchData.filter((item) => item.category !== 'admin');

  const resolved = filteredData.map((item) => ({
    ...item,
    title: t(item.titleKey),
    description: t(item.descriptionKey),
  }));

  if (!trimmedQuery) {
    return resolved;
  }

  return resolved
    .map((item) => {
      // 优先匹配标题
      const titleMatch =
        typeof match === 'function' ? match(item.title, trimmedQuery) : null;
      if (titleMatch) {
        return { ...item, matchRange: titleMatch as [number, number] };
      }

      // 匹配描述
      if (
        typeof match === 'function' &&
        match(item.description, trimmedQuery)
      ) {
        return item;
      }

      // 匹配关键词
      if (
        item.keywords.some(
          (keyword) =>
            typeof match === 'function' && match(keyword, trimmedQuery),
        )
      ) {
        return item;
      }

      return null;
    })
    .filter((item): item is SearchItem => item !== null)
    .sort((a, b) => {
      // 标题匹配优先
      if (a.matchRange && !b.matchRange) return -1;
      if (!a.matchRange && b.matchRange) return 1;

      // 如果都是标题匹配，按匹配位置排序
      if (a.matchRange && b.matchRange) {
        return a.matchRange[0] - b.matchRange[0];
      }

      return 0;
    });
}
