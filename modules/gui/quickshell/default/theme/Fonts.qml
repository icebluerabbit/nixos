pragma Singleton
import QtQuick
import QtCore
import Quickshell

QtObject {
    id: root

    property var settings: Settings {
        category: "Fonts"
    }

    readonly property string monospace: settings.value("monospace", "JetBrainsMono Nerd Font")
    readonly property string sansSerif: settings.value("sansSerif", "Inter")
    readonly property string serif:     settings.value("serif", "Noto Serif")
    readonly property string emoji:     settings.value("emoji", "Noto Color Emoji")
}
