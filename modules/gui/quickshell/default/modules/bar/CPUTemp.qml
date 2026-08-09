import QtQuick
import Quickshell.Io
import "../../theme/"

Item {
    implicitWidth: row.implicitWidth
    implicitHeight: row.implicitHeight

    property string hwmonPath: ""
    property string temp: "--"

    Process {
        id: discover
        command: ["sh", "-c", "for d in /sys/class/hwmon/hwmon*; do [ \"$(cat \"$d/name\" 2>/dev/null)\" = 'k10temp' ] && echo \"$d\" && break; done"]
        running: true
        stdout: SplitParser {
            onRead: data => { hwmonPath = data.trim() }
        }
    }

    Process {
        id: proc
        command: ["sh", "-c", `awk '{printf "%.0f", $1/1000}' ${hwmonPath}/temp1_input`]
        running: hwmonPath !== ""
        stdout: SplitParser {
            onRead: data => {
                var t = data.trim()
                if (t !== "") temp = t
            }
        }
    }

    Timer {
        interval: 3000
        repeat: true
        running: hwmonPath !== ""
        onTriggered: proc.running = true
    }

    Row {
        id: row
        spacing: Sizes.spacingXs
        anchors.verticalCenter: parent.verticalCenter

        Text {
            text: ""
            color: Colors.base08
            font.family: Fonts.monospace
            font.pixelSize: Sizes.iconSize
            anchors.verticalCenter: parent.verticalCenter
        }

        Text {
            text: temp
            color: temp === "--" ? Colors.base03 : Colors.base05
            font.family: Fonts.monospace
            font.pixelSize: Sizes.textSize
            anchors.verticalCenter: parent.verticalCenter
        }

    }
}
