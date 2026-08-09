import QtQuick
import QtQuick.Layouts
import Quickshell
import "../../theme/"

PanelWindow {
    id: bar

    required property var modelData
    screen: modelData

    anchors {
        top: true
        left: true
        right: true
    }
    implicitHeight: Sizes.height + Sizes.marginXs
    color: "transparent"

    HeadsetPopup {
        id: headsetPopup
        anchorWindow: bar
    }

    VolumePopup {
        id: volumePopup
        anchorWindow: bar
    }

    SpotifyPopup {
        id: spotifyPopup
        anchorWindow: bar
        player: spotifyWidget.player
    }

    CPUPopup {
        id: cpuPopup
        anchorWindow: bar
    }

    MemPopup {
        id: memPopup
        anchorWindow: bar
    }

    NetPopup {
        id: netPopup
        anchorWindow: bar
    }

    Item {
        anchors.fill: parent

        BarLeft {
            anchors.left: parent.left
            Workspaces {}
            CPUTemp {}
            NvidiaGPUTemp {}
            KrakenLiquidTemp {}
            KrakenFanSpeed {}
            FanSpeed { fan: 1 }
            FanSpeed { fan: 2 }
            FanSpeed { fan: 3 }
        }

        BarCenter {
            anchors.horizontalCenter: parent.horizontalCenter
            Title {}
        }

        BarRight {
            anchors.right: parent.right
            Spotify {
                id: spotifyWidget
                onClicked: {
                    var mapped = spotifyWidget.mapToItem(null, 0, 0)
                    spotifyPopup.anchorX = mapped.x
                    spotifyPopup.anchorItemWidth = spotifyWidget.width
                    spotifyPopup.visible = !spotifyPopup.visible
                }
            }
            NetUsage {
                id: netWidget
                onClicked: {
                    var mapped = netWidget.mapToItem(null, 0, 0)
                    netPopup.anchorX = mapped.x
                    netPopup.anchorItemWidth = netWidget.width
                    netPopup.visible = !netPopup.visible
                }
            }
            NetLatency {
                id: netLatencyWidget
            }
            CPUUsage {
                id: cpuWidget
                onClicked: {
                    var mapped = cpuWidget.mapToItem(null, 0, 0)
                    cpuPopup.anchorX = mapped.x
                    cpuPopup.anchorItemWidth = cpuWidget.width
                    cpuPopup.visible = !cpuPopup.visible
                }
            }
            MemUsage {
                id: memWidget
                onClicked: {
                    var mapped = memWidget.mapToItem(null, 0, 0)
                    memPopup.anchorX = mapped.x
                    memPopup.anchorItemWidth = memWidget.width
                    memPopup.visible = !memPopup.visible
                }
            }
            Volume {
                id: volumeWidget
                onClicked: {
                    var mapped = volumeWidget.mapToItem(null, 0, 0)
                    volumePopup.anchorX = mapped.x
                    volumePopup.anchorItemWidth = volumeWidget.width
                    volumePopup.visible = !volumePopup.visible
                }
            }
            HeadsetBattery {
                id: headsetBattery
                onClicked: {
                    var mapped = headsetBattery.mapToItem(null, 0, 0)
                    headsetPopup.anchorX = mapped.x
                    headsetPopup.anchorItemWidth = headsetBattery.width
                    headsetPopup.visible = !headsetPopup.visible
                }
            }
            Date {}
            Time {}
        }
    }
}
