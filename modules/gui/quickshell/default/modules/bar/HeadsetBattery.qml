import QtQuick
import Quickshell.Io
import "../../theme/"

Item {
    implicitWidth: row.implicitWidth
    implicitHeight: row.implicitHeight

    property string level: "--"
    property bool charging: false
    signal clicked

    Process {
        id: proc
        command: ["sh", "-c", "headsetcontrol -b -o JSON 2>/dev/null | jq -c '.devices[0].battery'"]
        running: true
        stdout: SplitParser {
            onRead: data => {
                try {
                    var battery = JSON.parse(data.trim())
                    var status = battery.status
                    if (status === "BATTERY_AVAILABLE" || status === "BATTERY_CHARGING") {
                        level = battery.level
                        charging = status === "BATTERY_CHARGING"
                    } else {
                        level = "--"
                        charging = false
                    }
                } catch (_) {}
            }
        }
    }

    Timer {
        interval: 60000
        repeat: true
        running: true
        onTriggered: proc.running = true
    }

    HoverBackground { anchors.fill: parent }
    MouseArea {
        anchors.fill: parent
        cursorShape: Qt.PointingHandCursor
        onClicked: parent.clicked()
    }

    Row {
        id: row
        spacing: Sizes.spacingXs
        anchors.verticalCenter: parent.verticalCenter

        Text {
            text: "󰋎"
            color: Colors.base0E
            font.family: Fonts.monospace
            font.pixelSize: Sizes.iconSize
            anchors.verticalCenter: parent.verticalCenter
        }

        Text {
            text: level
            color: level === "--" ? Colors.base03 : charging ? Colors.base0B : Colors.base05
            font.family: Fonts.monospace
            font.pixelSize: Sizes.textSize
            anchors.verticalCenter: parent.verticalCenter
        }
    }
}
