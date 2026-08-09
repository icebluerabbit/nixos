pragma Singleton
import QtQuick
import QtCore
import Quickshell

QtObject {
    id: root

    property var settings: Settings {
        category: "Colors"
    }

    readonly property color base00: settings.value("base00", "#1e1e2e")
    readonly property color base01: settings.value("base01", "#181825")
    readonly property color base02: settings.value("base02", "#313244")
    readonly property color base03: settings.value("base03", "#45475a")
    readonly property color base04: settings.value("base04", "#585b70")
    readonly property color base05: settings.value("base05", "#cdd6f4")
    readonly property color base06: settings.value("base06", "#f5e0dc")
    readonly property color base07: settings.value("base07", "#b4befe")
    readonly property color base08: settings.value("base08", "#f38ba8")
    readonly property color base09: settings.value("base09", "#fab387")
    readonly property color base0A: settings.value("base0A", "#f9e2af")
    readonly property color base0B: settings.value("base0B", "#a6e3a1")
    readonly property color base0C: settings.value("base0C", "#94e2d5")
    readonly property color base0D: settings.value("base0D", "#89b4fa")
    readonly property color base0E: settings.value("base0E", "#cba6f7")
    readonly property color base0F: settings.value("base0F", "#f2cdcd")
}
