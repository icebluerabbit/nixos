import QtQuick
import Quickshell
import "../../theme/"

Item {
    implicitWidth: row.implicitWidth
    implicitHeight: row.implicitHeight

    Row {
        id: row
        spacing: Sizes.spacingMd
        anchors.verticalCenter: parent.verticalCenter

        Repeater {
            model: niri.workspaces
            Rectangle {
                id: wsRect
                visible: index < 11
                height: 15
                radius: Shape.radiusFull
                color: model.isActive ? Colors.base0D : Colors.base03

                width: model.isActive ? (wsName.implicitWidth + 16) : 15

                Behavior on width {
                    NumberAnimation {
                        duration: 300
                        easing.type: Easing.OutQuint
                    }
                }

                Text {
                    id: wsName
                    text: model.name
                    anchors.centerIn: parent
                    color: Colors.base01
                    font.family: Fonts.monospace
                    font.pixelSize: 11
                    font.bold: true
                    opacity: model.isActive ? 1 : 0
                    visible: opacity > 0

                    Behavior on opacity {
                        NumberAnimation { duration: 200 }
                    }
                }

                MouseArea {
                    anchors.fill: parent
                    cursorShape: Qt.PointingHandCursor
                    onClicked: niri.focusWorkspaceById(model.id)
                }
            }
        }
    }
}
