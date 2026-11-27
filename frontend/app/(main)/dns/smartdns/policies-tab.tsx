'use client';

import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { CheckCircle2, GitBranch, Plus } from 'lucide-react';
import { useTranslations } from 'next-intl';
import { useState } from 'react';
import { toast } from 'sonner';

import { ErrorInline } from '@/components/layout/error';
import { LoadingStateWithBorder } from '@/components/layout/loading';
import {
  AlertDialog,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog';
import { Button } from '@/components/ui/button';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import { Switch } from '@/components/ui/switch';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { Textarea } from '@/components/ui/textarea';
import {
  SmartDNSService,
  smartdnsQueryKey,
  type SmartDNSFlowGraph,
  type SmartDNSPolicy,
  type SmartDNSPolicyConfig,
} from '@/lib/services/openflare';
import { getErrorMessage } from '../../websites/components/website-utils';
import { SmartDNSPolicyList } from './policy-list';
import { PresetRuleFields, type PresetRules } from './preset-rule-fields';
import { createDefaultFlowGraph, PolicyFlowEditor } from './policy-flow-editor';

function splitList(raw: string): string[] {
  return raw
    .split(/[\n,]+/)
    .map((s) => s.trim())
    .filter(Boolean);
}

const smartWeights = { latency: '0.55', load: '0.30', geo: '0.15' };
const balanceWeights = { latency: '0', load: '1', geo: '0' };
const defaultRules: PresetRules = {
  ...smartWeights,
  useTagWeights: true,
  countries: '',
  tags: '',
  groups: '',
};

export function SmartDNSPoliciesTab({
  onSimulatePolicy,
}: {
  onSimulatePolicy: (policyId: number) => void;
}) {
  const t = useTranslations('smartdns.policies');
  const tc = useTranslations('common');
  const queryClient = useQueryClient();

  const [section, setSection] = useState('basics');
  const [editorOpen, setEditorOpen] = useState(false);
  const [editing, setEditing] = useState<SmartDNSPolicy | null>(null);
  const [editorModeTab, setEditorModeTab] = useState<'preset' | 'flow' | 'jdm'>(
    'preset',
  );

  const [name, setName] = useState('');
  const [description, setDescription] = useState('');
  const [scopeType, setScopeType] = useState<'all' | 'domains'>('all');
  const [scopeDomains, setScopeDomains] = useState('');
  const [strategyType, setStrategyType] = useState<
    'smart' | 'load_balance' | 'custom'
  >('smart');
  const [priority, setPriority] = useState('100');
  const [ttl, setTTL] = useState('60');
  const [maxAnswers, setMaxAnswers] = useState('1');
  const [enabled, setEnabled] = useState(true);
  const [isDefault, setIsDefault] = useState(false);
  const [fallbackStrategy, setFallbackStrategy] = useState('smart');
  const [fallbackNodeIDs, setFallbackNodeIDs] = useState('');

  const [preset, setPreset] = useState(defaultRules);
  const [flowGraph, setFlowGraph] = useState<SmartDNSFlowGraph>(() =>
    createDefaultFlowGraph('smart'),
  );
  const [jdmJSON, setJdmJSON] = useState('');

  const [defaultTarget, setDefaultTarget] = useState<SmartDNSPolicy | null>(
    null,
  );
  const [deleteTarget, setDeleteTarget] = useState<SmartDNSPolicy | null>(null);

  const policiesQuery = useQuery({
    queryKey: [...smartdnsQueryKey, 'policies'],
    queryFn: () => SmartDNSService.listPolicies(),
  });

  const refresh = async () => {
    await queryClient.invalidateQueries({ queryKey: smartdnsQueryKey });
  };

  const buildConfigPayload = (): SmartDNSPolicyConfig => ({
    mode: strategyType === 'load_balance' ? 'load_balance' : 'smart',
    weights: {
      latency: Number(preset.latency) || 0,
      load: Number(preset.load) || 0,
      geo: Number(preset.geo) || 0,
    },
    use_tag_weights: preset.useTagWeights,
    filters: {
      node_ids: editing?.config?.filters?.node_ids ?? [],
      countries: splitList(preset.countries),
      tags: splitList(preset.tags),
      network_groups: splitList(preset.groups),
    },
    max_answers: Math.max(1, Math.trunc(Number(maxAnswers) || 1)),
    ttl: Math.max(10, Math.trunc(Number(ttl) || 60)),
    graph:
      strategyType === 'custom' && editorModeTab === 'flow' ? flowGraph : null,
  });

  const validateMutation = useMutation({
    mutationFn: () =>
      SmartDNSService.validatePolicy({
        strategy_type: strategyType,
        config: buildConfigPayload(),
        jdm_json:
          strategyType === 'custom' && editorModeTab === 'jdm'
            ? jdmJSON.trim()
            : '',
      }),
    onSuccess: (res) => {
      if (res.compiled_jdm) {
        setJdmJSON((previous) =>
          editorModeTab === 'jdm' || !previous.trim()
            ? res.compiled_jdm
            : previous,
        );
      }
      toast.success(t('validatedToast'));
    },
    onError: (err) => toast.error(getErrorMessage(err)),
  });

  const saveMutation = useMutation({
    mutationFn: async () => {
      const payload = {
        name: name.trim(),
        description: description.trim(),
        scope_type: scopeType,
        scope_domains: scopeType === 'domains' ? splitList(scopeDomains) : [],
        strategy_type: strategyType,
        priority: Math.trunc(Number(priority) || 100),
        ttl: Math.max(10, Math.trunc(Number(ttl) || 60)),
        max_answers: Math.max(1, Math.trunc(Number(maxAnswers) || 1)),
        enabled,
        is_default: isDefault,
        fallback_strategy: fallbackStrategy,
        fallback_node_ids:
          fallbackStrategy === 'static_nodes' ? splitList(fallbackNodeIDs) : [],
        config: buildConfigPayload(),
        jdm_json:
          strategyType === 'custom' && editorModeTab === 'jdm'
            ? jdmJSON.trim()
            : '',
      };
      if (editing) {
        return SmartDNSService.updatePolicy(editing.id, payload);
      }
      return SmartDNSService.createPolicy(payload);
    },
    onSuccess: async () => {
      toast.success(editing ? t('updated') : t('created'));
      setEditorOpen(false);
      await refresh();
    },
    onError: (err) => toast.error(getErrorMessage(err)),
  });

  const setDefaultMutation = useMutation({
    mutationFn: (id: number) => SmartDNSService.setDefaultPolicy(id),
    onSuccess: async () => {
      setDefaultTarget(null);
      toast.success(t('defaultUpdated'));
      await refresh();
    },
    onError: (err) => toast.error(getErrorMessage(err)),
  });

  const deleteMutation = useMutation({
    mutationFn: (id: number) => SmartDNSService.deletePolicy(id),
    onSuccess: async () => {
      toast.success(t('deleted'));
      setDeleteTarget(null);
      await refresh();
    },
    onError: (err) => toast.error(getErrorMessage(err)),
  });

  const openCreate = () => {
    setEditing(null);
    setName('');
    setDescription('');
    setScopeType('all');
    setScopeDomains('');
    setStrategyType('smart');
    setPriority('100');
    setTTL('60');
    setMaxAnswers('1');
    setEnabled(true);
    setIsDefault(false);
    setFallbackStrategy('smart');
    setFallbackNodeIDs('');
    setPreset(defaultRules);
    setFlowGraph(createDefaultFlowGraph('smart'));
    setJdmJSON('');
    setEditorModeTab('preset');
    setSection('basics');
    setEditorOpen(true);
  };

  const openEdit = (p: SmartDNSPolicy) => {
    setEditing(p);
    setName(p.name);
    setDescription(p.description ?? '');
    setScopeType(p.scope_type === 'domains' ? 'domains' : 'all');
    setScopeDomains((p.scope_domains ?? []).join(', '));
    const st =
      p.strategy_type === 'load_balance'
        ? 'load_balance'
        : p.strategy_type === 'custom'
          ? 'custom'
          : 'smart';
    setStrategyType(st);
    setPriority(String(p.priority));
    setTTL(String(p.ttl));
    setMaxAnswers(String(p.max_answers));
    setEnabled(p.enabled);
    setIsDefault(p.is_default);
    setFallbackStrategy(
      p.fallback_strategy && p.fallback_strategy !== 'lowest_load'
        ? p.fallback_strategy
        : 'smart',
    );
    setFallbackNodeIDs((p.fallback_node_ids ?? []).join(', '));
    setPreset({
      latency: String(p.config?.weights?.latency ?? smartWeights.latency),
      load: String(p.config?.weights?.load ?? smartWeights.load),
      geo: String(p.config?.weights?.geo ?? smartWeights.geo),
      useTagWeights: p.config?.use_tag_weights ?? true,
      countries: (p.config?.filters?.countries ?? []).join(', '),
      tags: (p.config?.filters?.tags ?? []).join(', '),
      groups: (p.config?.filters?.network_groups ?? []).join(', '),
    });
    setFlowGraph(
      p.config?.graph && p.config.graph.nodes.length > 0
        ? p.config.graph
        : createDefaultFlowGraph(st),
    );
    setJdmJSON(p.jdm_json ?? '');
    setEditorModeTab(
      st === 'custom'
        ? p.config?.graph?.nodes?.length
          ? 'flow'
          : 'jdm'
        : 'preset',
    );
    setSection('basics');
    setEditorOpen(true);
  };

  const invalidRules =
    strategyType === 'custom' && editorModeTab === 'jdm' && !jdmJSON.trim();
  const formBusy = saveMutation.isPending || validateMutation.isPending;

  return (
    <div className='flex flex-col gap-4'>
      <div className='flex flex-wrap items-center justify-between gap-3'>
        <div>
          <h2 className='text-lg font-semibold'>{t('listTitle')}</h2>
          <p className='text-sm text-muted-foreground'>{t('listDesc')}</p>
        </div>
        <Button size='sm' onClick={openCreate}>
          <Plus data-icon='inline-start' />
          {t('create')}
        </Button>
      </div>

      {policiesQuery.isPending ? (
        <LoadingStateWithBorder icon={GitBranch} description={t('loading')} />
      ) : policiesQuery.isError ? (
        <ErrorInline
          message={getErrorMessage(policiesQuery.error)}
          onRetry={() => void policiesQuery.refetch()}
        />
      ) : (
        <SmartDNSPolicyList
          policies={policiesQuery.data ?? []}
          onEdit={openEdit}
          onSimulate={onSimulatePolicy}
          onSetDefault={setDefaultTarget}
          onDelete={setDeleteTarget}
        />
      )}

      <Dialog
        open={editorOpen}
        onOpenChange={(open) => !formBusy && setEditorOpen(open)}
      >
        <DialogContent className='max-h-[calc(100dvh-2rem)] grid-rows-[auto_minmax(0,1fr)_auto] sm:max-w-4xl md:max-w-4xl'>
          <DialogHeader>
            <DialogTitle>
              {editing ? t('editTitle') : t('createTitle')}
            </DialogTitle>
            <DialogDescription>{t('dialogDesc')}</DialogDescription>
          </DialogHeader>

          <Tabs
            value={section}
            onValueChange={setSection}
            className='min-h-0 overflow-y-auto px-1'
          >
            <TabsList variant='line'>
              <TabsTrigger value='basics' disabled={formBusy}>
                {t('tabBasics')}
              </TabsTrigger>
              <TabsTrigger value='routing' disabled={formBusy}>
                {t('tabRouting')}
              </TabsTrigger>
            </TabsList>
            <TabsContent value='basics' className='pt-3 pb-1'>
              <div className='grid gap-4 sm:grid-cols-2'>
                <div className='space-y-1.5'>
                  <Label htmlFor='sdns-pol-name'>{t('name')}</Label>
                  <Input
                    id='sdns-pol-name'
                    value={name}
                    onChange={(e) => setName(e.target.value)}
                  />
                </div>
                <div className='space-y-1.5'>
                  <Label htmlFor='sdns-pol-strategy'>{t('strategyType')}</Label>
                  <Select
                    value={strategyType}
                    onValueChange={(v) => {
                      const next = v as 'smart' | 'load_balance' | 'custom';
                      setStrategyType(next);
                      if (next === 'custom') {
                        setEditorModeTab('flow');
                        return;
                      }
                      setEditorModeTab('preset');
                      setPreset((previous) => ({
                        ...previous,
                        ...(next === 'load_balance'
                          ? balanceWeights
                          : smartWeights),
                      }));
                    }}
                  >
                    <SelectTrigger id='sdns-pol-strategy' className='w-full'>
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value='smart'>
                        {t('strategySmart')}
                      </SelectItem>
                      <SelectItem value='load_balance'>
                        {t('strategyLB')}
                      </SelectItem>
                      <SelectItem value='custom'>
                        {t('strategyCustomJdm')}
                      </SelectItem>
                    </SelectContent>
                  </Select>
                </div>
                <div className='space-y-1.5 sm:col-span-2'>
                  <Label htmlFor='sdns-pol-desc'>{t('descLabel')}</Label>
                  <Input
                    id='sdns-pol-desc'
                    value={description}
                    onChange={(e) => setDescription(e.target.value)}
                  />
                </div>

                <div className='space-y-1.5'>
                  <Label htmlFor='sdns-pol-scope'>{t('scopeType')}</Label>
                  <Select
                    value={scopeType}
                    onValueChange={(v) => setScopeType(v as 'all' | 'domains')}
                  >
                    <SelectTrigger id='sdns-pol-scope' className='w-full'>
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value='all'>{t('scopeAll')}</SelectItem>
                      <SelectItem value='domains'>
                        {t('scopeDomains')}
                      </SelectItem>
                    </SelectContent>
                  </Select>
                </div>
                <div className='grid grid-cols-3 gap-2'>
                  <div className='space-y-1.5'>
                    <Label htmlFor='sdns-pol-priority'>{t('priority')}</Label>
                    <Input
                      id='sdns-pol-priority'
                      type='number'
                      value={priority}
                      onChange={(e) => setPriority(e.target.value)}
                    />
                  </div>
                  <div className='space-y-1.5'>
                    <Label htmlFor='sdns-pol-ttl'>TTL</Label>
                    <Input
                      id='sdns-pol-ttl'
                      type='number'
                      min={10}
                      max={86400}
                      value={ttl}
                      onChange={(e) => setTTL(e.target.value)}
                    />
                  </div>
                  <div className='space-y-1.5'>
                    <Label htmlFor='sdns-pol-max'>{t('maxAnswers')}</Label>
                    <Input
                      id='sdns-pol-max'
                      type='number'
                      min={1}
                      max={16}
                      value={maxAnswers}
                      onChange={(e) => setMaxAnswers(e.target.value)}
                    />
                  </div>
                </div>

                {scopeType === 'domains' ? (
                  <div className='space-y-1.5 sm:col-span-2'>
                    <Label htmlFor='sdns-pol-domains'>
                      {t('scopeDomainsList')}
                    </Label>
                    <Input
                      id='sdns-pol-domains'
                      value={scopeDomains}
                      placeholder='*.example.com, app.example.com'
                      onChange={(e) => setScopeDomains(e.target.value)}
                    />
                  </div>
                ) : null}

                <div className='space-y-1.5'>
                  <Label htmlFor='sdns-pol-fb'>{t('fallbackStrategy')}</Label>
                  <Select
                    value={fallbackStrategy}
                    onValueChange={setFallbackStrategy}
                  >
                    <SelectTrigger id='sdns-pol-fb' className='w-full'>
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value='smart'>{t('fbSmart')}</SelectItem>
                      <SelectItem value='load_balance'>
                        {t('fbLoadBalance')}
                      </SelectItem>
                      <SelectItem value='static_nodes'>
                        {t('fbStaticNodes')}
                      </SelectItem>
                      <SelectItem value='all_healthy'>
                        {t('fbAllHealthy')}
                      </SelectItem>
                    </SelectContent>
                  </Select>
                </div>
                {fallbackStrategy === 'static_nodes' ? (
                  <div className='space-y-1.5'>
                    <Label htmlFor='sdns-pol-fb-nodes'>{t('fbNodeIds')}</Label>
                    <Input
                      id='sdns-pol-fb-nodes'
                      value={fallbackNodeIDs}
                      placeholder='node-1, node-2'
                      onChange={(e) => setFallbackNodeIDs(e.target.value)}
                    />
                  </div>
                ) : null}
                <div className='flex items-center justify-between gap-3 rounded-lg border border-dashed px-3 py-2 sm:col-span-2'>
                  <Label htmlFor='sdns-policy-enabled'>
                    {t('enablePolicy')}
                  </Label>
                  <Switch
                    id='sdns-policy-enabled'
                    checked={enabled}
                    onCheckedChange={setEnabled}
                    disabled={formBusy}
                  />
                </div>
              </div>
            </TabsContent>

            <TabsContent value='routing' className='pt-3 pb-1'>
              <Tabs
                value={editorModeTab}
                onValueChange={(v) =>
                  setEditorModeTab(v as 'preset' | 'flow' | 'jdm')
                }
              >
                {strategyType === 'custom' && (
                  <div className='flex flex-col items-start gap-2'>
                    <p className='text-sm text-muted-foreground'>
                      {t('editingSourceHint')}
                    </p>
                    <TabsList>
                      <TabsTrigger value='flow' disabled={formBusy}>
                        {t('tabFlow')}
                      </TabsTrigger>
                      <TabsTrigger value='jdm' disabled={formBusy}>
                        {t('tabJdmJson')}
                      </TabsTrigger>
                    </TabsList>
                  </div>
                )}

                <TabsContent value='preset' className='pt-2'>
                  <PresetRuleFields
                    rules={preset}
                    onChange={(patch) =>
                      setPreset((previous) => ({ ...previous, ...patch }))
                    }
                  />
                </TabsContent>

                <TabsContent value='flow' className='pt-2'>
                  <PolicyFlowEditor
                    graph={flowGraph}
                    onChange={setFlowGraph}
                    validating={formBusy}
                    onValidate={() => !formBusy && validateMutation.mutate()}
                  />
                </TabsContent>

                <TabsContent value='jdm' className='space-y-2 pt-2'>
                  <div className='flex items-center justify-between'>
                    <Label htmlFor='sdns-policy-jdm'>{t('jdmJsonLabel')}</Label>
                    <Button
                      type='button'
                      variant='outline'
                      size='sm'
                      disabled={formBusy || invalidRules}
                      onClick={() => validateMutation.mutate()}
                    >
                      <CheckCircle2 data-icon='inline-start' />
                      {t('validateJson')}
                    </Button>
                  </div>
                  <Textarea
                    id='sdns-policy-jdm'
                    rows={12}
                    className='font-mono text-xs'
                    value={jdmJSON}
                    onChange={(e) => setJdmJSON(e.target.value)}
                  />
                </TabsContent>
              </Tabs>
            </TabsContent>
          </Tabs>

          <DialogFooter>
            <Button
              variant='outline'
              disabled={formBusy}
              onClick={() => setEditorOpen(false)}
            >
              {tc('cancel')}
            </Button>
            <Button
              disabled={formBusy || !name.trim() || invalidRules}
              onClick={() => saveMutation.mutate()}
            >
              {t('save')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <AlertDialog
        open={defaultTarget !== null}
        onOpenChange={(open) =>
          !open && !setDefaultMutation.isPending && setDefaultTarget(null)
        }
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t('setDefault')}</AlertDialogTitle>
            <AlertDialogDescription>
              {t('setDefaultDescription', { name: defaultTarget?.name ?? '' })}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={setDefaultMutation.isPending}>
              {tc('cancel')}
            </AlertDialogCancel>
            <Button
              disabled={setDefaultMutation.isPending}
              onClick={() =>
                defaultTarget && setDefaultMutation.mutate(defaultTarget.id)
              }
            >
              {t('setDefault')}
            </Button>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      <AlertDialog
        open={deleteTarget !== null}
        onOpenChange={(open) =>
          !open && !deleteMutation.isPending && setDeleteTarget(null)
        }
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t('deleteTitle')}</AlertDialogTitle>
            <AlertDialogDescription>
              {t('deleteDesc', { name: deleteTarget?.name ?? '' })}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={deleteMutation.isPending}>
              {tc('cancel')}
            </AlertDialogCancel>
            <Button
              variant='destructive'
              disabled={deleteMutation.isPending}
              onClick={() =>
                deleteTarget && deleteMutation.mutate(deleteTarget.id)
              }
            >
              {t('confirmDelete')}
            </Button>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  );
}
