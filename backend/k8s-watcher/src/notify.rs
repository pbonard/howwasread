//! Sends alerts to Discord and/or Telegram, or only logs them when neither is configured.

use std::time::Duration;

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

    fn icon(self) -> &'static str {
        match self {
            Severity::Critical => "🔴",
            Severity::Warning => "🟡",
            Severity::Resolved => "🟢",
            Severity::Info => "🔵",
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
}

/// A cheap handle to the sender task, cloning it clones only the channel sender.
#[derive(Clone)]
pub struct Notifier {
    tx: mpsc::UnboundedSender<Alert>,
}

impl Notifier {
    pub fn spawn() -> Self {
        let (tx, rx) = mpsc::unbounded_channel();
        tokio::spawn(send_alerts(rx));
        Self { tx }
    }

    /// Never waits, the alert is sent by the background task.
    pub fn notify(&self, title: impl Into<String>, body: impl Into<String>, severity: Severity) {
        let _ = self.tx.send(Alert {
            title: title.into(),
            body: body.into(),
            severity,
        });
    }
}

// one sender so a burst of alerts stays under the rate limits of both services
async fn send_alerts(mut rx: mpsc::UnboundedReceiver<Alert>) {
    let discord = std::env::var("DISCORD_WEBHOOK_URL")
        .ok()
        .filter(|v| !v.is_empty());
    let telegram = std::env::var("TELEGRAM_BOT_TOKEN")
        .ok()
        .filter(|v| !v.is_empty())
        .zip(
            std::env::var("TELEGRAM_CHAT_ID")
                .ok()
                .filter(|v| !v.is_empty()),
        );
    let http = reqwest::Client::builder()
        .timeout(Duration::from_secs(10))
        .build()
        .expect("http client");

    while let Some(alert) = rx.recv().await {
        let icon = alert.severity.icon();
        if discord.is_none() && telegram.is_none() {
            info!(
                "alert (no destination configured): {icon} {}\n{}",
                alert.title, alert.body
            );
        }
        if let Some(url) = &discord {
            post(
                &http,
                url,
                json!({
                    "embeds": [{
                        "title": truncate(&format!("{icon} {}", alert.title), 256),
                        "description": truncate(&alert.body, 4000),
                        "color": alert.severity.color(),
                    }],
                }),
            )
            .await;
        }
        if let Some((token, chat)) = &telegram {
            post(
                &http,
                &format!("https://api.telegram.org/bot{token}/sendMessage"),
                json!({
                    "chat_id": chat,
                    "text": truncate(&format!("{icon} {}\n\n{}", alert.title, alert.body), 4000),
                }),
            )
            .await;
        }
    }
}

async fn post(http: &reqwest::Client, url: &str, payload: Value) {
    for _ in 0..3 {
        match http.post(url).json(&payload).send().await {
            Ok(resp) if resp.status() == StatusCode::TOO_MANY_REQUESTS => {
                // both services return how long to wait, Telegram nests it under "parameters"
                let info: Value = resp.json().await.unwrap_or_default();
                let wait = info["retry_after"]
                    .as_f64()
                    .or(info["parameters"]["retry_after"].as_f64())
                    .unwrap_or(2.0);
                tokio::time::sleep(Duration::from_secs_f64(wait)).await;
            }
            Ok(resp) if !resp.status().is_success() => {
                let status = resp.status();
                let text = resp.text().await.unwrap_or_default();
                error!("fail to send alert: {status} {}", truncate(&text, 200));
                return;
            }
            Ok(_) => return,
            Err(e) => {
                error!("fail to send alert: {e}");
                return;
            }
        }
    }
}

/// Cuts by characters, not bytes, so a multi-byte character like an emoji is never split.
pub fn truncate(text: &str, max_chars: usize) -> String {
    text.chars().take(max_chars).collect()
}
