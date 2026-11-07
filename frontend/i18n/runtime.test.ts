import { afterEach, describe, expect, it } from 'vitest';
import { createTranslator, type AbstractIntlMessages } from 'next-intl';
import { NetworkError, isCancelError } from '@/lib/services/core/errors';
import { buildTaskPayload } from '@/lib/task-param-utils';
import { effectiveOfflinePageHTML } from '@/lib/openflare/offline-page-templates';
import { findGraphErrorTarget } from '@/app/(main)/waf/rules/editor/components/editor-behavior';
import { isLogStorageDisabledMessage } from '@/app/(main)/admin/logs/components/log-storage-error';
import { searchItems } from '@/lib/utils/search-data';
import english from '@/messages/en.json';
import chinese from '@/messages/zh-CN.json';
import { locales } from './config';
import { getRuntimeTranslator } from './runtime';

afterEach(() => {
  document.cookie = 'NEXT_LOCALE=; Max-Age=0; Path=/';
});

describe('imperative runtime translations', () => {
  it('recognizes disabled log storage independently of the current language', () => {
    expect(
      isLogStorageDisabledMessage(
        'Log storage is disabled; analytics are unavailable',
      ),
    ).toBe(true);
    expect(
      isLogStorageDisabledMessage('日志存储未启用，无法查询访问日志'),
    ).toBe(true);
    expect(isLogStorageDisabledMessage('Unexpected network error')).toBe(false);
  });
  it('uses the current locale when creating errors and preserves field values', () => {
    document.cookie = 'NEXT_LOCALE=en; Path=/';
    expect(new NetworkError().message).toBe(
      'Network connection failed; check your network',
    );
    const result = buildTaskPayload(
      [
        {
          name: 'value',
          label: '自定义名称',
          type: 'number',
          required: false,
          placeholder: '',
          description: '',
        },
      ],
      { value: 'invalid' },
    );
    expect(result.error).toBe('自定义名称 must be a valid number');
    document.cookie = 'NEXT_LOCALE=zh-CN; Path=/';
    expect(new NetworkError().message).toBe('网络连接失败，请检查您的网络');
  });

  it('recognizes historical canceled requests independently of the current locale', () => {
    document.cookie = 'NEXT_LOCALE=en; Path=/';
    expect(isCancelError(new Error('请求已被取消'))).toBe(true);
    document.cookie = 'NEXT_LOCALE=zh-CN; Path=/';
    expect(isCancelError(new Error('Request canceled'))).toBe(true);
    expect(isCancelError(new Error('Other error'))).toBe(false);
  });

  it('selects offline HTML at use time without rewriting custom HTML', () => {
    document.cookie = 'NEXT_LOCALE=en; Path=/';
    expect(effectiveOfflinePageHTML('')).toContain('lang="en"');
    document.cookie = 'NEXT_LOCALE=zh-CN; Path=/';
    expect(effectiveOfflinePageHTML('')).toContain('lang="zh-CN"');
    const custom = '<p>用户内容</p>';
    expect(effectiveOfflinePageHTML(custom)).toBe(custom);
  });

  it('finds graph targets in both current English and historical Chinese errors', () => {
    for (const message of [
      'Node node-1 contains invalid IP',
      '节点 node-1 包含无效 IP',
    ]) {
      expect(findGraphErrorTarget(new Error(message), ['node-1'], [])).toEqual({
        kind: 'node',
        id: 'node-1',
      });
    }
    for (const message of [
      'Edge edge-1 has invalid source',
      'Ребро edge-1 ссылается на отсутствующий исходный узел other',
      '边 ID edge-1 的端点无效',
    ]) {
      expect(findGraphErrorTarget(new Error(message), [], ['edge-1'])).toEqual({
        kind: 'edge',
        id: 'edge-1',
      });
    }
  });

  it('keeps Chinese search aliases in the Chinese catalog', () => {
    const zh = createTranslator({
      locale: 'zh-CN',
      messages: chinese as AbstractIntlMessages,
      namespace: 'layout',
    });
    const en = createTranslator({
      locale: 'en',
      messages: english as AbstractIntlMessages,
      namespace: 'layout',
    });
    expect(
      searchItems('日志', true, zh).some((item) => item.url === '/admin/logs'),
    ).toBe(true);
    expect(
      searchItems('terminal', true, en).some(
        (item) => item.url === '/admin/logs',
      ),
    ).toBe(true);
  });

  it('formats interpolated messages in all catalogs', () => {
    for (const locale of locales) {
      const t = getRuntimeTranslator(locale);
      for (const [key, text] of Object.entries(english.runtime)) {
        if (/\{value\d+(?:\}|,)/.test(text)) {
          const translated = t(key as keyof typeof english.runtime, {
            value0: 42,
            value1: 'VALUE',
            value2: 42,
          });
          expect(translated).toContain('42');
          expect(translated).not.toContain('{value0}');
        }
      }
    }
  });
});
