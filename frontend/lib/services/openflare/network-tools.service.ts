import { OpenFlareBaseService } from './base.service';
import type {
  NetToolsASNDetailResponse,
  NetToolsASNPrefixesResponse,
  NetToolsASNUpstreamGraphResponse,
  NetToolsASPathInput,
  NetToolsASPathResponse,
  NetToolsDNSQueryPayload,
  NetToolsDNSQueryResponse,
  NetToolsIPLookupResponse,
  NetToolsNetIntelStatusResponse,
  NetToolsPingPayload,
  NetToolsProbeTaskState,
  NetToolsQualityMatrixPage,
  NetToolsQualityMatrixQuery,
  NetToolsTracerouteHistoryPage,
  NetToolsTracerouteHistoryQuery,
  NetToolsTraceroutePayload,
  NetToolsTriggerSyncResponse,
} from './types';

export class NetworkToolsService extends OpenFlareBaseService {
  protected static override readonly basePath: string =
    '/api/v1/d/network-tools';

  static async dispatchPing(
    payload: NetToolsPingPayload,
  ): Promise<NetToolsProbeTaskState> {
    return this.post<NetToolsProbeTaskState>('/ping', payload);
  }

  static async dispatchTraceroute(
    payload: NetToolsTraceroutePayload,
  ): Promise<NetToolsProbeTaskState> {
    return this.post<NetToolsProbeTaskState>('/traceroute', payload);
  }

  static async getTask(taskId: string): Promise<NetToolsProbeTaskState> {
    return this.get<NetToolsProbeTaskState>(
      `/tasks/${encodeURIComponent(taskId)}`,
    );
  }

  static async listTracerouteHistory(
    query: NetToolsTracerouteHistoryQuery = {},
  ): Promise<NetToolsTracerouteHistoryPage> {
    return this.get<NetToolsTracerouteHistoryPage>(
      '/traceroute/history',
      query as Record<string, unknown>,
    );
  }

  static async queryDNS(
    payload: NetToolsDNSQueryPayload,
  ): Promise<NetToolsDNSQueryResponse> {
    return this.post<NetToolsDNSQueryResponse>('/dns', payload);
  }

  static async lookupIP(ipOrHost: string): Promise<NetToolsIPLookupResponse> {
    return this.get<NetToolsIPLookupResponse>('/lookup/ip', {
      ip: ipOrHost,
    });
  }

  static async lookupASN(asn: number): Promise<NetToolsASNDetailResponse> {
    return this.get<NetToolsASNDetailResponse>(`/lookup/asn/${asn}`);
  }

  static async listASNPrefixes(
    asn: number,
    query: { family?: string; p?: number; page_size?: number } = {},
  ): Promise<NetToolsASNPrefixesResponse> {
    return this.get<NetToolsASNPrefixesResponse>(
      `/lookup/asn/${asn}/prefixes`,
      query as Record<string, unknown>,
    );
  }

  static async getASNUpstreamGraph(
    asn: number,
    query: { prefix?: string; refresh?: boolean } = {},
  ): Promise<NetToolsASNUpstreamGraphResponse> {
    return this.get<NetToolsASNUpstreamGraphResponse>(
      `/lookup/asn/${asn}/upstream-graph`,
      query as Record<string, unknown>,
    );
  }

  static async traceASPath(
    payload: NetToolsASPathInput,
  ): Promise<NetToolsASPathResponse> {
    return this.post<NetToolsASPathResponse>('/as-path', payload);
  }

  static async listQualityMatrix(
    query: NetToolsQualityMatrixQuery = {},
  ): Promise<NetToolsQualityMatrixPage> {
    return this.get<NetToolsQualityMatrixPage>(
      '/quality-matrix',
      query as Record<string, unknown>,
    );
  }

  static async getNetIntelStatus(): Promise<NetToolsNetIntelStatusResponse> {
    return this.get<NetToolsNetIntelStatusResponse>('/netintel/status');
  }

  static async triggerNetIntelSync(
    force = false,
  ): Promise<NetToolsTriggerSyncResponse> {
    return this.post<NetToolsTriggerSyncResponse>('/netintel/sync', { force });
  }
}
