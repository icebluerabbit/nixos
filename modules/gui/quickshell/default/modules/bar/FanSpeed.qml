import QtQuick
import Quickshell.Io
import "../../theme/"

Item {
    implicitWidth: row.implicitWidth
    implicitHeight: row.implicitHeight

    property int fan: 1
    property string hwmonPath: ""
    property string rpm: "--"

    Process {
        id: discover
        command: ["sh", "-c", "for d in /sys/class/hwmon/hwmon*; do [ \"$(cat \"$d/name\" 2>/dev/null)\" = 'nzxtsmart2' ] && echo \"$d\" && break; done"]
        running: true
        stdout: SplitParser {
            onRead: data => { hwmonPath = data.trim() }
        }
    }

    Process {
        id: proc
        command: ["sh", "-c", `cat ${hwmonPath}/fan${fan}_input`]
        running: hwmonPath !== ""
        stdout: SplitParser {
            onRead: data => {
                var v = data.trim()
                if (v !== "") rpm = v
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
            text: "󰈐"
            color: Colors.base0B
            font.family: Fonts.monospace
            font.pixelSize: Sizes.iconSize
            anchors.verticalCenter: parent.verticalCenter
        }

        Text {
            text: rpm
            color: rpm === "--" ? Colors.base03 : Colors.base05
            font.family: Fonts.monospace
            font.pixelSize: Sizes.textSize
            anchors.verticalCenter: parent.verticalCenter
        }
    }
}
