'use client';

// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

import * as React from 'react';
import { useTranslations } from 'next-intl';
import { Code } from 'lucide-react';
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card';
import { Label } from '@/components/ui/label';
import { Input } from '@/components/ui/input';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import { Checkbox } from '@/components/ui/checkbox';
import { Button } from '@/components/ui/button';

export function ControlsTab() {
  const rt = useTranslations('runtime');

  const t = useTranslations('admin.demo.controls');
  const [formData, setFormData] = React.useState({
    name: '',
    category: 'database',
    allowNotify: true,
    channel: 'all',
  });

  return (
    <div className='grid grid-cols-1 lg:grid-cols-2 gap-6'>
      {/* 控件展示 */}
      <Card className='border-dashed shadow-none'>
        <CardHeader className='pb-3'>
          <CardTitle className='text-sm font-semibold'>
            {t('formDemoTitle')}
          </CardTitle>
          <CardDescription>{t('formDemoDesc')}</CardDescription>
        </CardHeader>
        <CardContent className='space-y-4'>
          {/* 输入框 */}
          <div className='space-y-1.5'>
            <Label
              htmlFor='task-name'
              className='text-xs font-medium text-foreground'
            >
              {t('taskName')}
            </Label>
            <Input
              id='task-name'
              value={formData.name}
              onChange={(e) =>
                setFormData((prev) => ({ ...prev, name: e.target.value }))
              }
              placeholder={t('taskNamePlaceholder')}
              className='h-8 text-xs shadow-none bg-background focus-visible:ring-1'
            />
            <p className='text-[10px] text-muted-foreground'>
              {t('taskNameHint')}
            </p>
          </div>

          {/* 下拉框 */}
          <div className='space-y-1.5'>
            <Label className='text-xs font-medium text-foreground'>
              {t('taskCategory')}
            </Label>
            <Select
              value={formData.category}
              onValueChange={(val) =>
                setFormData((prev) => ({ ...prev, category: val }))
              }
            >
              <SelectTrigger className='h-8 text-xs shadow-none bg-background focus:ring-1'>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value='database'>
                  {t('categoryDatabase')}
                </SelectItem>
                <SelectItem value='sync'>{t('categorySync')}</SelectItem>
                <SelectItem value='cleanup'>{t('categoryCleanup')}</SelectItem>
              </SelectContent>
            </Select>
          </div>

          {/* 多选框 Checkbox */}
          <div className='flex items-start gap-2.5 p-3 bg-muted/30 border border-dashed rounded-lg'>
            <Checkbox
              id='allow-notify'
              checked={formData.allowNotify}
              onCheckedChange={(checked) =>
                setFormData((prev) => ({
                  ...prev,
                  allowNotify: checked === true,
                }))
              }
              className='mt-0.5'
            />
            <div className='grid gap-1'>
              <label
                htmlFor='allow-notify'
                className='text-xs font-medium text-foreground cursor-pointer select-none leading-none'
              >
                {t('allowNotify')}
              </label>
              <p className='text-[10px] text-muted-foreground leading-normal'>
                {t('allowNotifyDesc')}
              </p>
            </div>
          </div>

          {/* 按钮变体 */}
          <div className='pt-2'>
            <div className='text-[10px] font-semibold text-muted-foreground uppercase tracking-wider mb-2'>
              {t('buttonStyles')}
            </div>
            <div className='flex flex-wrap items-center gap-2'>
              <Button size='sm' className='h-8 text-xs shadow-none'>
                {t('primaryButton')}
              </Button>
              <Button
                variant='secondary'
                size='sm'
                className='h-8 text-xs shadow-none'
              >
                {t('secondaryButton')}
              </Button>
              <Button
                variant='outline'
                size='sm'
                className='h-8 border-dashed text-xs shadow-none'
              >
                {t('outlineButton')}
              </Button>
              <Button
                variant='ghost'
                size='sm'
                className='h-8 text-xs hover:bg-muted'
              >
                {t('ghostButton')}
              </Button>
            </div>
          </div>
        </CardContent>
      </Card>

      {/* 控件设计规范说明 */}
      <Card className='border-dashed shadow-none bg-muted/20'>
        <CardHeader className='pb-3'>
          <CardTitle className='text-sm font-semibold flex items-center gap-1.5'>
            <Code className='size-4 text-primary' />
            {t('formDesignSpec')}
          </CardTitle>
        </CardHeader>
        <CardContent className='space-y-4 text-xs text-muted-foreground'>
          <div className='space-y-2 leading-relaxed'>
            <p className='font-semibold text-foreground text-[11px]'>
              {t('spec.unifiedHeight')}
            </p>
            <p>
              {rt('inputs_fabee312')}
              <code className='bg-muted px-1 py-0.5 rounded font-mono'>
                Input
              </code>
              {rt('and_selectors_e4875cdf')}
              <code className='bg-muted px-1 py-0.5 rounded font-mono'>
                SelectTrigger
              </code>
              {rt('should_share_the_compact_height_f1fe2218')}{' '}
              <code className='bg-muted px-1 py-0.5 rounded font-mono'>
                h-8
              </code>
              {rt('with_font_size_e7c29991')}{' '}
              <code className='bg-muted px-1 py-0.5 rounded font-mono'>
                text-xs
              </code>
              {rt('and_use_ca398a12')}{' '}
              <code className='bg-muted px-1 py-0.5 rounded font-mono'>
                shadow-none
              </code>{' '}
              {rt('to_remove_overlapping_native_shadows_dd65b475')}
            </p>

            <p className='font-semibold text-foreground text-[11px] pt-1'>
              {t('spec.checkboxAlignment')}
            </p>
            <p>
              {rt('align_checkboxes_toward_the_top_left_using_092dcef0')}{' '}
              <code className='bg-muted px-1 py-0.5 rounded font-mono'>
                flex items-start gap-2.5
              </code>{' '}
              {rt('add_bf15da3a')}{' '}
              <code className='bg-muted px-1 py-0.5 rounded font-mono'>
                mt-0.5
              </code>
              {rt('to_the_checkbox_so_multiline_descriptions_stay_1bb85c4b')}
            </p>

            <p className='font-semibold text-foreground text-[11px] pt-1'>
              {t('spec.buttonStrategy')}
            </p>
            <p>
              {rt('use_the_default_primary_or_secondary_db7513e5')}{' '}
              <code className='bg-muted px-1 py-0.5 rounded font-mono'>
                {'variant="secondary"'}
              </code>{' '}
              {rt('button_for_forms_and_create_save_actions_a657dba8')}{' '}
              <code className='bg-muted px-1 py-0.5 rounded font-mono'>
                h-8 text-xs
              </code>
              {rt('outline_buttons_must_include_the_66ace005')}{' '}
              <code className='bg-muted px-1 py-0.5 rounded font-mono'>
                border-dashed shadow-none
              </code>{' '}
              {rt('class_43eff5ff')}
            </p>
          </div>

          <div className='bg-background/80 p-3 rounded border border-border/40 font-mono text-[10px] space-y-2 overflow-x-auto'>
            <div>{rt('1_align_label_and_input_32d4558d')}</div>
            <div>{rt.raw('div_classname_space_y_1_5_label_32230ad7')}</div>
            <div className='pt-2'>
              {rt('2_align_checkbox_to_the_top_bdf68ad8')}
            </div>
            <div>{rt.raw('div_classname_flex_items_start_gap_2_ccf0d526')}</div>
          </div>
        </CardContent>
      </Card>
    </div>
  );
}
