pragma Singleton
import QtQuick
import QtCore

QtObject {
    id: root

    property var settings: Settings {
        category: "Sizes"
    }

    readonly property int height: settings.value("height", 30)

    readonly property int marginXs: settings.value("marginXs", 5)
    readonly property int marginSm: settings.value("marginSm", 10)
    readonly property int marginMd: settings.value("marginMd", 15)
    readonly property int marginLg: settings.value("marginLg", 25)

    readonly property int spacingXs: settings.value("spacingXs", 5)
    readonly property int spacingSm: settings.value("spacingSm", 10)
    readonly property int spacingMd: settings.value("spacingMd", 15)
    readonly property int spacingLg: settings.value("spacingLg", 25)

    readonly property int iconSize: settings.value("iconSize", 16)
    readonly property int textSize: settings.value("textSize", 13)
}
