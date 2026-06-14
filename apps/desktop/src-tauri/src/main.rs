use serde::Serialize;
use std::path::PathBuf;
use std::sync::Mutex;
use std::time::Duration;
use tauri::{Emitter, Manager, State};
use tauri_plugin_deep_link::DeepLinkExt;
use tauri_plugin_shell::process::CommandChild;
use tauri_plugin_shell::ShellExt;
use tokio::time::sleep;

const ZERO_API_BASE: &str = "http://127.0.0.1:8910";
const DEFAULT_PROVIDER_BASE: &str = "https://api.openai.com/v1";

fn load_dotenv() {
    let candidates: Vec<PathBuf> = [
        std::env::current_dir().ok().map(|p| p.join(".env")),
        std::env::var_os("HOME").map(|h| PathBuf::from(h).join(".config").join("zero").join(".env")),
        std::env::var_os("HOME").map(|h| PathBuf::from(h).join(".zero").join(".env")),
    ]
    .into_iter()
    .flatten()
    .collect();

    for path in candidates {
        if let Ok(contents) = std::fs::read_to_string(&path) {
            for line in contents.lines() {
                let trimmed = line.trim();
                if trimmed.is_empty() || trimmed.starts_with('#') {
                    continue;
                }
                if let Some(idx) = trimmed.find('=') {
                    let key = trimmed[..idx].trim();
                    let val = trimmed[idx + 1..].trim().trim_matches('"').trim_matches('\'');
                    if !key.is_empty() && std::env::var(key).is_err() {
                        std::env::set_var(key, val);
                    }
                }
            }
        }
    }
}

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

fn pending_project_path() -> Option<PathBuf> {
    let home = std::env::var_os("HOME")?;
    Some(PathBuf::from(home).join(".zero").join("pending-project.txt"))
}

#[tauri::command]
async fn consume_pending_project() -> Option<String> {
    let path = pending_project_path()?;
    let bytes = std::fs::read(&path).ok()?;
    let _ = std::fs::remove_file(&path);
    let raw = String::from_utf8(bytes).ok()?;
    let trimmed = raw.trim().to_string();
    if trimmed.is_empty() {
        None
    } else {
        Some(trimmed)
    }
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

#[tauri::command]
async fn upload_attachments(
    session_id: String,
    paths: Vec<String>,
    auth_token: Option<String>,
) -> Result<serde_json::Value, String> {
    if session_id.trim().is_empty() {
        return Err("session id required".to_string());
    }
    if paths.is_empty() {
        return Err("no files to upload".to_string());
    }

    let mut form = reqwest::multipart::Form::new();
    for path_str in &paths {
        let path = std::path::Path::new(path_str);
        let bytes = tokio::fs::read(path)
            .await
            .map_err(|err| format!("read {}: {}", path_str, err))?;
        let filename = path
            .file_name()
            .and_then(|s| s.to_str())
            .unwrap_or("file")
            .to_string();
        let mime = mime_for(path);
        let part = reqwest::multipart::Part::bytes(bytes)
            .file_name(filename)
            .mime_str(&mime)
            .map_err(|err| format!("mime {}: {}", mime, err))?;
        form = form.part("files", part);
    }

    let client = reqwest::Client::builder()
        .timeout(Duration::from_secs(120))
        .build()
        .map_err(|err| format!("http client: {err}"))?;

    let mut req = client
        .post(format!("{ZERO_API_BASE}/sessions/{session_id}/files"))
        .multipart(form);
    if let Some(token) = auth_token {
        if !token.is_empty() {
            req = req.bearer_auth(token);
        }
    }

    let resp = req.send().await.map_err(|err| format!("upload: {err}"))?;
    let status = resp.status();
    let body = resp.text().await.unwrap_or_default();
    if !status.is_success() {
        return Err(format!("upload failed (HTTP {status}): {body}"));
    }
    let parsed: serde_json::Value = serde_json::from_str(&body)
        .map_err(|err| format!("decode response: {err}"))?;
    Ok(parsed)
}

fn mime_for(path: &std::path::Path) -> String {
    let ext = path
        .extension()
        .and_then(|e| e.to_str())
        .unwrap_or("")
        .to_lowercase();
    match ext.as_str() {
        "png" => "image/png",
        "jpg" | "jpeg" => "image/jpeg",
        "gif" => "image/gif",
        "webp" => "image/webp",
        "bmp" => "image/bmp",
        "svg" => "image/svg+xml",
        "heic" => "image/heic",
        "tiff" | "tif" => "image/tiff",
        "pdf" => "application/pdf",
        "doc" => "application/msword",
        "docx" => "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
        "xls" => "application/vnd.ms-excel",
        "xlsx" => "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
        "csv" => "text/csv",
        "txt" | "log" | "md" | "mdx" => "text/plain",
        "json" => "application/json",
        "yaml" | "yml" => "application/yaml",
        "html" | "htm" => "text/html",
        "mp4" => "video/mp4",
        "mov" => "video/quicktime",
        "webm" => "video/webm",
        "mp3" => "audio/mpeg",
        "wav" => "audio/wav",
        _ => "application/octet-stream",
    }
    .to_string()
}

fn main() {
    load_dotenv();
    let mut builder = tauri::Builder::default();

    #[cfg(desktop)]
    {
        builder = builder.plugin(tauri_plugin_single_instance::init(|app, argv, _cwd| {
            forward_deep_link_argv(app, &argv);
            if let Some(window) = app.get_webview_window("main") {
                let _ = window.show();
                let _ = window.set_focus();
            }
        }));
    }

    builder
        .plugin(tauri_plugin_deep_link::init())
        .plugin(tauri_plugin_dialog::init())
        .plugin(tauri_plugin_shell::init())
        .manage(ServerProcess::default())
        .invoke_handler(tauri::generate_handler![
            server_status,
            provider_status,
            start_server,
            stop_server,
            consume_pending_project,
            upload_attachments
        ])
        .setup(|app| {
            let handle = app.handle().clone();
            tauri::async_runtime::spawn(async move {
                let _ = start_server(handle.clone(), handle.state::<ServerProcess>()).await;
            });

            #[cfg(any(target_os = "linux", all(debug_assertions, windows)))]
            {
                let _ = app.deep_link().register_all();
            }

            if let Ok(Some(urls)) = app.deep_link().get_current() {
                let strs: Vec<String> = urls.iter().map(|u| u.to_string()).collect();
                emit_deep_links(app.handle(), &strs);
            }

            let emit_handle = app.handle().clone();
            app.deep_link().on_open_url(move |event| {
                let strs: Vec<String> = event.urls().iter().map(|u| u.to_string()).collect();
                emit_deep_links(&emit_handle, &strs);
            });

            Ok(())
        })
        .run(tauri::generate_context!())
        .expect("error while running Zero desktop");
}

fn emit_deep_links(handle: &tauri::AppHandle, urls: &[String]) {
    for url in urls {
        if !url.to_lowercase().starts_with("zero://") {
            continue;
        }
        let _ = handle.emit("zero://deep-link", url.clone());
    }
}

fn forward_deep_link_argv(app: &tauri::AppHandle, argv: &[String]) {
    for arg in argv.iter().skip(1) {
        if arg.to_lowercase().starts_with("zero://") {
            let _ = app.emit("zero://deep-link", arg.clone());
        }
    }
}
