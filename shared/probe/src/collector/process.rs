/// Counts currently running OS processes.
/// On Linux, scans numeric entries in `/proc` directly (avoiding full sysinfo process refresh overhead).
pub(super) fn collect_process_count() -> u64 {
    #[cfg(target_os = "linux")]
    {
        let Ok(entries) = std::fs::read_dir("/proc") else {
            return 0;
        };
        let mut count = 0u64;
        for entry in entries.flatten() {
            if entry
                .file_name()
                .to_str()
                .is_some_and(|name| !name.is_empty() && name.bytes().all(|b| b.is_ascii_digit()))
            {
                count = count.saturating_add(1);
            }
        }
        count
    }

    #[cfg(not(target_os = "linux"))]
    {
        0
    }
}
