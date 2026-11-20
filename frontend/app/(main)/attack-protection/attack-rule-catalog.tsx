import { useTranslations } from 'next-intl';
import { useState, type ReactNode } from 'react';

import { Badge } from '@/components/ui/badge';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from '@/components/ui/dialog';
import { Input } from '@/components/ui/input';
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table';
import catalog from '@/lib/openflare/attack-rule-catalog.generated.json';
import {
  attackCategories,
  attackRuleActions,
  type AttackCategory,
  type AttackGroupInput,
  type AttackLevel,
  type AttackRuleAction,
} from '@/lib/services/openflare/attack-types';

type Rule = (typeof catalog.rules)[number];
type Preferences = Pick<AttackGroupInput, 'levels' | 'rule_overrides'>;
type CatalogEditor = Preferences & {
  onRuleChange: (id: number, action: AttackRuleAction | undefined) => void;
};

function defaultAction(rule: Rule, level: AttackLevel): AttackRuleAction {
  if (level === 'off' || (level === 'balanced' && !rule.balanced)) return 'off';
  return level === 'observe' ? 'observe' : 'enabled';
}

export function ruleCounts(
  preferences: Preferences,
  category?: AttackCategory,
) {
  let active = 0;
  let overrides = 0;
  for (const rule of catalog.rules) {
    if (category && rule.category !== category) continue;
    const override = preferences.rule_overrides?.[rule.id];
    const level = preferences.levels[rule.category as AttackCategory] ?? 'off';
    if (override !== undefined) overrides++;
    if ((override ?? defaultAction(rule, level)) !== 'off') active++;
  }
  return { active, overrides };
}

export function AttackRuleCatalog({
  children,
  category,
  editor,
}: {
  children: ReactNode;
  category?: AttackCategory;
  editor?: CatalogEditor;
}) {
  return (
    <Dialog>
      <DialogTrigger asChild>{children}</DialogTrigger>
      <DialogContent className='grid max-h-[min(46rem,calc(100dvh-1.5rem))] min-w-0 grid-rows-[auto_auto_minmax(0,1fr)] overflow-hidden p-4 sm:max-w-4xl sm:p-6 md:max-w-4xl'>
        <AttackRuleCatalogContent category={category} editor={editor} />
      </DialogContent>
    </Dialog>
  );
}

