import QtQuick
import Quickshell
import "../../theme/"

Text {
    SystemClock {
        id: clock
        precision: SystemClock.Minutes
    }

    text: Qt.formatDateTime(clock.date, "dd MMM yyyy")
    color: Colors.base05
    font.family: Fonts.monospace
    font.pixelSize: Sizes.textSize
}
