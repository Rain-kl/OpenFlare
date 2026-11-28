use crate::protocol::{UpdateCommand, UpdateReceipt};
use std::io::ErrorKind;
use std::path::{Path, PathBuf};
use tokio::fs;

pub struct RemoteUpdate {
    request: PathBuf,
    status: PathBuf,
}

impl RemoteUpdate {
    pub fn from_service(state_dir: &Path) -> Option<Self> {
        let status = PathBuf::from(std::env::var_os("OPENFLARE_DNS_UPDATE_STATUS")?);
        if !status.is_absolute() {
            return None;
        }
        Some(Self {
            request: state_dir.join("remote-update.request"),
            status,
        })
    }

    pub async fn receipt(&self) -> Option<UpdateReceipt> {
        let data = fs::read_to_string(&self.status).await.ok()?;
        let (id, status) = data.trim().split_once(' ')?;
        if !valid_id(id) || !matches!(status, "running" | "succeeded" | "failed") {
            return None;
        }
        Some(UpdateReceipt {
            id: id.into(),
            status: status.into(),
        })
    }

    pub async fn queue(&self, command: UpdateCommand) -> anyhow::Result<()> {
        anyhow::ensure!(
            valid_id(&command.id) && matches!(command.channel.as_str(), "stable" | "canary"),
            "invalid remote update command"
        );
        if self
            .receipt()
            .await
            .is_some_and(|receipt| receipt.id == command.id)
            || fs::try_exists(&self.request).await?
        {
            return Ok(());
        }

        let staged = self.request.with_extension("tmp");
        fs::write(&staged, format!("{} {}\n", command.id, command.channel)).await?;
        if let Err(err) = fs::rename(&staged, &self.request).await {
            match fs::remove_file(staged).await {
                Ok(()) => (),
                Err(cleanup) if cleanup.kind() == ErrorKind::NotFound => (),
                Err(cleanup) => tracing::warn!("could not remove staged remote update: {cleanup}"),
            }
            return Err(err.into());
        }
        Ok(())
    }
}

fn valid_id(id: &str) -> bool {
    id.len() == 26
        && id
            .bytes()
            .all(|byte| byte.is_ascii_uppercase() || (b'2'..=b'7').contains(&byte))
}
