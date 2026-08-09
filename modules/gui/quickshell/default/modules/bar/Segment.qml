import QtQuick
import QtQuick.Layouts
import QtQuick.Effects
import QtQuick.Shapes as QShapes
import "../../theme/"

Item {
    id: root
    default property alias content: innerLayout.data

    property int topLeftRadius: Shape.radiusMd
    property int topRightRadius: Shape.radiusMd
    property int bottomLeftRadius: Shape.radiusMd
    property int bottomRightRadius: Shape.radiusMd
    // false = sharp 90° top corner, true = concave wing
    property bool inverseCornerLeft: true
    property bool inverseCornerRight: true
    property color color: Colors.base01

    // effective top radii — 0 when wing disabled (sharp corner)
    readonly property int tlr: inverseCornerLeft ? topLeftRadius : 0
    readonly property int trr: inverseCornerRight ? topRightRadius : 0

    height: Sizes.height
    implicitWidth: innerLayout.implicitWidth + tlr + trr + Sizes.marginMd * 2

    layer.enabled: true
    layer.effect: MultiEffect {
        shadowEnabled: true
        shadowColor: "#80000000"
        shadowBlur: 0.5
        shadowVerticalOffset: 2
        shadowHorizontalOffset: 0
    }

    QShapes.Shape {
        anchors.fill: parent
        preferredRendererType: QShapes.Shape.CurveRenderer

        QShapes.ShapePath {
            strokeWidth: 0
            strokeColor: "transparent"
            fillColor: root.color

            // sharp top-left corner
            startX: 0
            startY: 0

            // top edge — full width, sharp top corners
            PathLine { x: root.width; y: 0 }

            // top-right: concave inward arc (macOS notch style)
            // center at (width, trr), CCW from north to west
            PathArc {
                x: root.width - root.trr
                y: root.trr
                radiusX: root.trr
                radiusY: root.trr
                direction: PathArc.Counterclockwise
            }

            // right body edge
            PathLine { x: root.width - root.trr; y: root.height - root.bottomRightRadius }

            // bottom-right convex
            PathArc {
                x: root.width - root.trr - root.bottomRightRadius
                y: root.height
                radiusX: root.bottomRightRadius
                radiusY: root.bottomRightRadius
                direction: PathArc.Clockwise
            }

            // bottom edge
            PathLine { x: root.tlr + root.bottomLeftRadius; y: root.height }

            // bottom-left convex
            PathArc {
                x: root.tlr
                y: root.height - root.bottomLeftRadius
                radiusX: root.bottomLeftRadius
                radiusY: root.bottomLeftRadius
                direction: PathArc.Clockwise
            }

            // left body edge
            PathLine { x: root.tlr; y: root.tlr }

            // top-left: concave inward arc (macOS notch style)
            // center at (0, tlr), CCW from east to north
            PathArc {
                x: 0
                y: 0
                radiusX: root.tlr
                radiusY: root.tlr
                direction: PathArc.Counterclockwise
            }
        }
    }

    RowLayout {
        id: innerLayout
        anchors.fill: parent
        anchors.leftMargin: root.tlr + Sizes.marginMd
        anchors.rightMargin: root.trr + Sizes.marginMd
        spacing: Sizes.spacingMd
    }
}
