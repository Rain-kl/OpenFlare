export { AboutService } from './about.service';
export { LegacyOpenFlareBaseService } from './legacy-base.service';
export type { LegacyApiResponse } from './legacy-base.service';

export { NodeService } from './node.service';
export { ProxyRouteService } from './proxy-route.service';
export { ConfigVersionService } from './config-version.service';
export { ApplyLogService } from './apply-log.service';
export { DashboardService } from './dashboard.service';
export { WafService } from './waf.service';
export { WebsiteService } from './website.service';
export { TlsCertificateService } from './tls-certificate.service';
export { DnsAccountService } from './dns-account.service';
export { PagesService } from './pages.service';
export { OriginService } from './origin.service';
export { AccessLogService } from './access-log.service';
export { OptionService } from './option.service';
export { UptimeKumaService } from './uptimekuma.service';
export { UpdateService } from './update.service';
export { StatusService } from './status.service';

export type {
  ApplyLogCleanupPayload,
  ApplyLogCleanupResult,
  ApplyLogItem,
  ApplyLogList,
  ApplyLogListQuery,
  ApplyResult,
  ConfigDiffResult,
  ConfigOptionDiffItem,
  ConfigPreviewResult,
  ConfigVersionCleanupPayload,
  ConfigVersionCleanupResult,
  ConfigVersionDetail,
  ConfigVersionSummary,
  NodeAgentReleaseInfo,
  NodeAgentUpdatePayload,
  NodeBootstrapToken,
  NodeItem,
  NodeMutationPayload,
  NodeObservability,
  NodeStatus,
  NodeType,
  OpenrestyStatus,
  ProxyRouteConfigSection,
  ProxyRouteCustomHeader,
  ProxyRouteItem,
  ProxyRouteMutationPayload,
  ProxyRoutePoWConfig,
  ReleaseChannel,
  SupportFile,
  DashboardOverview,
  DashboardNodeHealth,
  DashboardSummary,
  DashboardTraffic,
  DashboardCapacity,
  DistributionItem,
  TrafficDistributions,
  TrafficTrendPoint,
  CapacityTrendPoint,
  NetworkTrendPoint,
  DiskIOTrendPoint,
  WAFIPGroup,
  WAFIPGroupAutoTestPayload,
  WAFIPGroupAutoTestResult,
  WAFIPGroupPayload,
  WAFIPGroupSyncResult,
  WAFIPGroupSubscriptionFormat,
  WAFIPGroupType,
  WAFRuleGroup,
  WAFRuleGroupPayload,
  WAFSiteRuleGroups,
  AccessLogCleanupPayload,
  AccessLogCleanupResult,
  AccessLogFilters,
  AccessLogIPSummaryFilters,
  AccessLogIPSummaryItem,
  AccessLogIPSummaryList,
  AccessLogIPTrend,
  AccessLogIPTrendFilters,
  AccessLogItem,
  AccessLogList,
  FoldedAccessLogFilters,
  FoldedAccessLogIPFilters,
  FoldedAccessLogIPList,
  FoldedAccessLogList,
  OptionItem,
  GeoIPLookupResult,
  DatabaseCleanupPayload,
  DatabaseCleanupResult,
  DatabaseCleanupTarget,
  LatestReleaseInfo,
  OpenFlarePublicStatus,
  UpgradeLogItem,
  UpgradeStreamSnapshot,
  UploadedServerBinaryInfo,
  OriginDetail,
  OriginItem,
  OriginMutationPayload,
  PagesDeployment,
  PagesDeploymentFile,
  PagesProject,
  PagesProjectPayload,
  AcmeAccountItem,
  DnsAccountItem,
  DnsAccountMutationPayload,
  ManagedDomainItem,
  ManagedDomainMatchResult,
  ManagedDomainMutationPayload,
  TlsCertificateApplyPayload,
  TlsCertificateContentItem,
  TlsCertificateDetailItem,
  TlsCertificateFileImportPayload,
  TlsCertificateItem,
  TlsCertificateMutationPayload,
} from './types';

import {AboutService} from './about.service';
import {AccessLogService} from './access-log.service';
import {ApplyLogService} from './apply-log.service';
import {ConfigVersionService} from './config-version.service';
import {DashboardService} from './dashboard.service';
import {DnsAccountService} from './dns-account.service';
import {NodeService} from './node.service';
import {OptionService} from './option.service';
import {StatusService} from './status.service';
import {UpdateService} from './update.service';
import {UptimeKumaService} from './uptimekuma.service';
import {OriginService} from './origin.service';
import {PagesService} from './pages.service';
import {ProxyRouteService} from './proxy-route.service';
import {TlsCertificateService} from './tls-certificate.service';
import {WafService} from './waf.service';
import {WebsiteService} from './website.service';

export const openflareServices = {
  about: AboutService,
  node: NodeService,
  proxyRoute: ProxyRouteService,
  configVersion: ConfigVersionService,
  applyLog: ApplyLogService,
  dashboard: DashboardService,
  waf: WafService,
  website: WebsiteService,
  tlsCertificate: TlsCertificateService,
  dnsAccount: DnsAccountService,
  pages: PagesService,
  origin: OriginService,
  accessLog: AccessLogService,
  option: OptionService,
  uptimeKuma: UptimeKumaService,
  update: UpdateService,
  status: StatusService,
} as const;
