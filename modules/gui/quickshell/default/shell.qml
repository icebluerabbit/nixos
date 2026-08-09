import QtQuick
import QtQuick.Layouts
import Quickshell
import Quickshell.Wayland
import Niri 0.1
import "./modules/bar/"

ShellRoot {
    Component.onCompleted: {
        Qt.application.name = "MyShell"
        Qt.application.organization = "ZMS"
    }

    Niri {
        id: niri
        Component.onCompleted: connect()
        onConnected: console.info("Connected to niri")
        onErrorOccurred: function(error) {
            console.error("Niri error:", error)
        }
    }

    Variants {
        model: Quickshell.screens
        Bar {}
    }
}