function AttackRuleCatalogContent({
  category,
  editor,
}: {
  category?: AttackCategory;
  editor?: CatalogEditor;
}) {
  const t = useTranslations('attackProtection');
  const logs = useTranslations('attackLogs');
  const [selected, setSelected] = useState<string>(category ?? 'all');
  const [query, setQuery] = useState('');
  const keyword = query.trim().toLowerCase();
  const rules = catalog.rules.flatMap((rule) => {
    if (selected !== 'all' && rule.category !== selected) return [];
    const message =
      !rule.paranoia && logs.has(`rules.${rule.id}`)
        ? logs(`rules.${rule.id}`)
        : rule.message;
    if (
      !String(rule.id).includes(keyword) &&
      !rule.message.toLowerCase().includes(keyword) &&
      !message.toLowerCase().includes(keyword)
    )
      return [];
    return [{ ...rule, message }];
  });

  return (
    <>
      <DialogHeader className='pr-6'>
        <DialogTitle>{t('catalog.title')}</DialogTitle>
        <DialogDescription>
          {t('catalog.version', { version: catalog.crs_version })}
          {category && ` · ${t(`categories.${category}`)}`}
        </DialogDescription>
        {editor && (
          <p className='text-left text-xs text-muted-foreground'>
            {t('catalog.editHelp')}
          </p>
        )}
      </DialogHeader>
      <div className='flex flex-wrap items-center gap-2'>
        <Input
          className='min-w-40 flex-1'
          aria-label={t('catalog.search')}
          placeholder={t('catalog.search')}
          value={query}
          onChange={(event) => setQuery(event.target.value)}
        />
        {!category && (
          <Select value={selected} onValueChange={setSelected}>
            <SelectTrigger
              aria-label={t('catalog.category')}
              className='max-w-full'
            >
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectGroup>
                <SelectItem value='all'>
                  {t('catalog.allCategories')}
                </SelectItem>
                {attackCategories.map((item) => (
                  <SelectItem key={item} value={item}>
                    {t(`categories.${item}`)}
                  </SelectItem>
                ))}
              </SelectGroup>
            </SelectContent>
          </Select>
        )}
        <p
          className='text-xs whitespace-nowrap text-muted-foreground'
          role='status'
        >
          {t('catalog.ruleCount', { count: rules.length })}
        </p>
      </div>
      <div className='grid min-w-0 grid-rows-[minmax(0,1fr)] overflow-hidden rounded-md border border-dashed'>
        <Table>
          <TableHeader className='sticky top-0 z-10 bg-background'>
            <TableRow>
              <TableHead className='w-24 pl-3'>{t('catalog.id')}</TableHead>
              <TableHead>{t('catalog.message')}</TableHead>
              {editor && (
                <TableHead className='w-1 pr-3'>
                  {t('catalog.status')}
                </TableHead>
              )}
            </TableRow>
          </TableHeader>
          <TableBody>
            {rules.map((rule) => (
              <TableRow key={rule.id}>
                <TableCell className='align-top pl-3'>
                  <div className='flex flex-col items-start gap-1'>
                    <code className='tabular-nums'>{rule.id}</code>
                    <Badge variant='outline'>
                      {rule.paranoia
                        ? `PL${rule.paranoia}`
                        : t(
                            rule.balanced
                              ? 'catalog.baseRule'
                              : 'catalog.extendedRule',
                          )}
                    </Badge>
                  </div>
                </TableCell>
                <TableCell className='whitespace-normal align-top'>
                  <p className='wrap-anywhere'>{rule.message}</p>
                  {!category && (
                    <p className='mt-1 text-xs text-muted-foreground'>
                      {t(`categories.${rule.category as AttackCategory}`)}
                    </p>
                  )}
                </TableCell>
                {editor && (
                  <TableCell className='align-top pr-3'>
                    <RuleActionSelect rule={rule} editor={editor} />
                  </TableCell>
                )}
              </TableRow>
            ))}
            {rules.length === 0 && (
              <TableRow>
                <TableCell
                  colSpan={editor ? 3 : 2}
                  className='py-8 text-center text-muted-foreground'
                >
                  {t('catalog.noRules')}
                </TableCell>
              </TableRow>
            )}
          </TableBody>
        </Table>
      </div>
    </>
  );
}

function RuleActionSelect({
  rule,
  editor,
}: {
  rule: Rule;
  editor: CatalogEditor;
}) {
  const t = useTranslations('attackProtection');
  const level = editor.levels[rule.category as AttackCategory] ?? 'off';
  const inherited = defaultAction(rule, level);
  const override = editor.rule_overrides?.[rule.id];

  return (
    <div className='flex flex-col items-start gap-1'>
      <Select
        value={override ?? 'inherit'}
        onValueChange={(value: AttackRuleAction | 'inherit') => {
          editor.onRuleChange(rule.id, value === 'inherit' ? undefined : value);
        }}
      >
        <SelectTrigger
          size='sm'
          aria-label={t('catalog.ruleAction', { id: String(rule.id) })}
          className='w-full'
        >
          <SelectValue>{t(`ruleActions.${override ?? inherited}`)}</SelectValue>
        </SelectTrigger>
        <SelectContent align='end'>
          <SelectGroup>
            <SelectItem value='inherit'>
              {t('catalog.inherit', { action: t(`ruleActions.${inherited}`) })}
            </SelectItem>
            {attackRuleActions.map((action) => (
              <SelectItem key={action} value={action}>
                {t(`ruleActions.${action}`)}
              </SelectItem>
            ))}
          </SelectGroup>
        </SelectContent>
      </Select>
      <span className='text-xs text-muted-foreground'>
        {t(override ? 'catalog.custom' : 'catalog.inherited')}
      </span>
    </div>
  );
}
