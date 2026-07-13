export type ImageQuality = 'low' | 'medium' | 'high' | 'origin';

export function getFileUrl(
  id: string | number | null | undefined,
  quality: ImageQuality = 'origin',
): string | null {
  if (!id) return null;
  if (quality === 'origin') return `/f/${id}`;
  return `/f/${id}?quality=${quality}`;
}

export function formatFileSize(bytes: number): string {
  if (bytes === 0) return '0 B';
  const k = 1024;
  const sizes = ['B', 'KB', 'MB', 'GB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return `${parseFloat((bytes / Math.pow(k, i)).toFixed(1))} ${sizes[i]}`;
}
