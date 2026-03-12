'use client';

import {marked} from 'marked';
import {useEffect, useState} from 'react';

import {EmptyState} from '@/components/feedback/empty-state';
import {ErrorState} from '@/components/feedback/error-state';
import {LoadingState} from '@/components/feedback/loading-state';
import {AppCard} from '@/components/ui/app-card';
import {AppModal} from '@/components/ui/app-modal';
import {StatusBadge} from '@/components/ui/status-badge';
import type {LatestReleaseInfo, ReleaseChannel, UploadedServerBinaryInfo,} from '@/features/update/types';
import {
    PrimaryButton,
    ResourceField,
    ResourceInput,
    SecondaryButton,
} from '@/features/shared/components/resource-primitives';
import {formatDateTime, formatRelativeTime} from '@/lib/utils/date';

interface VersionUpgradeModalProps {
    isOpen: boolean;
    onClose: () => void;
    currentVersion: string;
    frontendVersion: string;
    release: LatestReleaseInfo | null | undefined;
    selectedChannel: ReleaseChannel;
    uploadedBinary: UploadedServerBinaryInfo | null;
    isLoading: boolean;
    releaseErrorMessage?: string;
    manualStatusMessage?: string;
    manualErrorMessage?: string;
    canUpgrade: boolean;
    isChecking: boolean;
    isUpgrading: boolean;
    isUploadingBinary: boolean;
    uploadProgress: number;
    isConfirmingManualUpgrade: boolean;
    onChannelChange: (channel: ReleaseChannel) => void;
    onCheck: () => void;
    onUpgrade: () => void;
    onUploadBinary: (file: File) => void;
    onConfirmManualUpgrade: () => void;
}

function getUpgradeBadge(release: LatestReleaseInfo | null | undefined) {
    if (!release) {
        return {label: '未检查', variant: 'info' as const};
    }
    if (release.in_progress) {
        return {label: '升级中', variant: 'warning' as const};
    }
    if (release.has_update) {
        return {label: '可升级', variant: 'warning' as const};
    }
    return {label: '最新', variant: 'success' as const};
}

