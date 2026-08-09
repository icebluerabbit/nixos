import QtQuick
import QtQuick.Layouts
import QtQuick.Controls.Basic
import Quickshell.Services.Pipewire
import "../../theme/"

BarPopup {
    id: root

    implicitWidth: 64

    property PwNode sink: Pipewire.defaultAudioSink
    property real vol: sink?.audio?.volume ?? 0
    property bool muted: sink?.audio?.muted ?? false

    Text {
        text: root.muted ? "muted" : Math.round(volSlider.value * 100) + "%"
        color: root.muted ? Colors.base08 : Colors.base05
        font.family: Fonts.monospace
        font.pixelSize: Sizes.textSize
        Layout.alignment: Qt.AlignHCenter
    }

    Slider {
        id: volSlider
        orientation: Qt.Vertical
        from: 0
        to: 1
        value: root.vol
        Layout.alignment: Qt.AlignHCenter
        Layout.preferredHeight: 120

        onMoved: root.sink.audio.volume = value

        background: Rectangle {
            x: volSlider.leftPadding + (volSlider.availableWidth - width) / 2
            y: volSlider.topPadding
            width: 4
            height: volSlider.availableHeight
            radius: Shape.radiusFull
            color: Colors.base03

            Rectangle {
                width: parent.width
                height: (1 - volSlider.visualPosition) * parent.height
                anchors.bottom: parent.bottom
                radius: Shape.radiusFull
                color: root.muted ? Colors.base03 : Colors.base0C
            }
        }

        handle: Rectangle {
            x: volSlider.leftPadding + (volSlider.availableWidth - width) / 2
            y: volSlider.topPadding + volSlider.visualPosition * volSlider.availableHeight - height / 2
            width: 14
            height: 14
            radius: Shape.radiusFull
            color: root.muted ? Colors.base04 : Colors.base0C
        }
    }

    Text {
        text: root.muted ? "󰝟" : root.vol > 0.66 ? "󰕾" : root.vol > 0.33 ? "󰖀" : "󰕿"
        color: root.muted ? Colors.base08 : Colors.base0C
        font.family: Fonts.monospace
        font.pixelSize: Sizes.iconSize
        Layout.alignment: Qt.AlignHCenter

        MouseArea {
            anchors.fill: parent
            cursorShape: Qt.PointingHandCursor
            onClicked: root.sink.audio.muted = !root.sink.audio.muted
        }
    }
}
