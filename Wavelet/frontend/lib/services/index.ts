/**
 * 服务层统一入口
 *
 * @example
 * ```typescript
 * import services from '@/lib/services';
 *
 * const user = await services.auth.getUserInfo();
 * const configs = await services.adminSystemConfig.listSystemConfigs();
 * ```
 */

import {
  AdminAuthSourceService,
  AdminCacheService,
  AdminLogService,
  AdminStatusService,
  AdminSystemConfigService,
  AdminTaskService,
  AdminTemplateService,
  AdminUserService,
} from './admin';
import {AuthService} from './auth';
import {ConfigService} from './config';
import {DbManageService} from './db-manage';
import {PushService} from './push';
import {AdminUploadService, UploadService} from './upload';
import {UserService} from './user';
import {
  AccessLogService,
  ApplyLogService,
  ConfigVersionService,
  DashboardService,
  DnsAccountService,
  NodeService,
  OptionService,
  OriginService,
  PagesService,
  ProxyRouteService,
  StatusService,
  TlsCertificateService,
  UptimeKumaService,
  WafService,
  WebsiteService,
} from './openflare';

const services = {
  auth: AuthService,
  adminSystemConfig: AdminSystemConfigService,
  adminAuthSource: AdminAuthSourceService,
  adminTask: AdminTaskService,
  adminUser: AdminUserService,
  adminStatus: AdminStatusService,
  adminLog: AdminLogService,
  adminTemplate: AdminTemplateService,
  adminCache: AdminCacheService,
  user: UserService,
  config: ConfigService,
  upload: UploadService,
  adminUpload: AdminUploadService,
  dbManage: DbManageService,
  push: PushService,
  openflareNode: NodeService,
  openflareProxyRoute: ProxyRouteService,
  openflareConfigVersion: ConfigVersionService,
  openflareApplyLog: ApplyLogService,
  openflareDashboard: DashboardService,
  openflareWaf: WafService,
  openflareWebsite: WebsiteService,
  openflareTls: TlsCertificateService,
  openflareDns: DnsAccountService,
  openflarePages: PagesService,
  openflareOrigin: OriginService,
  openflareAccessLog: AccessLogService,
  openflareOption: OptionService,
  openflareUptimeKuma: UptimeKumaService,
  openflareStatus: StatusService,
} as const;

export default services;
export { services };

// ==================== 核心模块导出 ====================

export {
  apiClient,
  BaseService,
  apiConfig,
  cancelRequest,
  cancelAllRequests,
} from './core';

export {
  ApiErrorBase,
  NetworkError,
  TimeoutError,
  UnauthorizedError,
  ForbiddenError,
  NotFoundError,
  ServerError,
  ValidationError,
  isCancelError,
} from './core';

export type {
  ApiResponse,
  ApiError,
  PaginationParams,
  PaginationResponse,
  RequestConfig,
} from './core';

// ==================== 业务服务导出 ====================

export { AuthService } from './auth';
export type { User, OAuthLoginUrlResponse, OAuthCallbackRequest, AuthSource, ExternalAccountBinding, ChangePasswordRequest, UpdateProfileRequest } from './auth';

export { ConfigService } from './config';
export type { PublicConfigResponse } from './config';

export {
  AdminSystemConfigService,
  AdminAuthSourceService,
  AdminTaskService,
  AdminUserService,
  AdminStatusService,
  AdminLogService,
  AdminTemplateService,
  AdminCacheService,
} from './admin';

export type {
  SystemConfig,
  CreateSystemConfigRequest,
  UpdateSystemConfigRequest,
  TaskMeta,
  TaskParam,
  TaskParamType,
  TaskExecution,
  TaskExecutionStatus,
  ListTaskExecutionsRequest,
  ListTaskExecutionsResponse,
  DispatchTaskRequest,
  AdminUser,
  ListUsersRequest,
  ListUsersResponse,
  UpdateUserStatusRequest,
  CreateUserRequest,
  SystemStatus,
  AppUpdateStatus,
  Schedule,
  CreateScheduleRequest,
  UpdateScheduleRequest,
  CacheStatus,
  CacheConfig,
  Template,
  CreateTemplateRequest,
  UpdateTemplateRequest,
  AuthSource as AdminAuthSource,
  AuthSourceRequest,
  ToggleAuthSourceRequest,
  StorageDriver,
  StorageConfig,
  ObjectStorageConfig,
} from './admin';

export { UserService } from './user';
export type { AccessToken, CreateTokenResponse } from './user';

export { UploadService, AdminUploadService, formatFileSize, getFileUrl } from './upload';
export type { UploadImageResponse, Upload, ListUploadsResponse, FileStatsResponse, ImageQuality } from './upload';

export { DbManageService } from './db-manage';
export type { DBOverview, TableDataResponse, ExecuteSQLResponse } from './db-manage';

export { PushService } from './push';
export type { PushEvent, PushHistory, PushChannelConfig, ListPushHistoriesRequest, ListPushHistoriesResponse, UpdatePushEventRequest, TestPushRequest } from './push';

// ==================== OpenFlare 业务服务导出 ====================

export {
  OpenFlareBaseService,
  NodeService,
  ProxyRouteService,
  ConfigVersionService,
  ApplyLogService,
  DashboardService,
  WafService,
  WebsiteService,
  TlsCertificateService,
  DnsAccountService,
  PagesService,
  OriginService,
  AccessLogService,
  OptionService,
  StatusService,
  UptimeKumaService,
} from './openflare';

export type {
  NodeItem,
  ProxyRouteItem,
  ProxyRouteConfigSection,
  ConfigVersionSummary,
  ConfigVersionDetail,
  ConfigDiffResult,
  ConfigPreviewResult,
  ApplyLogList,
  DashboardOverview,
  WAFIPGroup,
  WAFRuleGroup,
  WAFSiteRuleGroups,
  ManagedDomainItem,
  TlsCertificateItem,
  DnsAccountItem,
  PagesProject,
  PagesDeployment,
  OriginItem,
  OriginDetail,
  AccessLogList,
  OptionItem,
  GeoIPLookupResult,
  DatabaseCleanupResult,
  OpenFlarePublicStatus,
} from './openflare';
