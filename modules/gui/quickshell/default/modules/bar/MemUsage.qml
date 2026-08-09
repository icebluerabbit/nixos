import QtQuick
import Quickshell.Io
import "../../theme/"

Item {
    id: root
    implicitWidth: row.implicitWidth
    implicitHeight: row.implicitHeight

    property string used: "--"

    signal clicked

    Process {
        id: proc
        command: ["sh", "-c", "awk '/MemTotal/ {t=$2} /MemAvailable/ {a=$2} END {printf \"%.1f\", (t-a)/1024/1024}' /proc/meminfo"]
        running: true
        stdout: SplitParser {
            onRead: data => {
                var t = data.trim()
                if (t !== "") used = t
            }
        }
    }

    Timer {
        interval: 5000
        repeat: true
        running: true
        onTriggered: proc.running = true
    }

    HoverBackground { anchors.fill: parent }
    MouseArea {
        anchors.fill: parent
        cursorShape: Qt.PointingHandCursor
        onClicked: root.clicked()
    }

    Row {
        id: row
        spacing: Sizes.spacingXs
        anchors.verticalCenter: parent.verticalCenter

        Text {
            text: ""
            color: Colors.base0A
            font.family: Fonts.monospace
            font.pixelSize: Sizes.iconSize
            anchors.verticalCenter: parent.verticalCenter
        }

        Text {
            text: used
            color: used === "--" ? Colors.base03 : Colors.base05
            font.family: Fonts.monospace
            font.pixelSize: Sizes.textSize
            anchors.verticalCenter: parent.verticalCenter
        }
    }
}
