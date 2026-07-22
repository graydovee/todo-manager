// Tauri desktop client — library entry point.

mod tray;

use tauri::WebviewWindow;

#[cfg_attr(mobile, tauri::mobile_entry_point)]
pub fn run() {
    tauri::Builder::default()
        .plugin(tauri_plugin_store::Builder::default().build())
        .plugin(tauri_plugin_opener::init())
        .plugin(tauri_plugin_http::init())
        .setup(|app| {
            tray::setup(app)?;
            Ok(())
        })
        .invoke_handler(tauri::generate_handler![
            set_always_on_top,
            set_lock,
            tray::sync_tray_state,
        ])
        .run(tauri::generate_context!())
        .expect("error while running tauri application");
}

/// Toggle always-on-top for the main window.
#[tauri::command]
fn set_always_on_top(window: WebviewWindow, top: bool) -> Result<(), String> {
    window.set_always_on_top(top).map_err(|e| e.to_string())
}

/// Toggle click-through lock. When locked, the window becomes click-through
/// (mouse events pass through). The semi-transparent visual effect is handled
/// entirely by the frontend CSS (all elements get semi-transparent backgrounds
/// when #root has the .app-locked class).
#[tauri::command]
fn set_lock(window: WebviewWindow, locked: bool) -> Result<(), String> {
    if locked {
        window.set_always_on_top(true).map_err(|e| e.to_string())?;
        window
            .set_ignore_cursor_events(true)
            .map_err(|e| e.to_string())?;
    } else {
        window
            .set_ignore_cursor_events(false)
            .map_err(|e| e.to_string())?;
    }
    Ok(())
}
