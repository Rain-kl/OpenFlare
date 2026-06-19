'use client';

import {useId, useMemo} from 'react';
import {Link2, Minus, Plus} from 'lucide-react';

import {Badge} from '@/components/ui/badge';
import {Button} from '@/components/ui/button';
import {Input} from '@/components/ui/input';
import {Select, SelectContent, SelectItem, SelectTrigger, SelectValue,} from '@/components/ui/select';
import type {TlsCertificateItem} from '@/lib/services/openflare';
import {WebsiteService} from '@/lib/services/openflare';
import {validateDomain} from '../../components/helpers';
import type {DomainListRow} from '../helpers';

function ensureRows(rows: DomainListRow[]) {
  return rows.length > 0 ? rows : [{ domain: '', certificateId: '' }];
}

function buildSuggestionSources(domains: string[]) {
  const values = new Set<string>();

  for (const domain of domains) {
    const normalized = domain.trim().toLowerCase().replace(/^\*\./, '');
    if (!normalized) {
      continue;
    }

    values.add(normalized);

    const segments = normalized.split('.');
    for (let index = 1; index < segments.length - 1; index += 1) {
      values.add(segments.slice(index).join('.'));
    }
  }

  return Array.from(values);
}

function buildDomainSuggestions(input: string, sources: string[], rows: DomainListRow[]) {
  const normalizedInput = input.trim().toLowerCase();
  if (!normalizedInput) {
    return [];
  }

  const existingDomains = new Set(
    rows
      .map((row) => row.domain.trim().toLowerCase())
      .filter((row) => row && row !== normalizedInput),
  );
  const suggestions: string[] = [];

  for (const source of sources) {
    if (source.startsWith(normalizedInput) && source !== normalizedInput) {
      suggestions.push(source);
    }

    const separatorIndex = normalizedInput.lastIndexOf('.');
    if (separatorIndex <= 0) {
      continue;
    }

    const prefix = normalizedInput.slice(0, separatorIndex);
    const suffixInput = normalizedInput.slice(separatorIndex + 1);

    if (!suffixInput || source.startsWith(suffixInput)) {
      suggestions.push(`${prefix}.${source}`);
    }
  }

  return suggestions.filter((suggestion, index) => {
    return (
      suggestion !== normalizedInput &&
      !existingDomains.has(suggestion) &&
      suggestions.indexOf(suggestion) === index
    );
  });
}

interface DomainListInputProps {
  rows: DomainListRow[];
  onChange: (rows: DomainListRow[]) => void;
  onBlur?: () => void;
  suggestionSources?: string[];
  certificates?: TlsCertificateItem[];
  domainPlaceholder?: string;
}

export function DomainListInput({
  rows,
  onChange,
  onBlur,
  suggestionSources = [],
  certificates = [],
  domainPlaceholder = 'app.example.com',
}: DomainListInputProps) {
  const listId = useId();
  const safeRows = ensureRows(rows);
  const normalizedSources = useMemo(
    () => buildSuggestionSources(suggestionSources),
    [suggestionSources],
  );

  const updateRows = (nextRows: DomainListRow[]) => {
    onChange(ensureRows(nextRows));
  };

  const applyManagedDomainCertificate = async (index: number, domain: string) => {
    const normalized = domain.trim().toLowerCase();
    if (!normalized || validateDomain(normalized)) {
      return;
    }

    try {
      const result = await WebsiteService.match(normalized);
      if (!result.matched || !result.candidate?.certificate_id) {
        return;
      }

      const nextRows = safeRows.slice();
      if (!nextRows[index].certificateId) {
        nextRows[index] = {
          ...nextRows[index],
          certificateId: String(result.candidate.certificate_id),
        };
        updateRows(nextRows);
      }
    } catch {
      // Ignore match failures; user can still pick a certificate manually.
    }
  };

  return (
    <div className="space-y-4">
      {safeRows.map((row, index) => {
        const suggestions = buildDomainSuggestions(row.domain, normalizedSources, safeRows).slice(
          0,
          4,
        );

        return (
          <div key={`${index}-${safeRows.length}`} className="space-y-2">
            <div className="grid gap-3 md:grid-cols-[40px_minmax(0,1fr)_220px] md:items-start">
              <Button
                type="button"
                variant="outline"
                size="icon"
                className="size-9 shrink-0"
                disabled={safeRows.length === 1}
                aria-label={`删除域名输入框 ${index + 1}`}
                onClick={() => {
                  if (safeRows.length === 1) {
                    updateRows([{ domain: '', certificateId: '' }]);
                    return;
                  }
                  updateRows(safeRows.filter((_, rowIndex) => rowIndex !== index));
                }}
              >
                <Minus className="size-3.5" />
              </Button>

              <div className="min-w-0 space-y-2">
                <Input
                  value={row.domain}
                  list={`${listId}-${index}`}
                  aria-label={`域名 ${index + 1}`}
                  placeholder={index === 0 ? domainPlaceholder : 'www.example.com'}
                  onBlur={() => {
                    onBlur?.();
                    void applyManagedDomainCertificate(index, row.domain);
                  }}
                  onChange={(event) => {
                    const nextRows = safeRows.slice();
                    nextRows[index] = {
                      ...nextRows[index],
                      domain: event.target.value,
                    };
                    updateRows(nextRows);
                  }}
                />
                <datalist id={`${listId}-${index}`}>
                  {suggestions.map((suggestion) => (
                    <option key={suggestion} value={suggestion} />
                  ))}
                </datalist>

                {suggestions.length > 0 ? (
                  <div className="flex flex-wrap gap-1.5">
                    {suggestions.map((suggestion) => (
                      <button
                        key={suggestion}
                        type="button"
                        className="rounded-full border px-2.5 py-0.5 text-[11px] text-muted-foreground transition hover:border-primary hover:text-foreground"
                        onClick={() => {
                          const nextRows = safeRows.slice();
                          nextRows[index] = {
                            ...nextRows[index],
                            domain: suggestion,
                          };
                          updateRows(nextRows);
                          void applyManagedDomainCertificate(index, suggestion);
                        }}
                      >
                        {suggestion}
                      </button>
                    ))}
                  </div>
                ) : null}
              </div>

              <Select
                value={row.certificateId || 'none'}
                onValueChange={(value) => {
                  const nextRows = safeRows.slice();
                  nextRows[index] = {
                    ...nextRows[index],
                    certificateId: value === 'none' ? '' : value,
                  };
                  updateRows(nextRows);
                }}
              >
                <SelectTrigger className="h-9 w-full">
                  <SelectValue
                    placeholder={certificates.length === 0 ? '暂无可选证书' : '选择证书'}
                  />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="none">
                    {certificates.length === 0 ? '暂无可选证书' : '不绑定证书'}
                  </SelectItem>
                  {certificates.map((certificate) => (
                    <SelectItem key={certificate.id} value={String(certificate.id)}>
                      {certificate.name}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
          </div>
        );
      })}

      <div className="flex flex-wrap items-center gap-2">
        <Button
          type="button"
          variant="outline"
          size="sm"
          className="h-8 gap-1.5 text-xs"
          onClick={() => {
            updateRows([...safeRows, { domain: '', certificateId: '' }]);
          }}
        >
          <Plus className="size-3.5" />
          添加域名
        </Button>
        <Badge variant="outline" className="gap-1 text-[10px] font-normal">
          <Link2 className="size-3" />
          输入域名后将自动匹配托管域名证书
        </Badge>
      </div>
    </div>
  );
}
