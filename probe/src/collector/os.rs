use sysinfo::System;

pub(super) struct OsBasicInfo {
    pub hostname: String,
    pub os_name: String,
    pub os_version: String,
    pub display_name: String,
    pub kernel_version: String,
}

pub(super) fn collect_os_info() -> OsBasicInfo {
    let hostname = System::host_name().unwrap_or_default();
    let os_name = System::name().unwrap_or_else(|| "Unknown".to_owned());
    let os_version = System::os_version().unwrap_or_default();
    let display_name = if os_version.is_empty() {
        os_name.clone()
    } else {
        format!("{os_name} {os_version}")
    };
    let kernel_version = System::kernel_version().unwrap_or_else(|| "Unknown".to_owned());

    OsBasicInfo {
        hostname,
        os_name,
        os_version,
        display_name,
        kernel_version,
    }
}

/// Pure-Rust virtualization/container detector (Linux sysfs/procfs without external alpha crates).
pub(super) fn detect_virtualization() -> String {
    #[cfg(target_os = "linux")]
    {
        detect_linux_virtualization()
    }
    #[cfg(not(target_os = "linux"))]
    {
        "Unknown".to_owned()
    }
}

#[cfg(target_os = "linux")]
fn detect_linux_virtualization() -> String {
    if std::path::Path::new("/.dockerenv").exists() {
        return "docker".to_owned();
    }
    if std::path::Path::new("/run/.containerenv").exists() {
        return "podman".to_owned();
    }
    if let Ok(container) = std::fs::read_to_string("/run/systemd/container") {
        let trimmed = container.trim();
        if !trimmed.is_empty() {
            return trimmed.to_owned();
        }
    }
    if let Ok(cgroup) = std::fs::read_to_string("/proc/1/cgroup") {
        let lower = cgroup.to_ascii_lowercase();
        if lower.contains("docker") {
            return "docker".to_owned();
        }
        if lower.contains("lxc") {
            return "lxc".to_owned();
        }
        if lower.contains("kubepods") || lower.contains("containerd") {
            return "container".to_owned();
        }
    }
    if let Ok(version) = std::fs::read_to_string("/proc/version") {
        if version.to_ascii_lowercase().contains("microsoft") {
            return "wsl".to_owned();
        }
    }

    let dmi_paths = [
        "/sys/class/dmi/id/product_name",
        "/sys/class/dmi/id/sys_vendor",
        "/sys/class/dmi/id/bios_vendor",
        "/sys/class/dmi/id/board_vendor",
        "/sys/hypervisor/type",
    ];
    for path in dmi_paths {
        let Ok(content) = std::fs::read_to_string(path) else {
            continue;
        };
        if let Some(virt) = match_virt_keyword(&content) {
            return virt.to_owned();
        }
    }

    if let Ok(cpuinfo) = std::fs::read_to_string("/proc/cpuinfo") {
        if let Some(virt) = match_virt_keyword(&cpuinfo) {
            return virt.to_owned();
        }
        if cpuinfo.contains("hypervisor") {
            return "vm".to_owned();
        }
    }

    "none".to_owned()
}

#[cfg(target_os = "linux")]
fn match_virt_keyword(raw: &str) -> Option<&'static str> {
    let lower = raw.to_ascii_lowercase();
    if lower.contains("kvm") || lower.contains("qemu") {
        Some("kvm")
    } else if lower.contains("vmware") {
        Some("vmware")
    } else if lower.contains("virtualbox") || lower.contains("innotek") {
        Some("virtualbox")
    } else if lower.contains("xen") {
        Some("xen")
    } else if lower.contains("microsoft corporation") || lower.contains("hyper-v") {
        Some("hyperv")
    } else if lower.contains("parallels") {
        Some("parallels")
    } else if lower.contains("bhyve") {
        Some("bhyve")
    } else if lower.contains("bochs") {
        Some("bochs")
    } else if lower.contains("openvz") {
        Some("openvz")
    } else if lower.contains("amazon ec2") {
        Some("kvm")
    } else if lower.contains("google compute engine") {
        Some("kvm")
    } else {
        None
    }
}
