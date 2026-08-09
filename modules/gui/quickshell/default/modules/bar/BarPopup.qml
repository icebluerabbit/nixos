import QtQuick
import QtQuick.Layouts
import Quickshell
import "../../theme/"

PopupWindow {
    id: root

    property var anchorWindow: null
    property real anchorX: 0
    property real anchorItemWidth: 0

    default property alias content: innerLayout.data

    color: "transparent"
    implicitHeight: innerLayout.implicitHeight + Sizes.marginMd * 2

    anchor.window: anchorWindow
    anchor.rect: Qt.rect(anchorX, 0, anchorItemWidth, anchorWindow ? anchorWindow.height : 0)
    anchor.edges: Edges.Bottom | Edges.Left
    anchor.gravity: Edges.Bottom | Edges.Right

    Rectangle {
        anchors.fill: parent
        color: Colors.base01
        radius: Shape.radiusMd

        ColumnLayout {
            id: innerLayout
            anchors {
                top: parent.top
                left: parent.left
                right: parent.right
                margins: Sizes.marginMd
            }
            spacing: Sizes.spacingSm
        }
    }
}
