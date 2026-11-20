import { useFormatter, useTranslations } from 'next-intl';
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
  type AttackCategory,
  type AttackLevel,
} from '@/lib/services/openflare/attack-types';

function appliesAtLevel(
  rule: (typeof catalog.rules)[number],
  level: AttackLevel,
) {
  return level !== 'off' && (level !== 'balanced' || rule.balanced);
}

export function countEnabledRules(
  category: AttackCategory,
  level: AttackLevel,
) {
  return catalog.rules.filter(
    (rule) => rule.category === category && appliesAtLevel(rule, level),
  ).length;
}

export function AttackRuleCatalog({
  children,
  category,
  level,
}: {
  children: ReactNode;
  category?: AttackCategory;
  level?: AttackLevel;
}) {
  return (
    <Dialog>
      <DialogTrigger asChild>{children}</DialogTrigger>
      <DialogContent className='sm:max-w-3xl md:max-w-3xl'>
        <AttackRuleCatalogContent category={category} level={level} />
      </DialogContent>
    </Dialog>
  );
}

function AttackRuleCatalogContent({
  category,
  level,
}: {
  category?: AttackCategory;
  level?: AttackLevel;
}) {
  const t = useTranslations('attackProtection');
  const logs = useTranslations('attackLogs');
  const format = useFormatter();
  const [selected, setSelected] = useState<string>(category ?? 'all');
  const [query, setQuery] = useState('');
  const keyword = query.trim().toLowerCase();
  const rules = catalog.rules.filter((rule) => {
    if (selected !== 'all' && rule.category !== selected) return false;
    return (
      String(rule.id).includes(keyword) ||
      rule.message.toLowerCase().includes(keyword) ||
      (!rule.paranoia &&
        logs.has(`rules.${rule.id}`) &&
        logs(`rules.${rule.id}`).toLowerCase().includes(keyword))
    );
  });

  return (
    <>
      <DialogHeader>
        <DialogTitle>{t('catalog.title')}</DialogTitle>
        <DialogDescription>
          {t('catalog.version', { version: catalog.crs_version })}
        </DialogDescription>
      </DialogHeader>
      <div className='flex flex-wrap gap-2'>
        {!category && (
          <Select value={selected} onValueChange={setSelected}>
            <SelectTrigger aria-label={t('catalog.category')}>
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
        <Input
          className='min-w-44 flex-1'
          aria-label={t('catalog.search')}
          placeholder={t('catalog.search')}
          value={query}
          onChange={(event) => setQuery(event.target.value)}
        />
      </div>
      <p className='text-sm text-muted-foreground'>
        {category && level
          ? t('catalog.categorySummary', {
              category: t(`categories.${category}`),
              level: t(`levels.${level}`),
              count: format.number(countEnabledRules(category, level)),
            })
          : t('catalog.ruleCount', { count: format.number(rules.length) })}
      </p>
      <div className='min-w-0 rounded-md border border-dashed'>
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>{t('catalog.id')}</TableHead>
              {!category && <TableHead>{t('catalog.category')}</TableHead>}
              <TableHead>{t('catalog.message')}</TableHead>
              <TableHead>{t('catalog.ruleLevel')}</TableHead>
              {level && <TableHead>{t('catalog.status')}</TableHead>}
            </TableRow>
          </TableHeader>
          <TableBody>
            {rules.map((rule) => (
              <TableRow key={rule.id}>
                <TableCell>
                  <code>{rule.id}</code>
                </TableCell>
                {!category && (
                  <TableCell className='whitespace-normal'>
                    {t(`categories.${rule.category as AttackCategory}`)}
                  </TableCell>
                )}
                <TableCell className='min-w-36 whitespace-normal'>
                  {!rule.paranoia && logs.has(`rules.${rule.id}`)
                    ? logs(`rules.${rule.id}`)
                    : rule.message}
                </TableCell>
                <TableCell>
                  <Badge variant='outline'>
                    {rule.paranoia
                      ? `PL${rule.paranoia}`
                      : t('catalog.supplemental')}
                  </Badge>
                  {!rule.paranoia && (
                    <span className='ml-2 text-xs text-muted-foreground'>
                      {t(
                        rule.balanced
                          ? 'catalog.baseRule'
                          : 'catalog.extendedRule',
                      )}
                    </span>
                  )}
                </TableCell>
                {level && (
                  <TableCell>
                    {t(`levels.${appliesAtLevel(rule, level) ? level : 'off'}`)}
                  </TableCell>
                )}
              </TableRow>
            ))}
            {rules.length === 0 && (
              <TableRow>
                <TableCell
                  colSpan={4}
                  className='whitespace-normal text-center text-muted-foreground'
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
