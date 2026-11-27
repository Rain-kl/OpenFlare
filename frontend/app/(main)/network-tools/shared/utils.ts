import { getFormattingLocale } from '@/i18n/format';
import type {
  NetToolsNodeResultStatus,
  NetToolsTaskStatus,
} from '@/lib/services/openflare';

export function getNetToolsErrorMessage(
  err: unknown,
  fallback: string,
): string {
  if (err instanceof Error && err.message) {
    return err.message;
  }
  return fallback;
}

export function formatNetToolsMs(value?: number | null): string {
  if (value === undefined || value === null || !Number.isFinite(value)) {
    return '—';
  }
  return value.toFixed(1);
}

export function formatNetToolsPercent(value?: number | null): string {
  if (value === undefined || value === null || !Number.isFinite(value)) {
    return '—';
  }
  return value.toFixed(1);
}

export function formatNetToolsBytes(bytes?: number | null): string {
  if (!bytes || bytes <= 0) {
    return '0 B';
  }
  const units = ['B', 'KiB', 'MiB', 'GiB'];
  const idx = Math.min(
    Math.floor(Math.log(bytes) / Math.log(1024)),
    units.length - 1,
  );
  return `${(bytes / 1024 ** idx).toFixed(1)} ${units[idx]}`;
}

export interface NetToolsTargetHost {
  host: string;
  port: number | null;
}

/**
 * Extracts the bare host (hostname or IP literal) plus an optional explicit
 * port from a user-supplied probe target, dropping URL schemes, credentials,
 * paths, queries, and fragments so pasted URLs can be probed directly.
 *
 * The host is normalized through the URL parser when possible (lowercasing,
 * IDN punycode, IPv6 unbracketing) while the port is read from the raw input
 * so explicitly typed scheme-default ports are preserved.
 *
 * Returns null when no usable host remains; the caller should keep the raw
 * input in that case and let server-side validation surface the error.
 */
export function extractNetToolsTargetHost(
  raw: string,
): NetToolsTargetHost | null {
  const trimmed = raw.trim();
  if (!trimmed) {
    return null;
  }

  const authoritySource = trimmed.includes('://')
    ? trimmed.slice(trimmed.indexOf('://') + 3)
    : trimmed;
  const manual = extractHostFromAuthority(authoritySource);
  if (!manual) {
    return null;
  }

  const bracketed = manual.host.includes(':')
    ? `[${manual.host}]`
    : manual.host;
  try {
    const parsed = new URL(`http://${bracketed}`);
    const host = parsed.hostname.replace(/^\[|\]$/g, '');
    if (host) {
      return { host, port: manual.port };
    }
  } catch {
    // Not URL-normalizable (bare IPv6 or malformed input); keep as extracted.
  }
  return manual;
}

function extractHostFromAuthority(
  candidate: string,
): NetToolsTargetHost | null {
  // Keep only the authority component: cut path, query, and fragment.
  let authority = candidate.split(/[/?#\\]/, 1)[0];
  // Drop userinfo (user:pass@) when present.
  const atIdx = authority.lastIndexOf('@');
  if (atIdx !== -1) {
    authority = authority.slice(atIdx + 1);
  }
  authority = authority.trim();
  if (!authority) {
    return null;
  }

  // Bracketed IPv6 literal, optionally followed by :port.
  if (authority.startsWith('[')) {
    const closeIdx = authority.indexOf(']');
    if (closeIdx === -1) {
      return null;
    }
    const host = authority.slice(1, closeIdx);
    if (!host) {
      return null;
    }
    const portPart = authority.slice(closeIdx + 1);
    return {
      host,
      port: portPart.startsWith(':')
        ? parseNetToolsPort(portPart.slice(1))
        : null,
    };
  }

  // Treat host:port only when a single colon separates a numeric port; bare
  // IPv6 literals contain multiple colons and stay intact.
  if ((authority.match(/:/g) ?? []).length === 1) {
    const colonIdx = authority.indexOf(':');
    const hostPart = authority.slice(0, colonIdx);
    const port = parseNetToolsPort(authority.slice(colonIdx + 1));
    if (hostPart && port !== null) {
      return { host: hostPart, port };
    }
  }

  return { host: authority, port: null };
}

function parseNetToolsPort(raw: string): number | null {
  if (!/^\d+$/.test(raw)) {
    return null;
  }
  const port = Number(raw);
  return port >= 1 && port <= 65535 ? port : null;
}

export function formatNetToolsTime(value?: string | null): string {
  if (!value) return '—';
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return value;
  }
  return date.toLocaleString(getFormattingLocale());
}

export function getTaskStatusVariant(
  status: NetToolsTaskStatus | NetToolsNodeResultStatus,
): 'default' | 'secondary' | 'destructive' | 'outline' {
  switch (status) {
    case 'completed':
      return 'default';
    case 'running':
    case 'pending':
      return 'secondary';
    case 'failed':
    case 'timeout':
      return 'destructive';
    default:
      return 'outline';
  }
}
