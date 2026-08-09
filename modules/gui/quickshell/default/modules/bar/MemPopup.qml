import QtQuick
import QtQuick.Layouts
import QtQuick.Controls.Basic
import Quickshell.Services.Pipewire
import "../../theme/"

BarPopup {
    id: root

    implicitWidth: 64

    Text {
        text: "Mem Popup"
        color: Colors.base05
        font.family: Fonts.monospace
        font.pixelSize: Sizes.textSize
        Layout.alignment: Qt.AlignHCenter
    }
}
