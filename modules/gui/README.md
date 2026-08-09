# gui

Graphical user interface infrastructure — the Wayland session and its core components.

Each aspect covers one piece of the desktop stack: the compositor, the status bar, the notification daemon, the wallpaper daemon, and the Wayland session bootstrap (portals, env vars, display manager backend). These are the structural layer that application aspects sit on top of. The bar widget code lives under `quickshell/` as QML files alongside its aspect.
