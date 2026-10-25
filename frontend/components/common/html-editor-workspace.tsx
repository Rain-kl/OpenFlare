'use client';

import { useTranslations } from 'next-intl';
import { useRef, useState } from 'react';
import { useTheme } from 'next-themes';
import CodeMirror from '@uiw/react-codemirror';
import { html } from '@codemirror/lang-html';
import { Expand, Terminal } from 'lucide-react';
import Link from 'next/link';

import { Button } from '@/components/ui/button';
import {
  ORIGIN_ERROR_PAGE_HTML_MAX_BYTES,
  previewOriginErrorPageHTML,
} from '@/lib/openflare/default-origin-error-page-html';

type HtmlEditorWorkspaceProps = {
  value: string;
  onChange: (value: string) => void;
  toolbarRight?: React.ReactNode;
  maxBytes?: number;
  preview?: (html: string) => string;
  footerHint?: React.ReactNode;
  showPreviewLink?: boolean;
  previewTitle?: string;
};

/**
 * 通用 HTML 编辑器工作区：左代码 / 右实时预览横向布局，可拖拽分隔。
 */
export function HtmlEditorWorkspace({
  value,
  onChange,
  toolbarRight,
  maxBytes = ORIGIN_ERROR_PAGE_HTML_MAX_BYTES,
  preview = previewOriginErrorPageHTML,
  footerHint = (
    <>
      {'{{status}}'}→502 · {'{{host}}'}→example.com
    </>
  ),
  showPreviewLink = true,
  previewTitle = undefined,
}: HtmlEditorWorkspaceProps) {
  const rt = useTranslations('runtime');
  previewTitle ??= rt('live_origin_error_page_preview_a2a12823');

  const containerRef = useRef<HTMLDivElement>(null);
  const [editorWidthPercent, setEditorWidthPercent] = useState(48);
  const { resolvedTheme } = useTheme();
  const cmTheme = resolvedTheme === 'dark' ? 'dark' : 'light';
  const previewSrcDoc = preview(value);
  const htmlBytes = new TextEncoder().encode(value).length;

  const handleMouseDown = (e: React.MouseEvent) => {
    e.preventDefault();
    const container = containerRef.current;
    if (!container) return;
    const rect = container.getBoundingClientRect();

    const handleMouseMove = (moveEvent: MouseEvent) => {
      const x = moveEvent.clientX - rect.left;
      const pct = (x / rect.width) * 100;
      if (pct > 25 && pct < 75) {
        setEditorWidthPercent(pct);
      }
    };

    const handleMouseUp = () => {
      document.removeEventListener('mousemove', handleMouseMove);
      document.removeEventListener('mouseup', handleMouseUp);
    };

    document.addEventListener('mousemove', handleMouseMove);
    document.addEventListener('mouseup', handleMouseUp);
  };

  return (
    <div
      ref={containerRef}
      className='w-full border border-border/40 bg-card/60 backdrop-blur-md rounded-lg overflow-hidden flex flex-col shadow-sm h-[calc(100vh-160px)] min-h-[520px]'
    >
      <div className='flex items-center justify-between px-4 py-2 border-b bg-muted/40 shrink-0 gap-4 flex-wrap'>
        <div className='flex items-center gap-2 min-w-0'>
          <Terminal className='size-4 text-primary shrink-0' />
          <span className='text-xs font-semibold'>
            {rt('html_editor_19f0d7b7')}
          </span>
          <span className='text-[11px] text-muted-foreground font-mono truncate'>
            {htmlBytes} / {maxBytes} {rt('bytes_f5d87a8b')}
            {value.trim() === ''
              ? rt('leave_empty_for_the_built_in_default_29ffce9f')
              : ''}
          </span>
        </div>
        <div className='flex items-center gap-1.5 flex-wrap'>
          {toolbarRight}
        </div>
      </div>

      <div className='flex flex-1 min-h-0 overflow-hidden'>
        <div
          style={{ width: `${editorWidthPercent}%` }}
          className='h-full overflow-hidden relative min-w-[200px] bg-background border-r border-border/40'
        >
          <CodeMirror
            value={value}
            height='100%'
            extensions={[html()]}
            theme={cmTheme}
            onChange={onChange}
            className='h-full text-xs font-mono'
            basicSetup={{
              lineNumbers: true,
              foldGutter: true,
              dropCursor: true,
              allowMultipleSelections: false,
              indentOnInput: true,
            }}
            placeholder={rt('leave_empty_to_use_the_built_in_e57252bd')}
          />
        </div>

        <div
          onMouseDown={handleMouseDown}
          className='w-1.5 bg-border/60 hover:bg-primary/50 cursor-col-resize transition-colors flex items-center justify-center shrink-0 select-none z-10'
          title={rt('drag_to_resize_b5154511')}
        >
          <div className='h-8 w-1 rounded bg-muted-foreground/30' />
        </div>

        <div className='flex-1 min-w-[200px] flex flex-col overflow-hidden bg-muted/10'>
          <div className='flex items-center justify-between px-4 py-1.5 border-b bg-muted/20 shrink-0 text-[11px] text-muted-foreground font-mono gap-2'>
            <span className='font-semibold'>{rt('live_preview_a76fdaad')}</span>
            <div className='flex items-center gap-2'>
              <span
                className={footerHint === null ? 'hidden' : 'hidden sm:inline'}
              >
                {footerHint ?? null}
              </span>
              {showPreviewLink ? (
                <Button
                  variant='ghost'
                  size='sm'
                  className='h-6 px-2 text-[11px]'
                  asChild
                >
                  <Link href='/error-pages/preview'>
                    <Expand className='size-3' />
                    {rt('preview_13d61fea')}
                  </Link>
                </Button>
              ) : null}
            </div>
          </div>
          <div className='flex-1 min-h-0 overflow-hidden bg-background'>
            <iframe
              title={previewTitle}
              sandbox=''
              srcDoc={previewSrcDoc}
              className='h-full w-full border-0 bg-background'
            />
          </div>
        </div>
      </div>
    </div>
  );
}
