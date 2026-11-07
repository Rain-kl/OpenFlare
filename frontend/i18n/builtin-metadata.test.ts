import { describe, expect, it } from 'vitest';
import english from '@/messages/fragments/builtin-metadata.en.json';
import chinese from '@/messages/fragments/builtin-metadata.zh-CN.json';
import russian from '@/messages/fragments/builtin-metadata.ru.json';
import {
  localizeConfigDescription,
  localizeTemplateMetadata,
} from './builtin-metadata';

describe('built-in metadata presentation', () => {
  it('localizes known descriptions in both directions without changing stored data', () => {
    const original = {
      key: 'site_name',
      description: chinese.builtinMetadata.configDescriptions.site_name,
      value: 'My site',
    };
    expect(
      localizeConfigDescription(original.key, original.description, 'en'),
    ).toBe(english.builtinMetadata.configDescriptions.site_name);
    expect(
      localizeConfigDescription(original.key, 'Platform display name', 'zh-CN'),
    ).toBe(original.description);
    expect(original.description).toBe(
      chinese.builtinMetadata.configDescriptions.site_name,
    );
    expect(original.value).toBe('My site');
    expect(
      localizeConfigDescription(original.key, original.description, 'ru'),
    ).toBe(russian.builtinMetadata.configDescriptions.site_name);
    expect(
      localizeConfigDescription(
        original.key,
        russian.builtinMetadata.configDescriptions.site_name,
        'en',
      ),
    ).toBe(english.builtinMetadata.configDescriptions.site_name);
  });

  it('preserves custom descriptions, edited values and unknown configuration keys', () => {
    expect(
      localizeConfigDescription('site_name', 'Custom description', 'zh-CN'),
    ).toBe('Custom description');
    expect(localizeConfigDescription('site_name', '', 'en')).toBe('');
    expect(
      localizeConfigDescription(
        'unknown',
        chinese.builtinMetadata.configDescriptions.site_name,
        'en',
      ),
    ).toBe(chinese.builtinMetadata.configDescriptions.site_name);
  });

  it('only localizes the exact defaults of built-in templates', () => {
    const original = chinese.builtinMetadata.templates.login_email;
    expect(
      localizeTemplateMetadata(
        'login_email',
        true,
        'name',
        original.name,
        'en',
      ),
    ).toBe(english.builtinMetadata.templates.login_email.name);
    expect(
      localizeTemplateMetadata(
        'login_email',
        true,
        'description',
        original.description,
        'en',
      ),
    ).toBe(english.builtinMetadata.templates.login_email.description);
    expect(
      localizeTemplateMetadata(
        'login_email',
        false,
        'name',
        original.name,
        'en',
      ),
    ).toBe(original.name);
    expect(
      localizeTemplateMetadata(
        'login_email',
        true,
        'name',
        'My verification email',
        'zh-CN',
      ),
    ).toBe('My verification email');
    expect(
      localizeTemplateMetadata('unknown', true, 'name', original.name, 'en'),
    ).toBe(original.name);
  });
});
