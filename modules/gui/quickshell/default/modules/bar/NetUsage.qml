import QtQuick
import QtQuick.Layouts
import Quickshell.Io
import "../../theme/"

Item {
    id: root
    implicitWidth: row.implicitWidth
    implicitHeight: row.implicitHeight

    property real lastIn: 0
    property real lastOut: 0
    property string downSpeed: "0"
    property string upSpeed: "0"

    signal clicked

    function formatSpeed(bytes) {
        if (bytes < 1024) return bytes.toFixed(0);
        if (bytes < 1024 * 1024) return (bytes / 1024).toFixed(1);
        return (bytes / (1024 * 1024)).toFixed(1);
    }

    Process {
        id: proc
        command: ["sh", "-c", "awk '/eth0|wlan0|enp|wlp/ {d+=$2; u+=$10} END {print d, u}' /proc/net/dev"]
        running: true
        stdout: SplitParser {
            onRead: data => {
                var parts = data.trim().split(/\s+/);
                if (parts.length === 2) {
                    var currentIn = parseFloat(parts[0]);
                    var currentOut = parseFloat(parts[1]);

                    if (lastIn > 0) {
                        downSpeed = formatSpeed((currentIn - lastIn) / 2);
                        upSpeed = formatSpeed((currentOut - lastOut) / 2);
                    }

                    lastIn = currentIn;
                    lastOut = currentOut;
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

    RowLayout {
        id: row
        spacing: 2
        anchors.verticalCenter: parent.verticalCenter

        Text {
            text: "󰁅"
            color: Colors.base0D
            font.family: Fonts.monospace
            font.pixelSize: Sizes.iconSize
        }

        Text {
            text: downSpeed
            color: Colors.base05
            font.family: Fonts.monospace
            font.pixelSize: Sizes.textSize
        }

        Item { Layout.preferredWidth: Sizes.spacingSm }

        Text {
            text: "󰁝"
            color: Colors.base09
            font.family: Fonts.monospace
            font.pixelSize: Sizes.iconSize
        }

        Text {
            text: upSpeed
            color: Colors.base05
            font.family: Fonts.monospace
            font.pixelSize: Sizes.textSize
        }
    }
}
