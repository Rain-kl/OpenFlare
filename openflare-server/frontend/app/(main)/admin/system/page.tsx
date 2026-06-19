"use client"

import * as React from "react"
import {Input} from "@/components/ui/input"
import {Switch} from "@/components/ui/switch"
import {ManageDetailPanel, ManagePage} from "@/components/common/general/manage-pannel"
import {Tabs, TabsList, TabsTrigger} from "@/components/ui/tabs"
import {ShieldCheck} from "lucide-react"

import {formatDateTime} from "@/lib/utils"
import type {SystemConfig} from "@/lib/services/admin"
import {AdminProvider, useAdmin} from "@/contexts/admin-context"


/**
 * 系统配置
 * 显示系统配置的详细信息和编辑面板
 *
 * @example
 * ```tsx
 * <SystemConfigDetailPanel
 *   config={config}
 *   editData={editData}
 *   onEditDataChange={onEditDataChange}
 *   onSave={onSave}
 *   saving={saving}
 * />
 * ```
 * @param {SystemConfig} config - 系统配置
 * @param {Partial<SystemConfig>} editData - 编辑数据
 * @param {function} onEditDataChange - 编辑数据改变回调
 * @param {function} onSave - 保存回调
 * @param {boolean} saving - 是否正在保存
 * @returns {React.ReactNode} 系统配置详情面板组件
 */
function SystemConfigDetailPanel({
  config,
  editData,
  onEditDataChange,
  onSave,
  saving
}: {
  config: SystemConfig | null
  editData: Partial<SystemConfig>
  onEditDataChange: (field: keyof SystemConfig, value: SystemConfig[keyof SystemConfig]) => void
  onSave: () => void
  saving: boolean
}) {

  return (
    <ManageDetailPanel
      isEmpty={!config}
      onSave={onSave}
      saving={saving}
    >
      <div className="grid grid-cols-1 gap-0">
        <div className="border border-dashed rounded-lg">
          <div className="px-3 py-2 flex items-center justify-between border-b border-dashed last:border-b-0">
            <label className="text-xs font-medium text-muted-foreground">配置键</label>
            <p className="text-xs text-muted-foreground font-mono">{config?.key}</p>
          </div>

          <div className="pl-3 py-2 flex items-center justify-between border-b border-dashed last:border-b-0">
            <label className="text-xs font-medium text-muted-foreground">配置值</label>
            <div className="flex gap-1 w-[90%] justify-end items-center pr-3">
              {config?.key.endsWith('_enabled') ? (
                <Switch
                  checked={editData.value !== undefined ? editData.value === 'true' : (config?.value === 'true')}
                  onCheckedChange={(checked) => {
                    onEditDataChange('value', checked ? 'true' : 'false')
                  }}
                />
              ) : (
                <Input
                  type={
                    config?.key && (
                      config.key.endsWith('_limit') ||
                      config.key.endsWith('_minutes') ||
                      config.key.endsWith('_days') ||
                      config.key.endsWith('_seconds') ||
                      config.key.endsWith('_hours') ||
                      config.key.endsWith('_ttl') ||
                      config.key.endsWith('_max') ||
                      config.key.endsWith('_min') ||
                      config.key.includes('max_') ||
                      config.key.includes('min_') ||
                      config.key.includes('limit') ||
                      config.key.includes('count')
                    ) ? "number" : "text"
                  }
                  step="1"
                  min="0"
                  value={editData.value !== undefined ? editData.value : (config?.value || '')}
                  placeholder={editData.value === undefined && !config?.value ? '必需' : ''}
                  onChange={(e) => {
                    const value = e.target.value
                    if (value === '') {
                      onEditDataChange('value', '')
                      return
                    }
                    onEditDataChange('value', value)
                  }}
                  className="!text-[12px] text-right h-4 rounded-none border-none shadow-none focus-visible:ring-0 focus-visible:ring-offset-0 placeholder:!text-[12px] [&::-webkit-outer-spin-button]:appearance-none [&::-webkit-inner-spin-button]:appearance-none [&::-webkit-inner-spin-button]"
                  style={{
                    MozAppearance: 'textfield'
                  }}
                />
              )}
            </div>
          </div>

          <div className="pl-3 py-2 flex items-center justify-between border-b border-dashed last:border-b-0">
            <label className="text-xs font-medium text-muted-foreground">配置描述</label>
            <div className="flex gap-1 w-[90%]">
              <Input
                type="text"
                value={editData.description !== undefined ? editData.description : (config?.description || '')}
                placeholder="可选描述"
                onChange={(e) => {
                  const value = e.target.value
                  onEditDataChange('description', value)
                }}
                className="!text-[12px] text-right h-4 rounded-none border-none shadow-none focus-visible:ring-0 focus-visible:ring-offset-0 placeholder:!text-[12px]"
              />
            </div>
          </div>

          <div className="px-3 py-2 flex items-center justify-between border-b border-dashed last:border-b-0">
            <label className="text-xs font-medium text-muted-foreground">公共可见</label>
            <Switch
              checked={(editData.visibility ?? config?.visibility ?? 0) === 1}
              onCheckedChange={(checked) => {
                onEditDataChange('visibility', checked ? 1 : 0)
              }}
            />
          </div>

          <div className="px-3 py-2 flex items-center justify-between border-b border-dashed last:border-b-0">
            <label className="text-xs font-medium text-muted-foreground">配置类型</label>
            <span className={`text-[11px] px-1.5 py-0.5 rounded font-medium ${
              config?.type === 'system'
                ? 'bg-blue-500/10 text-blue-600 dark:bg-blue-500/20 dark:text-blue-400'
                : 'bg-indigo-500/10 text-indigo-600 dark:bg-indigo-500/20 dark:text-indigo-400'
            }`}>
              {config?.type === 'system' ? '系统配置' : '业务配置'}
            </span>
          </div>

          <div className="px-3 py-2 flex items-center justify-between">
            <label className="text-xs font-medium text-muted-foreground">创建时间</label>
            <p className="text-xs text-muted-foreground">{config ? formatDateTime(config.created_at) : ''}</p>
          </div>
        </div>
      </div>
    </ManageDetailPanel>
  )
}



