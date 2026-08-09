import QtQuick
import QtQuick.Layouts
import Quickshell.Services.Mpris
import "../../theme/"

BarPopup {
    id: root

    implicitWidth: 200

    property MprisPlayer player: null

    Text {
        text: root.player?.trackTitle ?? "No Track"
        color: Colors.base05
        font.family: Fonts.monospace
        font.pixelSize: Sizes.textSize
        font.bold: true
        Layout.fillWidth: true
        horizontalAlignment: Text.AlignHCenter
        elide: Text.ElideRight
    }

    Text {
        text: root.player?.trackArtist ?? "Unknown Artist"
        color: Colors.base04
        font.family: Fonts.monospace
        font.pixelSize: Sizes.textSize
        Layout.fillWidth: true
        horizontalAlignment: Text.AlignHCenter
        elide: Text.ElideRight
    }

    RowLayout {
        Layout.alignment: Qt.AlignHCenter
        spacing: Sizes.spacingMd

        Text {
            text: "󰒮"
            color: root.player?.canGoPrevious ? Colors.base05 : Colors.base03
            font.family: Fonts.monospace
            font.pixelSize: 24
            
            MouseArea {
                anchors.fill: parent
                onClicked: root.player?.previous()
                enabled: root.player?.canGoPrevious ?? false
                cursorShape: enabled ? Qt.PointingHandCursor : Qt.ArrowCursor
            }
        }

        Text {
            text: root.player?.playbackState === MprisPlaybackState.Playing ? "󰏤" : "󰐊"
            color: Colors.base0B
            font.family: Fonts.monospace
            font.pixelSize: 32

            MouseArea {
                anchors.fill: parent
                onClicked: root.player?.togglePlaying()
                cursorShape: Qt.PointingHandCursor
            }
        }

        Text {
            text: "󰒭"
            color: root.player?.canGoNext ? Colors.base05 : Colors.base03
            font.family: Fonts.monospace
            font.pixelSize: 24

            MouseArea {
                anchors.fill: parent
                onClicked: root.player?.next()
                enabled: root.player?.canGoNext ?? false
                cursorShape: enabled ? Qt.PointingHandCursor : Qt.ArrowCursor
            }
        }
    }
}
