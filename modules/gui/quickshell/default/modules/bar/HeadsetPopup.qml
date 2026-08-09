import QtQuick
import QtQuick.Layouts
import Quickshell.Io
import "../../theme/"

BarPopup {
    id: root

    implicitWidth: 240

    property var device: null
    property string _buf: ""

    Process {
        id: fetchProc
        command: ["headsetcontrol", "-o", "JSON"]
        running: false
        stdout: SplitParser {
            onRead: data => root._buf += data + "\n"
        }
        onRunningChanged: {
            if (!running && root._buf !== "") {
                try {
                    var json = JSON.parse(root._buf)
                    if (json.devices && json.devices.length > 0)
                        root.device = json.devices[0]
                } catch(e) { console.error("HeadsetPopup parse error:", e) }
                root._buf = ""
            }
        }
    }

    onVisibleChanged: {
        if (visible) {
            root._buf = ""
            root.device = null
            fetchProc.running = true
        }
    }

    Text {
        text: root.device ? root.device.device : "—"
        color: Colors.base05
        font.family: Fonts.monospace
        font.pixelSize: Sizes.textSize
        font.bold: true
        Layout.fillWidth: true
        elide: Text.ElideRight
    }

    RowLayout {
        spacing: Sizes.spacingXs
        Text {
            text: "󰋎"
            color: Colors.base0E
            font.family: Fonts.monospace
            font.pixelSize: Sizes.iconSize
        }
        Text {
            text: root.device ? root.device.battery.level + "%" : "—"
            color: Colors.base05
            font.family: Fonts.monospace
            font.pixelSize: Sizes.textSize
        }
        Text {
            text: root.device ? "· " + root.device.battery.status.replace("BATTERY_", "").toLowerCase() : ""
            color: Colors.base04
            font.family: Fonts.monospace
            font.pixelSize: Sizes.textSize
        }
    }

    Flow {
        Layout.fillWidth: true
        spacing: 4
        Repeater {
            model: root.device ? root.device.capabilities_str : []
            delegate: Rectangle {
                color: Colors.base02
                radius: Shape.radiusSm
                implicitWidth: capLabel.implicitWidth + Sizes.spacingSm
                implicitHeight: capLabel.implicitHeight + 6
                Text {
                    id: capLabel
                    anchors.centerIn: parent
                    text: modelData
                    color: Colors.base04
                    font.family: Fonts.monospace
                    font.pixelSize: Sizes.textSize
                }
            }
        }
    }
}
