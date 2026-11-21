'use client';

import { CodeBlock, Pre } from 'fumadocs-ui/components/codeblock';
import { useEffect, useId, useState } from 'react';

type MermaidResult = {
  svg: string;
};

export function Mermaid({ chart }: { chart: string }) {
  const id = useId().replaceAll(':', '');
  const [theme, setTheme] = useState<'default' | 'dark'>('default');
  const [result, setResult] = useState<MermaidResult | null>(null);
  const [failed, setFailed] = useState(false);

  useEffect(() => {
    const updateTheme = () =>
      setTheme(document.documentElement.classList.contains('dark') ? 'dark' : 'default');

    updateTheme();
    const observer = new MutationObserver(updateTheme);
    observer.observe(document.documentElement, { attributeFilter: ['class'] });
    return () => observer.disconnect();
  }, []);

  useEffect(() => {
    let cancelled = false;

    void import('mermaid')
      .then(async ({ default: mermaid }) => {
        mermaid.initialize({
          startOnLoad: false,
          securityLevel: 'strict',
          fontFamily: 'inherit',
          theme,
        });
        return mermaid.render(`mermaid-${id}`, chart.replaceAll('\\n', '\n'));
      })
      .then(({ svg }) => {
        if (!cancelled) {
          setResult({ svg });
          setFailed(false);
        }
      })
      .catch(() => {
        if (!cancelled) setFailed(true);
      });

    return () => {
      cancelled = true;
    };
  }, [chart, id, theme]);

  if (failed) {
    return (
      <CodeBlock title="Mermaid">
        <Pre>{chart}</Pre>
      </CodeBlock>
    );
  }

  if (!result) {
    return <div aria-label="Diagram loading" className="my-6 min-h-12" role="status" />;
  }

  return (
    <div
      aria-label="Mermaid diagram"
      className="my-6 overflow-x-auto [&_svg]:mx-auto [&_svg]:max-w-full"
      dangerouslySetInnerHTML={{ __html: result.svg }}
      role="img"
    />
  );
}
