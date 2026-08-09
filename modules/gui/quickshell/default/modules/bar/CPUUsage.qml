import QtQuick
import Quickshell.Io
import "../../theme/"

Item {
    id: root
    implicitWidth: row.implicitWidth
    implicitHeight: row.implicitHeight

    property int lastTotal: 0
    property int lastIdle: 0
    property string usage: "--"

    signal clicked

    // /proc/stat first line 'cpu' is an aggregate of all cores.
    // Columns: user nice system idle iowait irq softirq steal guest guest_nice
    // Work = user + nice + system + irq + softirq + steal
    // Idle = idle + iowait
    Process {
        id: proc
        command: ["sh", "-c", "awk '/^cpu / {print $2+$3+$4+$7+$8+$9, $5+$6}' /proc/stat"]
        running: true
        stdout: SplitParser {
            onRead: data => {
                var parts = data.trim().split(/\s+/)
                if (parts.length === 2) {
                    var work = parseInt(parts[0])
                    var idle = parseInt(parts[1])
                    var total = work + idle

                    if (lastTotal > 0) {
                        var totalDiff = total - lastTotal
                        var idleDiff = idle - lastIdle
                        if (totalDiff > 0) {
                            var usageVal = 100 * (totalDiff - idleDiff) / totalDiff
                            usage = Math.round(usageVal).toString()
                        }
                    }

                    lastTotal = total
                    lastIdle = idle
                }
            }
        }
    }

    Timer {
        interval: 2000
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
            color: Colors.base0E
            font.family: Fonts.monospace
            font.pixelSize: Sizes.iconSize
            anchors.verticalCenter: parent.verticalCenter
        }

        Text {
            text: usage
            color: usage === "--" ? Colors.base03 : Colors.base05
            font.family: Fonts.monospace
            font.pixelSize: Sizes.textSize
            anchors.verticalCenter: parent.verticalCenter
        }
    }
}
