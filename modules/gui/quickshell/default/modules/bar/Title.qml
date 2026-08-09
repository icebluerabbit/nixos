import QtQuick
import Quickshell
import "../../theme/"

Item {
    property int maxWidth: 400

    implicitWidth: Math.min(Math.max(label.implicitWidth, 200), maxWidth)
    implicitHeight: label.implicitHeight

    Text {
        id: label
        anchors.fill: parent
        horizontalAlignment: Text.AlignHCenter
        verticalAlignment: Text.AlignVCenter
        text: niri.focusedWindow?.title ?? ""
        font.family: Fonts.monospace
        font.pixelSize: Sizes.textSize
        color: Colors.base05
        elide: Text.ElideRight
    }
}
