import QtQuick
import QtQuick.Layouts
import Quickshell.Io
import "../../theme/"

Item {
    id: root
    implicitWidth: row.implicitWidth
    implicitHeight: row.implicitHeight

    property string latency: "--"
    property string targetHost: "1.1.1.1"

    signal clicked

    Process {
        id: proc
        command: ["sh", "-c", "ping -c 1 -W 2 " + targetHost + " | awk -F'time=' '/time=/ {split($2, a, \" \"); print a[1]; found=1; exit} END {if (!found) print \"offline\"}'"]
        running: true
        stdout: SplitParser {
            onRead: data => {
                var val = data.trim();
                if (val !== "") {
                    latency = val;
                }
            }
        }
    }

    Timer {
        interval: 30000 // 30 seconds
        repeat: true
        running: true
        onTriggered: proc.running = true
    }

    function getLatencyColor() {
        if (latency === "--") return Colors.base04;
        if (latency === "offline") return Colors.base08;

        var num = parseFloat(latency);
        if (isNaN(num)) return Colors.base05;
        if (num < 50) return Colors.base0B;
        if (num < 150) return Colors.base0A;
        return Colors.base09;
    }

    HoverBackground { anchors.fill: parent }
    MouseArea {
        anchors.fill: parent
        cursorShape: Qt.PointingHandCursor
        onClicked: {
            proc.running = true;
            root.clicked();
        }
    }

    RowLayout {
        id: row
        spacing: 2
        anchors.verticalCenter: parent.verticalCenter

        Text {
            text: "󰕑"
            color: getLatencyColor()
            font.family: Fonts.monospace
            font.pixelSize: Sizes.iconSize
        }

        Text {
            text: latency
            color: latency === "offline" ? Colors.base08 : Colors.base05
            font.family: Fonts.monospace
            font.pixelSize: Sizes.textSize
        }
    }
}
