use serde::Serialize;
use std::sync::Mutex;
use std::time::Duration;
use tauri::{Manager, State};
use tauri_plugin_shell::process::CommandChild;
use tauri_plugin_shell::ShellExt;
use tokio::time::sleep;

const ZERO_API_BASE: &str = "http://127.0.0.1:8910";
const DEFAULT_PROVIDER_BASE: &str = "http://127.0.0.1:20128/v1";

#[derive(Default)]
struct ServerProcess {
    child: Mutex<Option<CommandChild>>,
}

#[derive(Serialize)]
struct StatusResponse {
    ok: bool,
    status: String,
    detail: String,
}

#[tauri::command]
async fn server_status() -> StatusResponse {
    health_status(&format!("{ZERO_API_BASE}/health"), "server").await
}

#[tauri::command]
async fn provider_status() -> StatusResponse {
    let base =
        std::env::var("ZERO_ROUTER_BASE_URL").unwrap_or_else(|_| DEFAULT_PROVIDER_BASE.to_string());
    let url = format!("{}/models", base.trim_end_matches('/'));
    health_status(&url, "provider").await
}

#[tauri::command]
async fn start_server(
    app: tauri::AppHandle,
    state: State<'_, ServerProcess>,
) -> Result<StatusResponse, String> {
    let current = server_status().await;
    if current.ok {
        return Ok(current);
    }

    let already_starting = {
        let child = state
            .child
            .lock()
            .map_err(|_| "server process lock poisoned".to_string())?;
        child.is_some()
    };
    if already_starting {
        return Ok(wait_for_server().await);
    }

    let sidecar = app
        .shell()
        .sidecar("zero-server")
        .map_err(|err| format!("failed to create zero-server sidecar: {err}"))?;
    let (mut rx, child) = sidecar
        .spawn()
        .map_err(|err| format!("failed to start zero-server sidecar: {err}"))?;

    tauri::async_runtime::spawn(async move { while rx.recv().await.is_some() {} });

    {
        let mut slot = state
            .child
            .lock()
            .map_err(|_| "server process lock poisoned".to_string())?;
        *slot = Some(child);
    }

    Ok(wait_for_server().await)
}

#[tauri::command]
async fn stop_server(state: State<'_, ServerProcess>) -> Result<StatusResponse, String> {
    let child = {
        let mut slot = state
            .child
            .lock()
            .map_err(|_| "server process lock poisoned".to_string())?;
        slot.take()
    };

    if let Some(child) = child {
        child
            .kill()
            .map_err(|err| format!("failed to stop zero-server sidecar: {err}"))?;
        Ok(StatusResponse {
            ok: false,
            status: "stopped".to_string(),
            detail: "Stopped the desktop-managed zero-server sidecar.".to_string(),
        })
    } else {
        Ok(StatusResponse {
            ok: false,
            status: "external-or-stopped".to_string(),
            detail: "No desktop-managed zero-server sidecar is running. An external server may still be active.".to_string(),
        })
    }
}

async fn wait_for_server() -> StatusResponse {
    for _ in 0..30 {
        let status = server_status().await;
        if status.ok {
            return status;
        }
        sleep(Duration::from_millis(250)).await;
    }

    StatusResponse {
        ok: false,
        status: "starting".to_string(),
        detail: "zero-server was launched, but /health did not become ready within 7.5s."
            .to_string(),
    }
}

async fn health_status(url: &str, label: &str) -> StatusResponse {
    let client = match reqwest::Client::builder()
        .timeout(Duration::from_secs(2))
        .build()
    {
        Ok(client) => client,
        Err(err) => {
            return StatusResponse {
                ok: false,
                status: "client-error".to_string(),
                detail: format!("failed to build HTTP client for {label}: {err}"),
            };
        }
    };

    match client.get(url).send().await {
        Ok(response) => {
            let status = response.status();
            let ok = status.is_success();
            StatusResponse {
                ok,
                status: status.as_u16().to_string(),
                detail: if ok {
                    format!("{label} responded at {url}")
                } else {
                    format!("{label} returned HTTP {status} at {url}")
                },
            }
        }
        Err(err) => StatusResponse {
            ok: false,
            status: "offline".to_string(),
            detail: format!("{label} is unreachable at {url}: {err}"),
        },
    }
}

fn main() {
    tauri::Builder::default()
        .plugin(tauri_plugin_dialog::init())
        .plugin(tauri_plugin_shell::init())
        .manage(ServerProcess::default())
        .invoke_handler(tauri::generate_handler![
            server_status,
            provider_status,
            start_server,
            stop_server
        ])
        .setup(|app| {
            let handle = app.handle().clone();
            tauri::async_runtime::spawn(async move {
                let _ = start_server(handle.clone(), handle.state::<ServerProcess>()).await;
            });
            Ok(())
        })
        .run(tauri::generate_context!())
        .expect("error while running Zero desktop");
}
