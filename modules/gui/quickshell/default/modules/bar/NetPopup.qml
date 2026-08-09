import QtQuick
import QtQuick.Layouts
import Quickshell.Io
import "../../theme/"

BarPopup {
    id: root

    implicitWidth: 320

    function formatBytes(bytes) {
        if (bytes === 0) return "0 B"
        const k = 1024
        const sizes = ["B", "KB", "MB", "GB", "TB"]
        const i = Math.floor(Math.log(bytes) / Math.log(k))
        return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + " " + sizes[i]
    }

    ListModel {
        id: ifaceModel
    }

    property var lastStats: ({})
    property string _buffer: ""

    Process {
        id: proc
        command: ["sh", "-c", "ip -j -s link | jq -c '[.[] | select(.ifname != \"lo\") | {ifname: .ifname, operstate: .operstate, rx: .stats64.rx.bytes, tx: .stats64.tx.bytes}]'"]
        running: false
        stdout: SplitParser {
            onRead: data => root._buffer += data
        }
        onRunningChanged: {
            if (!running && root._buffer !== "") {
                try {
                    var parsed = JSON.parse(root._buffer)
                    updateModel(parsed)
                } catch(e) {
                    console.error("NetPopup JSON parse error:", e)
                }
                root._buffer = ""
            }
        }
    }

    function updateModel(interfaces) {
        var currentStats = {}

        for (var i = 0; i < interfaces.length; i++) {
            var iface = interfaces[i]
            var name = iface.ifname
            var rxBytes = iface.rx
            var txBytes = iface.tx
            
            currentStats[name] = { rx: rxBytes, tx: txBytes }

            var rxSpeed = 0
            var txSpeed = 0
            if (root.lastStats[name]) {
                rxSpeed = rxBytes - root.lastStats[name].rx
                txSpeed = txBytes - root.lastStats[name].tx
            }

            var found = false
            for (var j = 0; j < ifaceModel.count; j++) {
                if (ifaceModel.get(j).name === name) {
                    ifaceModel.setProperty(j, "operstate", iface.operstate)
                    ifaceModel.setProperty(j, "rxTotal", rxBytes)
                    ifaceModel.setProperty(j, "txTotal", txBytes)
                    ifaceModel.setProperty(j, "rxSpeed", rxSpeed)
                    ifaceModel.setProperty(j, "txSpeed", txSpeed)
                    found = true
                    break
                }
            }
            
            if (!found) {
                ifaceModel.append({
                    name: name,
                    operstate: iface.operstate,
                    rxTotal: rxBytes,
                    txTotal: txBytes,
                    rxSpeed: rxSpeed,
                    txSpeed: txSpeed
                })
            }
        }
        root.lastStats = currentStats
    }

    Timer {
        id: refreshTimer
        interval: 1000
        repeat: true
        running: root.visible
        onTriggered: proc.running = true
    }

    onVisibleChanged: {
        if (visible) {
            root._buffer = ""
            proc.running = true
        }
    }

    ColumnLayout {
        Layout.fillWidth: true
        spacing: Sizes.spacingSm

        Text {
            text: "Network Interfaces"
            color: Colors.base05
            font.family: Fonts.monospace
            font.pixelSize: Sizes.textSize
            font.bold: true
            Layout.alignment: Qt.AlignHCenter
        }

        Rectangle {
            Layout.fillWidth: true
            height: 1
            color: Colors.base03
        }

        Repeater {
            model: ifaceModel
            delegate: ColumnLayout {
                Layout.fillWidth: true
                spacing: 4

                RowLayout {
                    Layout.fillWidth: true
                    spacing: Sizes.spacingMd

                    Text {
                        text: model.name.startsWith("wl") ? (model.operstate === "UP" ? "󰖩" : "󰖪") : (model.operstate === "UP" ? "󰈀" : "󰈂")
                        color: model.operstate === "UP" ? Colors.base0B : Colors.base04
                        font.family: Fonts.monospace
                        font.pixelSize: 20
                    }

                    Text {
                        text: model.name
                        color: Colors.base05
                        font.family: Fonts.monospace
                        font.pixelSize: Sizes.textSize
                        font.bold: true
                        Layout.preferredWidth: 60
                    }

                    Item { Layout.fillWidth: true }
                }

                RowLayout {
                    Layout.fillWidth: true
                    Layout.leftMargin: 20 + Sizes.spacingMd

                    ColumnLayout {
                        spacing: 2
                        Layout.fillWidth: true
                        
                        Text {
                            text: "󰁅 " + root.formatBytes(model.rxSpeed) + "/s"
                            color: Colors.base0D
                            font.family: Fonts.monospace
                            font.pixelSize: 11
                        }
                        Text {
                            text: "Total: " + root.formatBytes(model.rxTotal)
                            color: Colors.base04
                            font.family: Fonts.monospace
                            font.pixelSize: 10
                        }
                    }

                    ColumnLayout {
                        spacing: 2
                        Layout.fillWidth: true
                        
                        Text {
                            text: "󰁝 " + root.formatBytes(model.txSpeed) + "/s"
                            color: Colors.base09
                            font.family: Fonts.monospace
                            font.pixelSize: 11
                            Layout.alignment: Qt.AlignRight
                        }
                        Text {
                            text: "Total: " + root.formatBytes(model.txTotal)
                            color: Colors.base04
                            font.family: Fonts.monospace
                            font.pixelSize: 10
                            Layout.alignment: Qt.AlignRight
                        }
                    }
                }
                
                Rectangle {
                    Layout.fillWidth: true
                    height: 1
                    color: Colors.base02
                    visible: index < ifaceModel.count - 1
                    Layout.topMargin: 4
                    Layout.bottomMargin: 4
                }
            }
        }
    }
}
