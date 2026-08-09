import QtQuick
import Quickshell.Io
import "../../theme/"

Item {
    implicitWidth: row.implicitWidth
    implicitHeight: row.implicitHeight

    property string temp: "--"

    Process {
        id: proc
        command: ["nvidia-smi", "--query-gpu=temperature.gpu", "--format=csv,noheader,nounits"]
        running: true
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
        running: true
        onTriggered: proc.running = true
    }

    Row {
        id: row
        spacing: Sizes.spacingXs
        anchors.verticalCenter: parent.verticalCenter

        Text {
            text: "󰾲"
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
