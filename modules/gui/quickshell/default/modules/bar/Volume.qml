import QtQuick
import Quickshell.Services.Pipewire
import "../../theme/"

Item {
    id: root
    implicitWidth: row.implicitWidth
    implicitHeight: row.implicitHeight

    property PwNode sink: Pipewire.defaultAudioSink
    property real vol: sink?.audio?.volume ?? 0
    property bool muted: sink?.audio?.muted ?? false

    PwObjectTracker { objects: [root.sink] }

    signal clicked

    HoverBackground { anchors.fill: parent }
    MouseArea {
        anchors.fill: parent
        cursorShape: Qt.PointingHandCursor
        onClicked: parent.clicked()
        onWheel: event => {
            var delta = event.angleDelta.y > 0 ? 0.05 : -0.05
            var newVol = Math.max(0, Math.min(1, root.vol + delta))
            root.sink.audio.volume = newVol
        }
    }

    Row {
        id: row
        spacing: Sizes.spacingXs
        anchors.verticalCenter: parent.verticalCenter

        Text {
            text: muted ? "󰝟" : vol > 0.66 ? "󰕾" : vol > 0.33 ? "󰖀" : "󰕿"
            color: muted ? Colors.base08 : Colors.base0C
            font.family: Fonts.monospace
            font.pixelSize: Sizes.iconSize
            anchors.verticalCenter: parent.verticalCenter
        }

        Text {
            text: muted ? "muted" : Math.round(vol * 100)
            color: Colors.base05
            font.family: Fonts.monospace
            font.pixelSize: Sizes.textSize
            anchors.verticalCenter: parent.verticalCenter
        }

    }
}
