export interface DnsAccountItem {
  id: number;
  name: string;
  type: string;
  created_at: string;
  updated_at: string;
}

export interface DnsAccountMutationPayload {
  name: string;
  type: string;
  authorization: string;
}
