export interface OriginItem {
  id: number;
  name: string;
  address: string;
  remark: string;
  route_count: number;
  created_at: string;
  updated_at: string;
}

export interface OriginRouteSummary {
  id: number;
  domain: string;
  origin_url: string;
  enabled: boolean;
  updated_at: string;
}

export interface OriginDetail extends OriginItem {
  routes: OriginRouteSummary[];
}

export interface OriginMutationPayload {
  name: string;
  address: string;
  remark: string;
}
