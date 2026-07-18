import { OpenFlareBaseService } from './base.service';
import type {
  AccessLogCleanupPayload,
  AccessLogCleanupResult,
  AccessLogFilters,
  AccessLogIPAnalysis,
  AccessLogIPAnalysisFilters,
  AccessLogIPSummaryFilters,
  AccessLogIPSummaryList,
  AccessLogIPTrend,
  AccessLogIPTrendFilters,
  AccessLogList,
  AccessLogOverview,
  AccessLogOverviewFilters,
  FoldedAccessLogFilters,
  FoldedAccessLogIPFilters,
  FoldedAccessLogIPList,
  FoldedAccessLogList,
} from './types';

function buildSearchParams(filters: object): Record<string, unknown> {
  const params: Record<string, unknown> = {};
  Object.entries(filters as Record<string, unknown>).forEach(([key, value]) => {
    if (value === undefined || value === null || value === '') return;
    if (Array.isArray(value)) {
      const items = value
        .map((item) => String(item).trim())
        .filter((item) => item !== '');
      if (items.length === 0) return;
      params[key] = items;
      return;
    }
    params[key] = value;
  });
  return params;
}

export class AccessLogService extends OpenFlareBaseService {
  protected static override readonly basePath: string = '/api/v1/d/access-logs';

  static list(filters: AccessLogFilters = {}): Promise<AccessLogList> {
    return this.get<AccessLogList>('/', buildSearchParams(filters));
  }

  static getOverview(
    filters: AccessLogOverviewFilters = {},
  ): Promise<AccessLogOverview> {
    return this.get<AccessLogOverview>('/overview', buildSearchParams(filters));
  }

  static listFolds(
    filters: FoldedAccessLogFilters,
  ): Promise<FoldedAccessLogList> {
    return this.get<FoldedAccessLogList>('/folds', buildSearchParams(filters));
  }

  static listFoldIPs(
    filters: FoldedAccessLogIPFilters,
  ): Promise<FoldedAccessLogIPList> {
    return this.get<FoldedAccessLogIPList>(
      '/folds/ip-summary',
      buildSearchParams(filters),
    );
  }

  static listIPSummaries(
    filters: AccessLogIPSummaryFilters = {},
  ): Promise<AccessLogIPSummaryList> {
    return this.get<AccessLogIPSummaryList>(
      '/ip-summary',
      buildSearchParams(filters),
    );
  }

  static getIPTrend(
    filters: AccessLogIPTrendFilters,
  ): Promise<AccessLogIPTrend> {
    return this.get<AccessLogIPTrend>(
      '/ip-summary/trend',
      buildSearchParams(filters),
    );
  }

  static getIPAnalysis(
    filters: AccessLogIPAnalysisFilters,
  ): Promise<AccessLogIPAnalysis> {
    return this.get<AccessLogIPAnalysis>(
      '/ip-summary/analysis',
      buildSearchParams(filters),
    );
  }

  static cleanup(
    payload: AccessLogCleanupPayload,
  ): Promise<AccessLogCleanupResult> {
    return this.post<AccessLogCleanupResult>('/cleanup', payload);
  }
}
