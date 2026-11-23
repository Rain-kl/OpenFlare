pub mod bootstrap;
pub mod cidr;
pub mod hardening;
pub mod identity;
pub mod node_ip;
pub mod protocol;
pub mod updater;
pub mod ws;

pub const TUNNEL_VERSION: &str = match option_env!("OPENFLARE_TUNNEL_VERSION") {
    Some(value) if !value.is_empty() => value,
    _ => "dev",
};

pub const CONTRACT_VERSION: u32 = 1;

pub fn user_agent(binary: &str) -> String {
    format!("{binary}/{TUNNEL_VERSION}")
}
