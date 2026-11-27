'use client';

import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import {
  CheckCircle2,
  GitBranch,
  Play,
  Plus,
  Star,
  Trash2,
} from 'lucide-react';
import { useTranslations } from 'next-intl';
import { useState } from 'react';
import { toast } from 'sonner';

import { ErrorInline } from '@/components/layout/error';
import { LoadingStateWithBorder } from '@/components/layout/loading';
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card';
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
import { createDefaultFlowGraph, PolicyFlowEditor } from './policy-flow-editor';

function splitList(raw: string): string[] {
  return raw
    .split(/[\n,]+/)
    .map((s) => s.trim())
    .filter(Boolean);
}

export function SmartDNSPoliciesTab({
  onSimulatePolicy,
}: {
  onSimulatePolicy: (policyId: number) => void;
}) {
  const t = useTranslations('smartdns.policies');
  const tc = useTranslations('common');
  const queryClient = useQueryClient();

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
    'smart' | 'load_balance' | 'custom_jdm'
  >('smart');
  const [priority, setPriority] = useState('100');
  const [ttl, setTTL] = useState('60');
  const [maxAnswers, setMaxAnswers] = useState('1');
  const [enabled, setEnabled] = useState(true);
  const [isDefault, setIsDefault] = useState(false);
  const [fallbackStrategy, setFallbackStrategy] = useState('lowest_load');
  const [fallbackNodeIDs, setFallbackNodeIDs] = useState('');

  const [wLatency, setWLatency] = useState('0.55');
  const [wLoad, setWLoad] = useState('0.30');
  const [wGeo, setWGeo] = useState('0.15');
  const [useTagWeights, setUseTagWeights] = useState(true);
  const [filterCountries, setFilterCountries] = useState('');
  const [filterTags, setFilterTags] = useState('');
  const [filterGroups, setFilterGroups] = useState('');
  const [flowGraph, setFlowGraph] = useState<SmartDNSFlowGraph>(() =>
    createDefaultFlowGraph('smart'),
  );
  const [jdmJSON, setJdmJSON] = useState('');

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
      latency: Number(wLatency) || 0,
      load: Number(wLoad) || 0,
      geo: Number(wGeo) || 0,
    },
    use_tag_weights: useTagWeights,
    filters: {
      node_ids: [],
      countries: splitList(filterCountries),
      tags: splitList(filterTags),
      network_groups: splitList(filterGroups),
    },
    max_answers: Math.max(1, Math.trunc(Number(maxAnswers) || 1)),
    ttl: Math.max(10, Math.trunc(Number(ttl) || 60)),
    graph: strategyType === 'custom_jdm' ? flowGraph : null,
  });

  const validateMutation = useMutation({
    mutationFn: () =>
      SmartDNSService.validatePolicy({
        strategy_type: strategyType,
        config: buildConfigPayload(),
        jdm_json: editorModeTab === 'jdm' ? jdmJSON : '',
      }),
    onSuccess: (res) => {
      if (res.compiled_jdm) {
        setJdmJSON(res.compiled_jdm);
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
        fallback_node_ids: splitList(fallbackNodeIDs),
        config: buildConfigPayload(),
        jdm_json: editorModeTab === 'jdm' ? jdmJSON : '',
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
    setFallbackStrategy('lowest_load');
    setFallbackNodeIDs('');
    setWLatency('0.55');
    setWLoad('0.30');
    setWGeo('0.15');
    setUseTagWeights(true);
    setFilterCountries('');
    setFilterTags('');
    setFilterGroups('');
    setFlowGraph(createDefaultFlowGraph('smart'));
    setJdmJSON('');
    setEditorModeTab('preset');
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
        : p.strategy_type === 'custom_jdm'
          ? 'custom_jdm'
          : 'smart';
    setStrategyType(st);
    setPriority(String(p.priority));
    setTTL(String(p.ttl));
    setMaxAnswers(String(p.max_answers));
    setEnabled(p.enabled);
    setIsDefault(p.is_default);
    setFallbackStrategy(p.fallback_strategy || 'lowest_load');
    setFallbackNodeIDs((p.fallback_node_ids ?? []).join(', '));
    setWLatency(String(p.config?.weights?.latency ?? 0.55));
    setWLoad(String(p.config?.weights?.load ?? 0.3));
    setWGeo(String(p.config?.weights?.geo ?? 0.15));
    setUseTagWeights(p.config?.use_tag_weights ?? true);
    setFilterCountries((p.config?.filters?.countries ?? []).join(', '));
    setFilterTags((p.config?.filters?.tags ?? []).join(', '));
    setFilterGroups((p.config?.filters?.network_groups ?? []).join(', '));
    setFlowGraph(
      p.config?.graph && p.config.graph.nodes.length > 0
        ? p.config.graph
        : createDefaultFlowGraph(st),
    );
    setJdmJSON(p.jdm_json ?? '');
    setEditorModeTab(
      st === 'custom_jdm' && p.config?.graph?.nodes?.length ? 'flow' : 'preset',
    );
    setEditorOpen(true);
  };

  return (
    <div className='space-y-6'>
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

      {policiesQuery.isLoading ? (
        <LoadingStateWithBorder icon={GitBranch} description={t('loading')} />
      ) : policiesQuery.isError ? (
        <ErrorInline
          message={getErrorMessage(policiesQuery.error)}
          onRetry={() => void policiesQuery.refetch()}
        />
      ) : (policiesQuery.data ?? []).length === 0 ? (
        <Card>
          <CardContent className='py-10 text-center text-sm text-muted-foreground'>
            {t('empty')}
          </CardContent>
        </Card>
      ) : (
        <div className='grid gap-4 lg:grid-cols-2'>
          {(policiesQuery.data ?? []).map((p) => (
            <Card key={p.id}>
              <CardHeader>
                <div className='flex items-start justify-between gap-2'>
                  <div>
                    <CardTitle className='flex items-center gap-2 text-base'>
                      {p.name}
                      {p.is_default ? (
                        <Badge variant='default'>{t('defaultBadge')}</Badge>
                      ) : null}
                    </CardTitle>
                    <CardDescription>
                      {p.description ||
                        (p.scope_type === 'domains'
                          ? p.scope_domains.join(', ')
                          : t('scopeAll'))}
                    </CardDescription>
                  </div>
                  <div className='flex items-center gap-1.5'>
                    <Badge variant='outline'>{p.strategy_type}</Badge>
                    <Badge variant={p.enabled ? 'default' : 'secondary'}>
                      {p.enabled ? t('enabled') : t('disabled')}
                    </Badge>
                  </div>
                </div>
              </CardHeader>
              <CardContent className='space-y-3'>
                <div className='flex flex-wrap gap-2 text-xs text-muted-foreground'>
                  <span>TTL {p.ttl}s</span>
                  <span>·</span>
                  <span>
                    {t('maxAnswersSummary', { count: p.max_answers })}
                  </span>
                  <span>·</span>
                  <span>{t('prioritySummary', { value: p.priority })}</span>
                  <span>·</span>
                  <span>rev {p.revision}</span>
                </div>
                <div className='flex flex-wrap gap-2'>
                  <Button
                    variant='outline'
                    size='sm'
                    onClick={() => openEdit(p)}
                  >
                    {t('edit')}
                  </Button>
                  <Button
                    variant='outline'
                    size='sm'
                    onClick={() => onSimulatePolicy(p.id)}
                  >
                    <Play data-icon='inline-start' />
                    {t('simulate')}
                  </Button>
                  {!p.is_default ? (
                    <Button
                      variant='outline'
                      size='sm'
                      onClick={() => setDefaultMutation.mutate(p.id)}
                    >
                      <Star data-icon='inline-start' />
                      {t('setDefault')}
                    </Button>
                  ) : null}
                  {!p.is_default ? (
                    <Button
                      variant='destructive'
                      size='sm'
                      onClick={() => setDeleteTarget(p)}
                    >
                      <Trash2 data-icon='inline-start' />
                      {t('delete')}
                    </Button>
                  ) : null}
                </div>
              </CardContent>
            </Card>
          ))}
        </div>
      )}

      <Dialog open={editorOpen} onOpenChange={setEditorOpen}>
        <DialogContent className='max-h-[92vh] overflow-y-auto sm:max-w-4xl'>
          <DialogHeader>
            <DialogTitle>
              {editing ? t('editTitle') : t('createTitle')}
            </DialogTitle>
            <DialogDescription>{t('dialogDesc')}</DialogDescription>
          </DialogHeader>

          <div className='space-y-4'>
            <div className='grid gap-3 sm:grid-cols-2'>
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
                    const next = v as 'smart' | 'load_balance' | 'custom_jdm';
                    setStrategyType(next);
                    if (next === 'custom_jdm') {
                      setEditorModeTab('flow');
                    } else if (next === 'load_balance') {
                      setWLatency('0');
                      setWLoad('1');
                      setWGeo('0');
                    } else {
                      setWLatency('0.55');
                      setWLoad('0.30');
                      setWGeo('0.15');
                    }
                  }}
                >
                  <SelectTrigger id='sdns-pol-strategy' className='w-full'>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value='smart'>{t('strategySmart')}</SelectItem>
                    <SelectItem value='load_balance'>
                      {t('strategyLB')}
                    </SelectItem>
                    <SelectItem value='custom_jdm'>
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
                    <SelectItem value='domains'>{t('scopeDomains')}</SelectItem>
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
                    <SelectItem value='lowest_load'>
                      {t('fbLowestLoad')}
                    </SelectItem>
                    <SelectItem value='static_nodes'>
                      {t('fbStaticNodes')}
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
              ) : (
                <div className='flex items-center justify-between rounded-lg border border-dashed px-3 py-2'>
                  <Label>{t('enabled')}</Label>
                  <Switch checked={enabled} onCheckedChange={setEnabled} />
                </div>
              )}
            </div>

            <Tabs
              value={editorModeTab}
              onValueChange={(v) =>
                setEditorModeTab(v as 'preset' | 'flow' | 'jdm')
              }
            >
              <TabsList>
                <TabsTrigger value='preset'>{t('tabPreset')}</TabsTrigger>
                <TabsTrigger value='flow'>{t('tabFlow')}</TabsTrigger>
                <TabsTrigger value='jdm'>{t('tabJdmJson')}</TabsTrigger>
              </TabsList>

              <TabsContent value='preset' className='space-y-3 pt-2'>
                <div className='grid gap-3 sm:grid-cols-3'>
                  <div className='space-y-1.5'>
                    <Label>{t('wLatency')}</Label>
                    <Input
                      type='number'
                      step='0.05'
                      value={wLatency}
                      onChange={(e) => setWLatency(e.target.value)}
                    />
                  </div>
                  <div className='space-y-1.5'>
                    <Label>{t('wLoad')}</Label>
                    <Input
                      type='number'
                      step='0.05'
                      value={wLoad}
                      onChange={(e) => setWLoad(e.target.value)}
                    />
                  </div>
                  <div className='space-y-1.5'>
                    <Label>{t('wGeo')}</Label>
                    <Input
                      type='number'
                      step='0.05'
                      value={wGeo}
                      onChange={(e) => setWGeo(e.target.value)}
                    />
                  </div>
                </div>
                <div className='flex items-center justify-between rounded-lg border border-dashed px-3 py-2'>
                  <Label>{t('useTagWeights')}</Label>
                  <Switch
                    checked={useTagWeights}
                    onCheckedChange={setUseTagWeights}
                  />
                </div>
                <div className='grid gap-3 sm:grid-cols-3'>
                  <div className='space-y-1.5'>
                    <Label>{t('filterCountries')}</Label>
                    <Input
                      value={filterCountries}
                      placeholder='CN, SG'
                      onChange={(e) => setFilterCountries(e.target.value)}
                    />
                  </div>
                  <div className='space-y-1.5'>
                    <Label>{t('filterTags')}</Label>
                    <Input
                      value={filterTags}
                      placeholder='optimized, bgp'
                      onChange={(e) => setFilterTags(e.target.value)}
                    />
                  </div>
                  <div className='space-y-1.5'>
                    <Label>{t('filterGroups')}</Label>
                    <Input
                      value={filterGroups}
                      placeholder='cn2-gia'
                      onChange={(e) => setFilterGroups(e.target.value)}
                    />
                  </div>
                </div>
              </TabsContent>

              <TabsContent value='flow' className='pt-2'>
                <PolicyFlowEditor
                  graph={flowGraph}
                  onChange={(next) => {
                    setFlowGraph(next);
                    if (strategyType !== 'custom_jdm') {
                      setStrategyType('custom_jdm');
                    }
                  }}
                  validating={validateMutation.isPending}
                  onValidate={() => validateMutation.mutate()}
                />
              </TabsContent>

              <TabsContent value='jdm' className='space-y-2 pt-2'>
                <div className='flex items-center justify-between'>
                  <Label>{t('jdmJsonLabel')}</Label>
                  <Button
                    type='button'
                    variant='outline'
                    size='sm'
                    disabled={validateMutation.isPending}
                    onClick={() => validateMutation.mutate()}
                  >
                    <CheckCircle2 data-icon='inline-start' />
                    {t('validateJson')}
                  </Button>
                </div>
                <Textarea
                  rows={12}
                  className='font-mono text-xs'
                  value={jdmJSON}
                  onChange={(e) => setJdmJSON(e.target.value)}
                />
              </TabsContent>
            </Tabs>
          </div>

          <DialogFooter>
            <Button variant='outline' onClick={() => setEditorOpen(false)}>
              {tc('cancel')}
            </Button>
            <Button
              disabled={saveMutation.isPending || !name.trim()}
              onClick={() => saveMutation.mutate()}
            >
              {t('save')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <AlertDialog
        open={deleteTarget !== null}
        onOpenChange={(open) => !open && setDeleteTarget(null)}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t('deleteTitle')}</AlertDialogTitle>
            <AlertDialogDescription>
              {t('deleteDesc', { name: deleteTarget?.name ?? '' })}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{tc('cancel')}</AlertDialogCancel>
            <AlertDialogAction
              className='bg-destructive text-white hover:bg-destructive/90'
              disabled={deleteMutation.isPending}
              onClick={() =>
                deleteTarget && deleteMutation.mutate(deleteTarget.id)
              }
            >
              {t('confirmDelete')}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  );
}
