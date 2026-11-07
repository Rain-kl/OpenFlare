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
  keywordKey: string;
  icon?: string;
}

export interface SearchItem extends Omit<SearchItemSource, 'keywordKey'> {
  keywords: string[];
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
    keywordKey: 'search.keywords.home',
  },

  // ==================== 文档库 ====================
  {
    id: 'docs-how-to-use',
    titleKey: 'nav.usageDocs',
    descriptionKey: 'search.items.usageDocsDesc',
    url: 'https://openflare.fyrn.link/',
    category: 'page',
    keywordKey: 'search.keywords.docs-how-to-use',
  },

  // ==================== 业务控制台 ====================
  {
    id: 'console-nodes',
    titleKey: 'nav.nodes',
    descriptionKey: 'search.items.nodesDesc',
    url: '/nodes',
    category: 'page',
    keywordKey: 'search.keywords.console-nodes',
  },
  {
    id: 'console-proxy-routes',
    titleKey: 'nav.proxyRoutes',
    descriptionKey: 'search.items.proxyRoutesDesc',
    url: '/proxy-routes',
    category: 'page',
    keywordKey: 'search.keywords.console-proxy-routes',
  },
  {
    id: 'console-websites',
    titleKey: 'nav.websites',
    descriptionKey: 'search.items.websitesDesc',
    url: '/websites',
    category: 'page',
    keywordKey: 'search.keywords.console-websites',
  },
  {
    id: 'console-certificates',
    titleKey: 'nav.certificates',
    descriptionKey: 'search.items.certificatesDesc',
    url: '/certificates',
    category: 'page',
    keywordKey: 'search.keywords.console-certificates',
  },
  {
    id: 'console-dns-accounts',
    titleKey: 'nav.dnsAccounts',
    descriptionKey: 'search.items.dnsAccountsDesc',
    url: '/dns-accounts',
    category: 'page',
    keywordKey: 'search.keywords.console-dns-accounts',
  },
  {
    id: 'console-origins',
    titleKey: 'nav.origins',
    descriptionKey: 'search.items.originsDesc',
    url: '/origins',
    category: 'page',
    keywordKey: 'search.keywords.console-origins',
  },
  {
    id: 'console-responses',
    titleKey: 'nav.responses',
    descriptionKey: 'search.items.responsesDesc',
    url: '/responses',
    category: 'page',
    keywordKey: 'search.keywords.console-responses',
  },
  {
    id: 'console-waf',
    titleKey: 'nav.waf',
    descriptionKey: 'search.items.wafDesc',
    url: '/waf',
    category: 'page',
    keywordKey: 'search.keywords.console-waf',
  },
  {
    id: 'console-ip-groups',
    titleKey: 'nav.ipGroups',
    descriptionKey: 'search.items.ipGroupsDesc',
    url: '/ip-groups',
    category: 'page',
    keywordKey: 'search.keywords.console-ip-groups',
  },
  {
    id: 'console-rate-limits',
    titleKey: 'nav.rateLimits',
    descriptionKey: 'search.items.rateLimitsDesc',
    url: '/rate-limits',
    category: 'page',
    keywordKey: 'search.keywords.console-rate-limits',
  },
  {
    id: 'console-pages',
    titleKey: 'nav.pages',
    descriptionKey: 'search.items.pagesDesc',
    url: '/pages',
    category: 'page',
    keywordKey: 'search.keywords.console-pages',
  },
  {
    id: 'console-config-versions',
    titleKey: 'nav.configVersions',
    descriptionKey: 'search.items.configVersionsDesc',
    url: '/config-versions',
    category: 'page',
    keywordKey: 'search.keywords.console-config-versions',
  },
  {
    id: 'console-access-logs',
    titleKey: 'nav.accessLogs',
    descriptionKey: 'search.items.accessLogsDesc',
    url: '/access-logs',
    category: 'page',
    keywordKey: 'search.keywords.console-access-logs',
  },
  {
    id: 'console-apply-logs',
    titleKey: 'nav.applyLogs',
    descriptionKey: 'search.items.applyLogsDesc',
    url: '/apply-logs',
    category: 'page',
    keywordKey: 'search.keywords.console-apply-logs',
  },
  {
    id: 'console-performance',
    titleKey: 'nav.performance',
    descriptionKey: 'search.items.performanceDesc',
    url: '/performance',
    category: 'page',
    keywordKey: 'search.keywords.console-performance',
  },

  // ==================== 个人设置 ====================
  {
    id: 'settings',
    titleKey: 'search.items.globalSettings',
    descriptionKey: 'search.items.settingsDesc',
    url: '/settings',
    category: 'setting',
    keywordKey: 'search.keywords.settings',
  },
  {
    id: 'settings-profile',
    titleKey: 'user.profile',
    descriptionKey: 'search.items.profileDesc',
    url: '/settings/profile',
    category: 'setting',
    keywordKey: 'search.keywords.settings-profile',
  },
  {
    id: 'settings-appearance',
    titleKey: 'search.items.appearance',
    descriptionKey: 'search.items.appearanceDesc',
    url: '/settings/appearance',
    category: 'setting',
    keywordKey: 'search.keywords.settings-appearance',
  },
  {
    id: 'admin-settings',
    titleKey: 'nav.adminSettings',
    descriptionKey: 'search.items.adminSettingsDesc',
    url: '/admin/settings',
    category: 'admin',
    keywordKey: 'search.keywords.admin-settings',
  },
  {
    id: 'admin-system',
    titleKey: 'nav.system',
    descriptionKey: 'search.items.systemDesc',
    url: '/admin/system',
    category: 'admin',
    keywordKey: 'search.keywords.admin-system',
  },
  {
    id: 'admin-users',
    titleKey: 'nav.users',
    descriptionKey: 'search.items.usersDesc',
    url: '/admin/users',
    category: 'admin',
    keywordKey: 'search.keywords.admin-users',
  },
  {
    id: 'admin-tasks',
    titleKey: 'nav.tasks',
    descriptionKey: 'search.items.tasksDesc',
    url: '/admin/tasks',
    category: 'admin',
    keywordKey: 'search.keywords.admin-tasks',
  },
  {
    id: 'admin-files',
    titleKey: 'nav.storage',
    descriptionKey: 'search.items.storageDesc',
    url: '/admin/files',
    category: 'admin',
    keywordKey: 'search.keywords.admin-files',
  },
  {
    id: 'admin-database',
    titleKey: 'nav.database',
    descriptionKey: 'search.items.databaseDesc',
    url: '/admin/database',
    category: 'admin',
    keywordKey: 'search.keywords.admin-database',
  },
  {
    id: 'admin-push',
    titleKey: 'nav.push',
    descriptionKey: 'search.items.pushDesc',
    url: '/admin/push',
    category: 'admin',
    keywordKey: 'search.keywords.admin-push',
  },
  {
    id: 'admin-logs',
    titleKey: 'nav.logs',
    descriptionKey: 'search.items.logsDesc',
    url: '/admin/logs',
    category: 'admin',
    keywordKey: 'search.keywords.admin-logs',
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

  const resolved: SearchItem[] = filteredData.map((item) => ({
    ...item,
    title: t(item.titleKey),
    description: t(item.descriptionKey),
    keywords: t(item.keywordKey).split('|'),
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
