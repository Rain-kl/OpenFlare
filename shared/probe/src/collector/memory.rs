use crate::types::{DiskMetrics, MemoryMetrics, SwapMetrics};
use std::collections::HashSet;
use std::path::Path;
use sysinfo::{Disk, Disks, System};

const ALLOWED_FILESYSTEMS: &[&str] = &[
    "apfs",
    "ext4",
    "ext3",
    "ext2",
    "f2fs",
    "reiserfs",
    "jfs",
    "btrfs",
    "fuseblk",
    "zfs",
    "simfs",
    "ntfs",
    "fat32",
    "exfat",
    "xfs",
    "fuse.rclone",
    "ubifs",
];

const EXCLUDED_MOUNT_KEYWORDS: &[&str] = &[
    "/snap",
    "/var/lib/docker",
    "/var/lib/lxcfs",
    "/run/user",
    "/tmp",
    "/dev",
    "/sys",
    "/proc",
    "/boot",
    "/lost+found",
    "/nix/store",
    "/var/log.hdd",
];

pub(super) fn collect_memory(system: &System) -> MemoryMetrics {
    let total = system.total_memory();
    let available = system.available_memory();
    let used = if available > 0 {
        total.saturating_sub(available)
    } else {
        system.used_memory().min(total)
    };
    MemoryMetrics { used, total }
}

pub(super) fn collect_swap(system: &System) -> SwapMetrics {
    let total = system.total_swap();
    let used = system.used_swap().min(total);
    SwapMetrics { used, total }
}

pub(super) fn collect_disk(disks: &Disks, fallback_path: Option<&Path>) -> DiskMetrics {
    let filtered = filter_disks(disks);
    let mut used = 0u64;
    let mut total = 0u64;

    if !filtered.is_empty() {
        for disk in filtered {
            let disk_total = disk.total_space();
            let disk_avail = disk.available_space();
            used = used.saturating_add(disk_total.saturating_sub(disk_avail));
            total = total.saturating_add(disk_total);
        }
    } else if let Some(path) = fallback_path {
        if let Some(disk) = disks
            .iter()
            .filter(|disk| path.starts_with(disk.mount_point()))
            .max_by_key(|disk| disk.mount_point().as_os_str().len())
        {
            total = disk.total_space();
            used = total.saturating_sub(disk.available_space());
        }
    }

    let (read_bytes, write_bytes) = disks.iter().fold((0u64, 0u64), |(read, write), disk| {
        let usage = disk.usage();
        (
            read.saturating_add(usage.total_read_bytes),
            write.saturating_add(usage.total_written_bytes),
        )
    });

    DiskMetrics {
        used,
        total,
        read_bytes,
        write_bytes,
    }
}

fn filter_disks(disks: &Disks) -> Vec<&Disk> {
    let mut unique = Vec::new();
    let mut seen_devices = HashSet::new();

    for disk in disks.iter() {
        let fs = disk.file_system().to_string_lossy().to_ascii_lowercase();
        if !ALLOWED_FILESYSTEMS.contains(&fs.as_str()) {
            continue;
        }
        let mount = disk.mount_point().to_string_lossy();
        if EXCLUDED_MOUNT_KEYWORDS
            .iter()
            .any(|keyword| mount.contains(keyword))
        {
            continue;
        }
        let dev = disk.name().to_string_lossy().into_owned();
        if seen_devices.insert(dev) {
            unique.push(disk);
        }
    }

    unique
}