export function VersionUpgradeModal({
                                        isOpen,
                                        onClose,
                                        currentVersion,
                                        frontendVersion,
                                        release,
                                        selectedChannel,
                                        uploadedBinary,
                                        isLoading,
                                        releaseErrorMessage,
                                        manualStatusMessage,
                                        manualErrorMessage,
                                        canUpgrade,
                                        isChecking,
                                        isUpgrading,
                                        isUploadingBinary,
                                        uploadProgress,
                                        isConfirmingManualUpgrade,
                                        onChannelChange,
                                        onCheck,
                                        onUpgrade,
                                        onUploadBinary,
                                        onConfirmManualUpgrade,
                                    }: VersionUpgradeModalProps) {
    const upgradeBadge = getUpgradeBadge(release);
    const [selectedBinary, setSelectedBinary] = useState<File | null>(null);
    const selectedChannelLabel =
        selectedChannel === 'preview' ? '预览版' : '正式版';
    const toggleChannel = () => {
        onChannelChange(selectedChannel === 'preview' ? 'stable' : 'preview');
    };
    const canConfirmManualUpgrade = Boolean(
        uploadedBinary?.ready_to_upgrade && uploadedBinary.upload_token,
    );
    const showConfirmManualUpgradeAction = canConfirmManualUpgrade;

    useEffect(() => {
        if (!isOpen) {
            setSelectedBinary(null);
        }
    }, [isOpen]);

    return (
        <AppModal
            isOpen={isOpen}
            onClose={onClose}
            title="版本"
            description="默认检查正式版更新；你也可以手动检查 preview 发布并选择升级，或上传 Server 二进制确认升级。升级开始后服务会短暂重启。"
            size="lg"
        >
            <div className="space-y-6">
                <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-2">

                    <AppCard title="服务端版本">
                        <div className="flex flex-wrap items-center gap-3">
                            <p className="text-sm font-medium text-[var(--foreground-primary)]">
                                {currentVersion || 'unknown'}
                            </p>
                            <StatusBadge
                                label={upgradeBadge.label}
                                variant={upgradeBadge.variant}
                            />
                        </div>
                    </AppCard>

                    <AppCard title="最新版本">
                        <div className="space-y-4">
                            <div className="flex flex-wrap items-center gap-3">
                                <p className="text-sm font-medium text-[var(--foreground-primary)]">
                                    {release?.tag_name || '未检查'}
                                </p>
                            {canUpgrade ? (
                                <div className="flex flex-wrap items-center justify-between gap-3">
                                    <div className="flex flex-wrap items-center gap-2">
                                        <StatusBadge
                                            label={selectedChannelLabel}
                                            variant={
                                                selectedChannel === 'preview' ? 'warning' : 'info'
                                            }
                                            onClick={toggleChannel}
                                            disabled={isChecking || isUpgrading || isUploadingBinary}
                                        />
                                    </div>
                                </div>
                            ) : null}
                            </div>
                            <SecondaryButton
                                type="button"
                                onClick={onCheck}
                                disabled={isChecking || isUpgrading || isUploadingBinary}
                                className="w-full md:w-auto"
                            >
                                {isChecking ? '检查中...' : '检查'}
                            </SecondaryButton>
                        </div>
                    </AppCard>
                </div>

                {isLoading ? <LoadingState/> : null}
                {!isLoading && releaseErrorMessage ? (
                    <ErrorState title="版本检查失败" description={releaseErrorMessage}/>
                ) : null}
                {!isLoading && !releaseErrorMessage && !release ? (
                    <EmptyState
                        title={`尚未检查${selectedChannelLabel}`}
                        description={`点击“检查${selectedChannelLabel}”后会在这里展示对应 GitHub Release 信息。`}
                    />
                ) : null}
                {!isLoading && !releaseErrorMessage && release ? (
                    <AppCard
                        title={`GitHub ${selectedChannelLabel} · ${release.tag_name}`}
                        description={
                            release.published_at
                                ? `发布时间：${formatRelativeTime(release.published_at)} · ${formatDateTime(release.published_at)}`
                                : '未提供发布时间'
                        }
                    >
                        <div className="space-y-4">
                            <div className="flex flex-wrap items-center gap-3">
                                <StatusBadge
                                    label={release.has_update ? '发现新版本' : '已经是最新版本'}
                                    variant={release.has_update ? 'warning' : 'success'}
                                />
                                {release.prerelease ? (
                                    <StatusBadge label="Preview 发布" variant="warning"/>
                                ) : (
                                    <StatusBadge label="正式发布" variant="info"/>
                                )}
                                {!release.upgrade_supported ? (
                                    <StatusBadge
                                        label="当前平台不支持自动升级"
                                        variant="danger"
                                    />
                                ) : null}
                                {release.in_progress ? (
                                    <StatusBadge label="升级任务执行中" variant="warning"/>
                                ) : null}
                            </div>
                            <div
                                className="prose prose-sm max-w-none text-[var(--foreground-primary)] [&_a]:text-[var(--brand-primary)]"
                                dangerouslySetInnerHTML={{
                                    __html: marked.parse(
                                        release.body || '暂无更新说明',
                                    ) as string,
                                }}
                            />
                            <a
                                href={release.html_url}
                                target="_blank"
                                rel="noreferrer"
                                className="text-sm font-medium text-[var(--brand-primary)] transition hover:opacity-80"
                            >
                                查看发布详情
                            </a>
                            {canUpgrade ? (
                                <div className="flex justify-end">

                                    <PrimaryButton
                                        type="button"
                                        onClick={onUpgrade}
                                        disabled={
                                            !release.has_update ||
                                            release.in_progress ||
                                            isUpgrading ||
                                            !release.upgrade_supported ||
                                            isUploadingBinary ||
                                            isConfirmingManualUpgrade
                                        }
                                    >
                                        {isUpgrading
                                            ? '升级中...'
                                            : release.in_progress
                                                ? '升级中...'
                                                : selectedChannel === 'preview'
                                                    ? '升级到预览版'
                                                    : '升级到正式版'}
                                    </PrimaryButton>
                                </div>
                            ) : null}
                        </div>
                    </AppCard>
                ) : null}

                {canUpgrade ? (
                    <AppCard
                        title="手动升级"
                        description="上传服务端二进制后，服务端会先检查版本，再由你确认是否执行升级。"
                    >
                        <div className="space-y-4">
                            <ResourceField
                                label="服务端二进制"
                                hint="支持上传已编译好的当前平台 Server 可执行文件。"
                            >
                                <ResourceInput
                                    type="file"
                                    onChange={(event) => {
                                        const file = event.target.files?.[0] ?? null;
                                        setSelectedBinary(file);
                                    }}
                                    disabled={isUploadingBinary || isConfirmingManualUpgrade}
                                />
                            </ResourceField>

                            <div className="flex justify-end">
                                {showConfirmManualUpgradeAction ? (
                                    <PrimaryButton
                                        type="button"
                                        onClick={onConfirmManualUpgrade}
                                        disabled={
                                            !canConfirmManualUpgrade ||
                                            isConfirmingManualUpgrade ||
                                            isUploadingBinary ||
                                            isUpgrading
                                        }
                                    >
                                        {isConfirmingManualUpgrade ? '升级中...' : '升级'}
                                    </PrimaryButton>
                                ) : (
                                    <PrimaryButton
                                        type="button"
                                        onClick={() => {
                                            if (selectedBinary) {
                                                onUploadBinary(selectedBinary);
                                            }
                                        }}
                                        disabled={
                                            !selectedBinary ||
                                            isUploadingBinary ||
                                            isConfirmingManualUpgrade
                                        }
                                    >
                                        {isUploadingBinary ? '上传检查中...' : '上传并检查'}
                                    </PrimaryButton>
                                )}
                            </div>

                            {isUploadingBinary ? (
                                <div className="space-y-1">
                                    <div className="flex items-center justify-between text-xs text-[var(--foreground-secondary)]">
                                        <span>上传中...</span>
                                        <span>{uploadProgress}%</span>
                                    </div>
                                    <div className="h-2 w-full overflow-hidden rounded-full bg-[var(--border-default)]">
                                        <div
                                            className="h-full rounded-full bg-[var(--brand-primary)] transition-all duration-200"
                                            style={{width: `${uploadProgress}%`}}
                                        />
                                    </div>
                                </div>
                            ) : null}

                            {manualErrorMessage ? (
                                <ErrorState
                                    title="手动升级检查失败"
                                    description={manualErrorMessage}
                                />
                            ) : null}

                            {!manualErrorMessage && manualStatusMessage ? (
                                <div
                                    className="rounded-2xl border border-[var(--status-warning-border)] bg-[var(--status-warning-soft)] px-4 py-3 text-sm text-[var(--status-warning-foreground)]">
                                    {manualStatusMessage}
                                </div>
                            ) : null}

                            {uploadedBinary ? (
                                <div
                                    className="space-y-4 rounded-3xl border border-[var(--border-default)] bg-[var(--surface-elevated)] p-4">
                                    <div className="flex flex-wrap items-center gap-3">
                                        <StatusBadge
                                            label={
                                                uploadedBinary.ready_to_upgrade
                                                    ? '可确认升级'
                                                    : uploadedBinary.has_update
                                                        ? '待确认'
                                                        : '不可升级'
                                            }
                                            variant={
                                                uploadedBinary.ready_to_upgrade ? 'warning' : 'info'
                                            }
                                        />
                                        {!uploadedBinary.upgrade_supported ? (
                                            <StatusBadge
                                                label="当前版本不支持手动升级"
                                                variant="danger"
                                            />
                                        ) : null}
                                    </div>

                                    <div className="grid gap-4 md:grid-cols-2">
                                        <div>
                                            <p className="text-xs text-[var(--foreground-secondary)]">
                                                文件名
                                            </p>
                                            <p className="mt-1 text-sm font-medium text-[var(--foreground-primary)]">
                                                {uploadedBinary.file_name}
                                            </p>
                                        </div>
                                        <div>
                                            <p className="text-xs text-[var(--foreground-secondary)]">
                                                上传时间
                                            </p>
                                            <p className="mt-1 text-sm font-medium text-[var(--foreground-primary)]">
                                                {uploadedBinary.uploaded_at
                                                    ? formatDateTime(uploadedBinary.uploaded_at)
                                                    : '未知'}
                                            </p>
                                        </div>
                                        <div>
                                            <p className="text-xs text-[var(--foreground-secondary)]">
                                                当前版本
                                            </p>
                                            <p className="mt-1 text-sm font-medium text-[var(--foreground-primary)]">
                                                {uploadedBinary.current_version}
                                            </p>
                                        </div>
                                        <div>
                                            <p className="text-xs text-[var(--foreground-secondary)]">
                                                上传版本
                                            </p>
                                            <p className="mt-1 text-sm font-medium text-[var(--foreground-primary)]">
                                                {uploadedBinary.detected_version}
                                            </p>
                                        </div>
                                    </div>

                                    <p className="text-sm text-[var(--foreground-secondary)]">
                                        {uploadedBinary.comparison_message}
                                    </p>
                                </div>
                            ) : (
                                <EmptyState
                                    title="尚未上传升级包"
                                    description="上传后会在这里展示识别出的版本信息，并允许你确认升级。"
                                />
                            )}
                        </div>
                    </AppCard>
                ) : null}
            </div>
        </AppModal>
    );
}
