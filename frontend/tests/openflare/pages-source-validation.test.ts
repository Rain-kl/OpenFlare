import { describe, expect, it } from 'vitest';

import {
  validGitHubAssetName,
  validGitHubReleaseTag,
  validGitHubRepositoryURL,
} from '@/app/(main)/pages/detail/components/pages-source-validation';

describe('Pages GitHub source validation', () => {
  it('accepts only canonical public GitHub repository URLs', () => {
    expect(validGitHubRepositoryURL('https://github.com/acme/site')).toBe(true);
    expect(validGitHubRepositoryURL('https://GitHub.com/acme/site.git')).toBe(
      true,
    );

    const invalid = [
      'http://github.com/acme/site',
      'https://github.com//acme/site',
      'https://github.com/acme/site/',
      'https://github.com/acme/site/extra',
      'https://github.com/acme/./site',
      'https://github.com/acme/../site',
      'https://github.com/acme/%73ite',
      'https://github.com/acme/site?token=secret',
      'https://github.com:443/acme/site',
      String.raw`https://github.com/acme\site`,
    ];
    for (const value of invalid) {
      expect(validGitHubRepositoryURL(value), value).toBe(false);
    }
  });

  it('mirrors Git ref rules while preserving legal release tag characters', () => {
    const valid = [
      '@',
      'release/v1#stable&channel=prod',
      'foo.LOCK',
      '中文/发布=稳定',
    ];
    for (const value of valid) {
      expect(validGitHubReleaseTag(value), value).toBe(true);
    }

    const invalid = [
      '',
      'release v1',
      'release~v1',
      'release^v1',
      'release:v1',
      'release?v1',
      'release*v1',
      'release[v1',
      String.raw`release\v1`,
      'release..v1',
      'release@{v1',
      'release//v1',
      '/release',
      'release/',
      'release.',
      '.release',
      'release/.candidate',
      'release/v1.lock',
      'release\nsecret',
      'release\u2028secret',
      'release\u202esecret',
      'a'.repeat(256),
    ];
    for (const value of invalid) {
      expect(validGitHubReleaseTag(value), value).toBe(false);
    }
  });

  it('preserves exact legal asset names and rejects path or display controls', () => {
    const valid = [
      'dist.zip',
      'dist?channel=stable&part#1.zip',
      ' dist.zip ',
      'build=production.zip',
    ];
    for (const value of valid) {
      expect(validGitHubAssetName(value), value).toBe(true);
    }

    const invalid = [
      '',
      '.',
      '..',
      '../dist.zip',
      String.raw`dir\dist.zip`,
      'dist\n.zip',
      'dist\u2028.zip',
      'dist\u202e.zip',
      'a'.repeat(256),
      '\ud800',
    ];
    for (const value of invalid) {
      expect(validGitHubAssetName(value), value).toBe(false);
    }
  });
});
