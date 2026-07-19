import {
  Field,
  FieldContent,
  FieldDescription,
  FieldError,
  FieldLabel,
  FieldTitle,
} from '@/components/ui/field';
import { Input } from '@/components/ui/input';
import { Switch } from '@/components/ui/switch';
import { ToggleGroup, ToggleGroupItem } from '@/components/ui/toggle-group';
import type { PagesGitHubReleaseSelector } from '@/lib/services/openflare';

export interface PagesGitHubSourceFormValue {
  repositoryURL: string;
  releaseSelector: PagesGitHubReleaseSelector;
  releaseTag: string;
  assetName: string;
  autoUpdateEnabled: boolean;
  checkIntervalMinutes: string;
}

export interface PagesGitHubSourceFormErrors {
  repository: string;
  releaseTag: string;
  assetName: string;
  checkInterval: string;
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
          aria-describedby='pages-github-repository-description pages-github-repository-error'
          autoComplete='off'
          onChange={(event) => {
            onChange({ ...value, repositoryURL: event.target.value });
            onErrorsChange({ ...errors, repository: '' });
          }}
        />
        <FieldDescription id='pages-github-repository-description'>
          仅支持公开 github.com 仓库。
        </FieldDescription>
        <FieldError id='pages-github-repository-error'>
          {errors.repository}
        </FieldError>
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
              onChange({
                ...value,
                releaseSelector: selector,
                autoUpdateEnabled:
                  selector === 'latest' ? value.autoUpdateEnabled : false,
              });
              onErrorsChange({
                ...errors,
                releaseTag: '',
                checkInterval: '',
              });
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
      </Field>

      {value.releaseSelector === 'tag' ? (
        <Field data-invalid={Boolean(errors.releaseTag)}>
          <FieldLabel htmlFor='pages-github-tag'>Release tag</FieldLabel>
          <Input
            id='pages-github-tag'
            placeholder='v1.2.3'
            value={value.releaseTag}
            aria-invalid={Boolean(errors.releaseTag)}
            aria-describedby='pages-github-tag-description pages-github-tag-error'
            autoComplete='off'
            onChange={(event) => {
              onChange({ ...value, releaseTag: event.target.value });
              onErrorsChange({ ...errors, releaseTag: '' });
            }}
          />
          <FieldDescription id='pages-github-tag-description'>
            精确检查并同步指定 tag。
          </FieldDescription>
          <FieldError id='pages-github-tag-error'>
            {errors.releaseTag}
          </FieldError>
        </Field>
      ) : null}

      {value.releaseSelector === 'latest' ? (
        <>
          <Field orientation='horizontal'>
            <FieldContent>
              <FieldLabel htmlFor='pages-github-auto-update'>
                自动更新
              </FieldLabel>
              <FieldDescription>
                检查到新的 Release 后自动同步并发布。
              </FieldDescription>
            </FieldContent>
            <Switch
              id='pages-github-auto-update'
              checked={value.autoUpdateEnabled}
              onCheckedChange={(checked) =>
                onChange({ ...value, autoUpdateEnabled: checked })
              }
            />
          </Field>

          <Field data-invalid={Boolean(errors.checkInterval)}>
            <FieldLabel htmlFor='pages-github-check-interval'>
              检查间隔（分钟）
            </FieldLabel>
            <Input
              id='pages-github-check-interval'
              type='number'
              min={5}
              max={1440}
              step={1}
              inputMode='numeric'
              value={value.checkIntervalMinutes}
              aria-invalid={Boolean(errors.checkInterval)}
              aria-describedby='pages-github-check-interval-description pages-github-check-interval-error'
              onChange={(event) => {
                onChange({
                  ...value,
                  checkIntervalMinutes: event.target.value,
                });
                onErrorsChange({ ...errors, checkInterval: '' });
              }}
            />
            <FieldDescription id='pages-github-check-interval-description'>
              可设置为 5–1440 分钟。
            </FieldDescription>
            <FieldError id='pages-github-check-interval-error'>
              {errors.checkInterval}
            </FieldError>
          </Field>
        </>
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
          aria-describedby='pages-github-asset-description pages-github-asset-error'
          autoComplete='off'
          onChange={(event) => {
            onChange({ ...value, assetName: event.target.value });
            onErrorsChange({ ...errors, assetName: '' });
          }}
        />
        <FieldError id='pages-github-asset-error'>
          {errors.assetName}
        </FieldError>
      </Field>
    </>
  );
}
