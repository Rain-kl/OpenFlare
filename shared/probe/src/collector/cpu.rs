use crate::types::CpuMetrics;
use std::collections::HashSet;
use sysinfo::System;

pub(super) struct CpuBasicInfo {
    pub arch: String,
    pub logical_cores: u64,
    pub physical_cores: u64,
    pub name: String,
}

pub(super) fn collect_cpu_info(system: &System) -> CpuBasicInfo {
    let cpus = system.cpus();
    let logical_cores = cpus.len() as u64;
    let physical_cores = System::physical_core_count()
        .map(|count| count as u64)
        .unwrap_or(logical_cores);

    let mut seen = HashSet::new();
    let mut brands = Vec::new();
    for cpu in cpus {
        let brand = cpu.brand().trim();
        if !brand.is_empty() && seen.insert(brand.to_owned()) {
            brands.push(brand.to_owned());
        }
    }

    CpuBasicInfo {
        arch: std::env::consts::ARCH.to_owned(),
        logical_cores,
        physical_cores,
        name: brands.join(", "),
    }
}

pub(super) fn collect_cpu_usage(system: &System) -> CpuMetrics {
    let cpus = system.cpus();
    if cpus.is_empty() {
        let fallback = f64::from(system.global_cpu_usage());
        return CpuMetrics {
            usage: if fallback.is_finite() {
                fallback.clamp(0.0, 100.0)
            } else {
                0.0
            },
        };
    }

    let sum: f64 = cpus.iter().map(|cpu| f64::from(cpu.cpu_usage())).sum();
    let avg = sum / cpus.len() as f64;
    CpuMetrics {
        usage: if avg.is_finite() {
            avg.clamp(0.0, 100.0)
        } else {
            0.0
        },
    }
}
