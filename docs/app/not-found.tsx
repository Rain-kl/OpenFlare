import Link from 'next/link';

export default function NotFound() {
  return (
    <main className="mx-auto flex min-h-screen max-w-3xl flex-col items-center justify-center px-6 text-center">
      <p className="mb-3 font-mono text-sm tracking-[0.2em] text-fd-primary">404</p>
      <h1 className="text-3xl font-semibold tracking-tight">页面未找到</h1>
      <p className="mt-3 text-fd-muted-foreground">
        该页面可能已移动，或链接地址不正确。
      </p>
      <Link
        className="mt-7 rounded-full bg-fd-primary px-5 py-2.5 text-sm font-medium text-fd-primary-foreground"
        href="/"
      >
        返回文档首页
      </Link>
    </main>
  );
}
