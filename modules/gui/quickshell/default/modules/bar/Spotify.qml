import QtQuick
import Quickshell.Services.Mpris
import "../../theme/"

Item {
    id: root
    implicitWidth: visible ? row.implicitWidth : 0
    implicitHeight: row.implicitHeight

    property MprisPlayer player: {
        for (var i = 0; i < Mpris.players.values.length; i++) {
            var p = Mpris.players.values[i]
            if (p.identity && p.identity.toLowerCase().includes("spotify"))
                return p
        }
        return null
    }

    property bool playing: player?.playbackState === MprisPlaybackState.Playing
    property string artist: player?.trackArtist ?? ""
    property string title: player?.trackTitle ?? ""
    property double position: player?.position ?? 0
    property double length: player?.length ?? 0

    visible: player !== null && title !== ""

    signal clicked

    HoverBackground { anchors.fill: parent }
    MouseArea {
        anchors.fill: parent
        cursorShape: Qt.PointingHandCursor
        onClicked: root.clicked()
    }

    function fmt(secs) {
        var s = Math.floor(secs)
        var m = Math.floor(s / 60)
        var h = Math.floor(m / 60)
        m = m % 60
        s = s % 60
        var mm = m.toString().padStart(2, "0")
        var ss = s.toString().padStart(2, "0")
        return h > 0 ? h + ":" + mm + ":" + ss : m + ":" + ss
    }

    Timer {
        interval: 1000
        running: root.playing
        repeat: true
        onTriggered: root.position = root.player?.position ?? 0
    }

    Row {
        id: row
        spacing: Sizes.spacingXs
        anchors.verticalCenter: parent.verticalCenter

        Text {
            text: "󰓇"
            color: Colors.base0B
            font.family: Fonts.monospace
            font.pixelSize: Sizes.iconSize
            anchors.verticalCenter: parent.verticalCenter
        }

        Text {
            text: root.artist
            color: Colors.base0D
            font.family: Fonts.monospace
            font.pixelSize: Sizes.textSize
            anchors.verticalCenter: parent.verticalCenter
            width: Math.min(implicitWidth, 150)
            elide: Text.ElideRight
        }

        Text {
            text: "—"
            color: Colors.base03
            font.family: Fonts.monospace
            font.pixelSize: Sizes.textSize
            anchors.verticalCenter: parent.verticalCenter
            visible: root.artist !== ""
        }

        Text {
            text: root.title
            color: Colors.base05
            font.family: Fonts.monospace
            font.pixelSize: Sizes.textSize
            anchors.verticalCenter: parent.verticalCenter
            width: Math.min(implicitWidth, 200)
            elide: Text.ElideRight
        }

        Text {
            text: root.fmt(root.position) + " / " + root.fmt(root.length)
            color: Colors.base04
            font.family: Fonts.monospace
            font.pixelSize: Sizes.textSize
            anchors.verticalCenter: parent.verticalCenter
            visible: root.length > 0
        }
    }
}
