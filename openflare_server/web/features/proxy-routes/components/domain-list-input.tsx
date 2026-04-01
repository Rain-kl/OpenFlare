'use client';

import { useId, useMemo } from 'react';
import { Minus, Plus } from 'lucide-react';

import type { TlsCertificateItem } from '@/features/tls-certificates/types';
import {
  ResourceInput,
  ResourceSelect,
  SecondaryButton,
} from '@/features/shared/components/resource-primitives';
export type DomainListRow = {
  domain: string;
  certificateId: string;
};

const actionButtonBaseClassName = 'h-11 w-11 shrink-0 rounded-2xl px-0';
const removeButtonClassName =
  'border-[var(--border-default)] bg-[var(--surface-elevated)] text-[var(--foreground-secondary)] hover:border-[var(--status-danger-border)] hover:bg-[var(--status-danger-soft)] hover:text-[var(--status-danger-foreground)] disabled:border-[var(--border-default)] disabled:bg-[var(--surface-muted)] disabled:text-[var(--foreground-muted)]';
const addButtonClassName =
  'border-dashed border-[var(--border-default)] bg-[var(--surface-muted)] text-[var(--foreground-secondary)] hover:border-[var(--brand-primary)] hover:bg-[var(--brand-primary-soft)] hover:text-[var(--brand-primary)]';

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

function buildDomainSuggestions(
  input: string,
  sources: string[],
  rows: DomainListRow[],
) {
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

export function buildDomainRowsFromRoute(
  domains: string[],
  domainCertIDs: number[],
  certIDs: number[],
): DomainListRow[] {
  if (domains.length === 0) {
    return ensureRows([]);
  }

  if (domainCertIDs.length === domains.length) {
    return domains.map((domain, index) => ({
      domain,
      certificateId: domainCertIDs[index]
        ? String(domainCertIDs[index])
        : '',
    }));
  }

  if (certIDs.length === 0) {
    return domains.map((domain) => ({ domain, certificateId: '' }));
  }

  if (certIDs.length === 1) {
    return domains.map((domain) => ({
      domain,
      certificateId: String(certIDs[0]),
    }));
  }

  return domains.map((domain, index) => ({
    domain,
    certificateId: certIDs[index] ? String(certIDs[index]) : '',
  }));
}

export function DomainListInput({
  rows,
  onChange,
  onBlur,
  suggestionSources = [],
  certificates = [],
  domainPlaceholder = 'app.example.com',
}: {
  rows: DomainListRow[];
  onChange: (rows: DomainListRow[]) => void;
  onBlur?: () => void;
  suggestionSources?: string[];
  certificates?: TlsCertificateItem[];
  domainPlaceholder?: string;
}) {
  const listId = useId();
  const safeRows = ensureRows(rows);
  const normalizedSources = useMemo(
    () => buildSuggestionSources(suggestionSources),
    [suggestionSources],
  );

  const updateRows = (nextRows: DomainListRow[]) => {
    onChange(ensureRows(nextRows));
  };

  return (
    <div className="space-y-4">
      {safeRows.map((row, index) => {
        const suggestions = buildDomainSuggestions(
          row.domain,
          normalizedSources,
          safeRows,
        ).slice(0, 4);

        return (
          <div key={`${index}-${safeRows.length}`} className="space-y-2">
            <div className="grid gap-3 md:grid-cols-[44px_minmax(0,1fr)_280px] md:items-start">
              <SecondaryButton
                type="button"
                aria-label={`删除域名输入框 ${index + 1}`}
                className={`${actionButtonBaseClassName} ${removeButtonClassName}`}
                disabled={safeRows.length === 1}
                onClick={() => {
                  if (safeRows.length === 1) {
                    updateRows([{ domain: '', certificateId: '' }]);
                    return;
                  }

                  updateRows(
                    safeRows.filter((_, rowIndex) => rowIndex !== index),
                  );
                }}
              >
                <Minus aria-hidden="true" className="h-[14px] w-[14px]" />
              </SecondaryButton>

              <div className="min-w-0 space-y-2">
                <ResourceInput
                  value={row.domain}
                  list={`${listId}-${index}`}
                  aria-label={`域名 ${index + 1}`}
                  placeholder={index === 0 ? domainPlaceholder : 'www.example.com'}
                  onBlur={onBlur}
                  onChange={(event) => {
                    const nextRows = safeRows.slice();
                    nextRows[index] = {
                      ...nextRows[index],
                      domain: event.target.value,
                    };
                    updateRows(nextRows);
                  }}
                  className="h-12"
                />
                <datalist id={`${listId}-${index}`}>
                  {suggestions.map((suggestion) => (
                    <option key={suggestion} value={suggestion} />
                  ))}
                </datalist>

                {suggestions.length > 0 ? (
                  <div className="flex flex-wrap gap-2">
                    {suggestions.map((suggestion) => (
                      <button
                        key={suggestion}
                        type="button"
                        className="inline-flex items-center rounded-full border border-[var(--border-default)] bg-[var(--surface-panel)] px-3 py-1 text-xs text-[var(--foreground-secondary)] transition hover:border-[var(--border-strong)] hover:text-[var(--foreground-primary)]"
                        onClick={() => {
                          const nextRows = safeRows.slice();
                          nextRows[index] = {
                            ...nextRows[index],
                            domain: suggestion,
                          };
                          updateRows(nextRows);
                        }}
                      >
                        {suggestion}
                      </button>
                    ))}
                  </div>
                ) : null}
              </div>

              <ResourceSelect
                aria-label={`证书 ${index + 1}`}
                value={row.certificateId}
                onChange={(event) => {
                  const nextRows = safeRows.slice();
                  nextRows[index] = {
                    ...nextRows[index],
                    certificateId: event.target.value,
                  };
                  updateRows(nextRows);
                }}
                className="h-12"
              >
                <option value="">
                  {certificates.length === 0 ? '暂无可选证书' : '选择证书'}
                </option>
                {certificates.map((certificate) => (
                  <option key={certificate.id} value={certificate.id}>
                    {certificate.name}
                  </option>
                ))}
              </ResourceSelect>
            </div>
          </div>
        );
      })}

      <SecondaryButton
        type="button"
        aria-label="新增域名输入框"
        className={`${actionButtonBaseClassName} ${addButtonClassName}`}
        onClick={() => {
          updateRows([...safeRows, { domain: '', certificateId: '' }]);
        }}
      >
        <Plus aria-hidden="true" className="h-[14px] w-[14px]" />
      </SecondaryButton>
    </div>
  );
}
