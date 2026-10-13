//! Sends alerts to Discord, or only logs them when it isn't configured.

use std::time::Duration;

use crate::monitor::Msg;
use crate::util::truncate;
use reqwest::StatusCode;
use serde_json::{Value, json};
use tokio::sync::mpsc;
use tracing::{error, info};

#[derive(Clone, Copy, Debug, PartialEq)]
pub enum Severity {
    Critical,
    Warning,
    Resolved,
    Info,
}

impl Severity {
    pub fn as_str(self) -> &'static str {
        match self {
            Severity::Critical => "critical",
            Severity::Warning => "warning",
            Severity::Resolved => "resolved",
            Severity::Info => "info",
        }
    }

    fn color(self) -> u32 {
        match self {
            Severity::Critical => 0xE5484D,
            Severity::Warning => 0xF5A524,
            Severity::Resolved => 0x30A46C,
            Severity::Info => 0x3E63DD,
        }
    }
}

struct Alert {
    title: String,
    body: String,
    severity: Severity,
    // what the alert is about, reported back once it is sent
    keys: Vec<String>,
}

/// A cheap handle to the sender task, cloning it clones only the channel sender.
#[derive(Clone)]
pub struct Notifier {
    tx: mpsc::UnboundedSender<Alert>,
}

impl Notifier {
    /// Every alert is reported back to the monitor through `done` once it is sent or failed, only a sent one is
    /// saved as notified.
    pub fn spawn(done: mpsc::Sender<Msg>) -> Self {
        let discord = std::env::var("DISCORD_WEBHOOK_URL")
            .ok()
            .filter(|v| !v.is_empty());
        let http = reqwest::Client::builder()
            .timeout(Duration::from_secs(10))
            .build()
            .expect("http client");
        let (tx, mut rx) = mpsc::unbounded_channel::<Alert>();
        // one sender so a burst of alerts stays under the rate limit of the webhook
        tokio::spawn(async move {
            while let Some(mut alert) = rx.recv().await {
                let keys = std::mem::take(&mut alert.keys);
                let sent = send_alert(&http, discord.as_deref(), alert).await;
                let _ = done.send(Msg::Sent { keys, sent }).await;
            }
        });
        Self { tx }
    }

    pub fn notify(
        &self,
        title: impl Into<String>,
        body: impl Into<String>,
        severity: Severity,
        keys: Vec<String>,
    ) {
        let _ = self.tx.send(Alert {
            title: title.into(),
            body: body.into(),
            severity,
            keys,
        });
    }
}

/// Whether the alert reached its destination, a logged one counts as sent.
async fn send_alert(http: &reqwest::Client, discord: Option<&str>, alert: Alert) -> bool {
    let Some(url) = discord else {
        info!(
            "alert (no destination configured): {} {}\n{}",
            alert.severity.as_str(),
            alert.title,
            alert.body
        );
        return true;
    };
    let payload = json!({
        "embeds": [{
            "title": truncate(&alert.title, 256),
            "description": truncate(&alert.body, 4000),
            "color": alert.severity.color(),
        }],
    });
    for _ in 0..3 {
        match http.post(url).json(&payload).send().await {
            Ok(resp) if resp.status() == StatusCode::TOO_MANY_REQUESTS => {
                let info: Value = resp.json().await.unwrap_or_default();
                let wait = info["retry_after"].as_f64().unwrap_or(2.0);
                tokio::time::sleep(Duration::from_secs_f64(wait)).await;
            }
            Ok(resp) if !resp.status().is_success() => {
                let status = resp.status();
                let text = resp.text().await.unwrap_or_default();
                error!("fail to send alert: {status} {}", truncate(&text, 200));
                return false;
            }
            Ok(_) => return true,
            Err(e) => {
                error!("fail to send alert: {e}");
                return false;
            }
        }
    }
    error!("fail to send alert: still rate limited after 3 tries");
    false
}
