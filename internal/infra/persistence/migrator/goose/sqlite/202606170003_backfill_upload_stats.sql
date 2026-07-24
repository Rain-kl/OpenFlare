-- +goose Up
INSERT INTO w_upload_stats (dimension, stat_key, file_count, file_size)
SELECT 'total', '', COUNT(*), COALESCE(SUM(file_size), 0)
FROM w_uploads
WHERE status != 'deleted'
ON CONFLICT (dimension, stat_key) DO UPDATE SET
    file_count = excluded.file_count,
    file_size = excluded.file_size,
    updated_at = CURRENT_TIMESTAMP;

INSERT INTO w_upload_stats (dimension, stat_key, file_count, file_size)
SELECT
    'type',
    COALESCE(NULLIF(type, ''), 'generic'),
    COUNT(*),
    COALESCE(SUM(file_size), 0)
FROM w_uploads
WHERE status != 'deleted'
GROUP BY COALESCE(NULLIF(type, ''), 'generic')
ON CONFLICT (dimension, stat_key) DO UPDATE SET
    file_count = excluded.file_count,
    file_size = excluded.file_size,
    updated_at = CURRENT_TIMESTAMP;

INSERT INTO w_upload_stats (dimension, stat_key, file_count, file_size)
SELECT
    'category',
    CASE
        WHEN LOWER(mime_type) LIKE 'image/%'
            OR LOWER(extension) IN ('jpg', 'jpeg', 'png', 'webp', 'gif') THEN '图片'
        WHEN LOWER(mime_type) LIKE 'video/%' THEN '视频'
        WHEN LOWER(mime_type) LIKE 'audio/%' THEN '音频'
        WHEN LOWER(extension) IN ('zip', 'rar', '7z', 'tar', 'gz', 'tgz', 'bz2', 'xz')
            OR LOWER(mime_type) LIKE '%zip%'
            OR LOWER(mime_type) LIKE '%tar%'
            OR LOWER(mime_type) LIKE '%gzip%' THEN '压缩包'
        WHEN LOWER(extension) IN ('pdf', 'doc', 'docx', 'xls', 'xlsx', 'ppt', 'pptx', 'txt', 'md', 'csv', 'json', 'yaml', 'yml', 'xml')
            OR LOWER(mime_type) LIKE 'text/%'
            OR LOWER(mime_type) = 'application/pdf' THEN '文档'
        ELSE '其他'
    END,
    COUNT(*),
    COALESCE(SUM(file_size), 0)
FROM w_uploads
WHERE status != 'deleted'
GROUP BY 2
ON CONFLICT (dimension, stat_key) DO UPDATE SET
    file_count = excluded.file_count,
    file_size = excluded.file_size,
    updated_at = CURRENT_TIMESTAMP;

INSERT INTO w_upload_stats (dimension, stat_key, file_count, file_size)
SELECT
    'trend',
    STRFTIME('%Y-%m-%d', created_at),
    COUNT(*),
    COALESCE(SUM(file_size), 0)
FROM w_uploads
WHERE status != 'deleted'
GROUP BY STRFTIME('%Y-%m-%d', created_at)
ON CONFLICT (dimension, stat_key) DO UPDATE SET
    file_count = excluded.file_count,
    file_size = excluded.file_size,
    updated_at = CURRENT_TIMESTAMP;

-- +goose Down
DELETE FROM w_upload_stats;