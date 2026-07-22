// System tray setup — creates a tray icon with Lock / Pin / Quit menu items.
// Lock and Pin use CheckMenuItem so they show a checkmark when active.

use tauri::{
    menu::{CheckMenuItem, Menu, MenuEvent, MenuItem, PredefinedMenuItem},
    tray::{MouseButton, MouseButtonState, TrayIconBuilder, TrayIconEvent},
    App, AppHandle, Emitter, Manager, Wry,
};

/// Holds the checkable tray menu items so we can update their checked state.
pub struct TrayMenuState {
    pub lock_item: CheckMenuItem<Wry>,
    pub pin_item: CheckMenuItem<Wry>,
}

/// Set up the system tray with a menu.
pub fn setup(app: &App) -> Result<(), Box<dyn std::error::Error>> {
    let lock_item = CheckMenuItem::with_id(app, "lock", "Lock", true, false, None::<&str>)?;
    let pin_item = CheckMenuItem::with_id(app, "pin", "Pin on top", true, false, None::<&str>)?;
    let sep = PredefinedMenuItem::separator(app)?;
    let quit_item = MenuItem::with_id(app, "quit", "Quit", true, None::<&str>)?;

    let menu = Menu::with_items(app, &[&lock_item, &pin_item, &sep, &quit_item])?;

    let _tray = TrayIconBuilder::new()
        .icon(app.default_window_icon().unwrap().clone())
        .tooltip("Todo Desktop")
        .menu(&menu)
        .show_menu_on_left_click(false)
        .on_menu_event(on_menu_event)
        .on_tray_icon_event(on_tray_icon_event)
        .build(app)?;

    app.manage(TrayMenuState { lock_item, pin_item });

    Ok(())
}

fn on_menu_event(app: &AppHandle, event: MenuEvent) {
    match event.id().as_ref() {
        "lock" => {
            if let Some(window) = app.get_webview_window("main") {
                let _ = window.emit("tray-toggle-lock", ());
            }
        }
        "pin" => {
            if let Some(window) = app.get_webview_window("main") {
                let _ = window.emit("tray-toggle-pin", ());
            }
        }
        "quit" => {
            app.exit(0);
        }
        _ => {}
    }
}

fn on_tray_icon_event(tray: &tauri::tray::TrayIcon, event: TrayIconEvent) {
    if let TrayIconEvent::Click {
        button: MouseButton::Left,
        button_state: MouseButtonState::Up,
        ..
    } = event
    {
        let app = tray.app_handle();
        if let Some(window) = app.get_webview_window("main") {
            if window.is_visible().unwrap_or(false) {
                let _ = window.hide();
            } else {
                let _ = window.show();
                let _ = window.set_focus();
            }
        }
    }
}

/// Sync the tray check items' checked state from the frontend.
#[tauri::command]
pub fn sync_tray_state(
    app: AppHandle,
    locked: bool,
    topmost: bool,
) -> Result<(), String> {
    if let Some(state) = app.try_state::<TrayMenuState>() {
        state
            .lock_item
            .set_checked(locked)
            .map_err(|e| e.to_string())?;
        state
            .pin_item
            .set_checked(topmost)
            .map_err(|e| e.to_string())?;
    }
    Ok(())
}