/**
 * 系统配置管理组件
 *
 * @example
 * ```tsx
 * <SystemConfigs />
 * ```
 * @returns {React.ReactNode} 系统配置管理组件
 */
export function SystemConfigs() {
  const {
    systemConfigs: configs,
    systemConfigsLoading: loading,
    systemConfigsError: error,
    refetchSystemConfigs,
    updateSystemConfig
  } = useAdmin()

  const [activeTab, setActiveTab] = React.useState<'system' | 'business'>('business')

  React.useEffect(() => {
    refetchSystemConfigs(activeTab)
  }, [activeTab, refetchSystemConfigs])

  const getInitialEditData = (config: SystemConfig) => ({
    value: config.value,
    visibility: config.visibility,
    description: config.description
  })

  const handleSave = async (config: SystemConfig, editData: Partial<SystemConfig>) => {
    if (!config) return

    await updateSystemConfig(config.key, {
      value: editData.value ?? config.value,
      visibility: editData.visibility ?? config.visibility,
      description: editData.description ?? config.description
    })
  }

  return (
    <ManagePage<SystemConfig>
      title="系统配置"
      icon={ShieldCheck}
      data={configs}
      loading={loading}
      error={error}
      onReload={() => refetchSystemConfigs(activeTab)}
      getInitialEditData={getInitialEditData}
      onSave={handleSave}
      getId={(config) => config.key}
      emptyDescription="未发现系统配置"
      loadingDescription="配置加载中"
      headerExtra={
        <Tabs value={activeTab} onValueChange={(val) => setActiveTab(val as 'system' | 'business')} className="w-[180px]">
          <TabsList className="grid w-full grid-cols-2 h-8">
            <TabsTrigger value="business" className="text-[11px] h-7">业务配置</TabsTrigger>
            <TabsTrigger value="system" className="text-[11px] h-7">系统配置</TabsTrigger>
          </TabsList>
        </Tabs>
      }
      columns={[
        { header: "配置键", cell: (item) => <span className="font-mono font-medium">{item.key}</span>, width: "200px" },
        { header: "配置值", cell: (item) => <span className="truncate max-w-[120px] inline-block" title={item.value}>{item.value}</span>, width: "120px" },
        { header: "公共可见", cell: (item) => <span>{item.visibility === 1 ? "可见" : "不可见"}</span>, width: "80px" },
        { header: "描述", cell: (item) => <span className="truncate max-w-[200px] inline-block text-muted-foreground" title={item.description}>{item.description}</span>, width: "200px" },
      ]}
      renderDetail={({ selected, hovered, editData, onEditDataChange, onSave, saving }) => (
        <SystemConfigDetailPanel
          config={selected || hovered}
          editData={editData}
          onEditDataChange={onEditDataChange}
          onSave={onSave}
          saving={saving}
        />
      )}
    />
  )
}

export default function SystemConfigPage() {
  return (
    <AdminProvider>
      <SystemConfigs />
    </AdminProvider>
  )
}
