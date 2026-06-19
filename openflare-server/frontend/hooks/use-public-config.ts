import {useQuery} from '@tanstack/react-query'
import services from '@/lib/services'

/**
 * 获取公共配置（使用 React Query 进行统一缓存与自动同步）
 *
 * @example
 * ```tsx
 * const { config, loading, error } = usePublicConfig()
 * ```
 */
export function usePublicConfig() {
  const { data: config, isLoading: loading, error } = useQuery({
    queryKey: ['public-config'],
    queryFn: () => services.config.getPublicConfig(),
    staleTime: 5 * 60 * 1000, // 5 分钟缓存
  })

  return { config: config || null, loading, error }
}
