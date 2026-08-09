pragma Singleton
import QtQuick
import QtCore

QtObject {
    id: root

    property var settings: Settings {
        category: "Shape"
    }

    readonly property int radiusSm: settings.value("radiusSm", 6)
    readonly property int radiusMd: settings.value("radiusMd", 10)
    readonly property int radiusLg: settings.value("radiusLg", 12)
    readonly property int radiusFull: settings.value("radiusFull", 999)
}
