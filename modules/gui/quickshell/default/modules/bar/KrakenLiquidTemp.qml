import QtQuick
import Quickshell.Io
import "../../theme/"

Item {
    implicitWidth: row.implicitWidth
    implicitHeight: row.implicitHeight

    property string temp: "--"

    Process {
        id: proc
        command: ["sh", "-c", "liquidctl status --json 2>/dev/null"]
        running: true
        stdout: SplitParser {
            onRead: data => {
                try {
                    var devices = JSON.parse(data.trim())
                    var kraken = devices.find(d => d.description && d.description.includes("Kraken"))
                    var stat = kraken && kraken.status.find(s => s.key === "Liquid temperature")
                    if (stat) temp = Math.round(stat.value)
                } catch (_) {}
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
            text: "󰖎"
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
