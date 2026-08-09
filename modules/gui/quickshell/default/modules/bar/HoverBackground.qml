import QtQuick
import "../../theme/"

Rectangle {
    property alias containsMouse: hoverHandler.hovered
    
    anchors.fill: parent
    // Expand a bit to give padding around the text
    anchors.margins: -Sizes.marginXs
    radius: Shape.radiusSm
    color: Colors.base02
    opacity: containsMouse ? 1 : 0

    Behavior on opacity {
        NumberAnimation { duration: 150 }
    }

    HoverHandler {
        id: hoverHandler
    }
}
