use std::fs;
use std::path::Path;

#[test]
fn manifest_and_lockfile_keep_the_same_package_version() {
    let manifest_dir = Path::new(env!("CARGO_MANIFEST_DIR"));
    let manifest = fs::read_to_string(manifest_dir.join("Cargo.toml")).unwrap();
    let lockfile = fs::read_to_string(manifest_dir.join("Cargo.lock")).unwrap();

    let manifest_version = manifest
        .lines()
        .find_map(|line| line.strip_prefix("version = "))
        .expect("Cargo.toml has a package version");
    let lockfile_version = lockfile
        .lines()
        .skip_while(|line| !line.contains("name = \"openflare-edge\""))
        .find_map(|line| line.strip_prefix("version = "))
        .expect("Cargo.lock has the openflare-edge version");

    assert_eq!(manifest_version, lockfile_version);
}
