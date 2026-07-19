import {
  Field,
  FieldDescription,
  FieldLabel,
  FieldTitle,
} from '@/components/ui/field';
import { Input } from '@/components/ui/input';
import { ToggleGroup, ToggleGroupItem } from '@/components/ui/toggle-group';
import type { PagesGitHubReleaseSelector } from '@/lib/services/openflare';

export interface PagesGitHubSourceFormValue {
  repositoryURL: string;
  releaseSelector: PagesGitHubReleaseSelector;
  releaseTag: string;
  assetName: string;
}

export interface PagesGitHubSourceFormErrors {
  repository: string;
  releaseTag: string;
  assetName: string;
}

interface PagesSourceGitHubFieldsProps {
  value: PagesGitHubSourceFormValue;
  errors: PagesGitHubSourceFormErrors;
  defaultAssetName: string;
  onChange: (value: PagesGitHubSourceFormValue) => void;
  onErrorsChange: (errors: PagesGitHubSourceFormErrors) => void;
}

export function PagesSourceGitHubFields({
  value,
  errors,
  defaultAssetName,
  onChange,
  onErrorsChange,
}: PagesSourceGitHubFieldsProps) {
  return (
    <>
      <Field data-invalid={Boolean(errors.repository)}>
        <FieldLabel htmlFor='pages-github-repository'>
          GitHub 仓库 URL
        </FieldLabel>
        <Input
          id='pages-github-repository'
          type='url'
          placeholder='https://github.com/owner/repo'
          value={value.repositoryURL}
          aria-invalid={Boolean(errors.repository)}
          autoComplete='off'
          onChange={(event) => {
            onChange({ ...value, repositoryURL: event.target.value });
            onErrorsChange({ ...errors, repository: '' });
          }}
        />
        <FieldDescription>
          {errors.repository || '仅支持公开 github.com 仓库。'}
        </FieldDescription>
      </Field>

      <Field>
        <FieldTitle id='pages-github-selector'>Release 选择</FieldTitle>
        <ToggleGroup
          type='single'
          variant='outline'
          value={value.releaseSelector}
          aria-labelledby='pages-github-selector'
          className='grid w-full grid-cols-2'
          onValueChange={(selector) => {
            if (selector === 'latest' || selector === 'tag') {
              onChange({ ...value, releaseSelector: selector });
              onErrorsChange({ ...errors, releaseTag: '' });
            }
          }}
        >
          <ToggleGroupItem value='latest' className='w-full'>
            最新 Release
          </ToggleGroupItem>
          <ToggleGroupItem value='tag' className='w-full'>
            固定 Tag
          </ToggleGroupItem>
        </ToggleGroup>
        <FieldDescription>
          当前阶段由管理员手动检查并决定是否发布。
        </FieldDescription>
      </Field>

      {value.releaseSelector === 'tag' ? (
        <Field data-invalid={Boolean(errors.releaseTag)}>
          <FieldLabel htmlFor='pages-github-tag'>Release tag</FieldLabel>
          <Input
            id='pages-github-tag'
            placeholder='v1.2.3'
            value={value.releaseTag}
            aria-invalid={Boolean(errors.releaseTag)}
            autoComplete='off'
            onChange={(event) => {
              onChange({ ...value, releaseTag: event.target.value });
              onErrorsChange({ ...errors, releaseTag: '' });
            }}
          />
          <FieldDescription>
            {errors.releaseTag || '精确检查并同步指定 tag。'}
          </FieldDescription>
        </Field>
      ) : null}

      <Field data-invalid={Boolean(errors.assetName)}>
        <FieldLabel htmlFor='pages-github-asset'>
          Release Asset 文件名
        </FieldLabel>
        <Input
          id='pages-github-asset'
          placeholder={defaultAssetName}
          value={value.assetName}
          aria-invalid={Boolean(errors.assetName)}
          autoComplete='off'
          onChange={(event) => {
            onChange({ ...value, assetName: event.target.value });
            onErrorsChange({ ...errors, assetName: '' });
          }}
        />
        <FieldDescription>
          {errors.assetName || '按文件名精确匹配已上传的 Release Asset。'}
        </FieldDescription>
      </Field>
    </>
  );
}
