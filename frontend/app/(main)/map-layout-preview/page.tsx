import { WorldStage } from '../components/dashboard/world-stage';

export default function MapPreview() {
  return <div className='py-6 px-1'><WorldStage
    summary={{ total_nodes: 1, online_nodes: 1, offline_nodes: 0, pending_nodes: 0, unhealthy_nodes: 0 }}
    traffic={{ request_count: 47124, unique_visitors: 120, error_count: 0, estimated_qps: 0.5, reported_nodes: 1 }}
    sourceCountries={[
      {key: 'China', value: 45400}, {key: 'United States of America', value: 686},
      {key: 'Germany', value: 292}, {key: 'Belgium', value: 265},
      {key: 'United Kingdom', value: 98}, {key: 'South Africa', value: 96},
      {key: 'Netherlands', value: 77},
    ]}
    nodes={[{id: 1, node_id: 'preview', name: 'Shanghai', geo_name: 'China', geo_latitude: 31.2, geo_longitude: 121.5, status: 'online', openresty_status: 'healthy', current_version: '', last_seen_at: '', active_event_count: 0, cpu_usage_percent: 0, memory_usage_percent: 0, storage_usage_percent: 0, request_count: 45400, error_count: 0, unique_visitor_count: 100}]}
  /></div>;
}
