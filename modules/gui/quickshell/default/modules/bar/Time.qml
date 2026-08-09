import QtQuick
import Quickshell
import "../../theme/"

Text {
    SystemClock {
        id: clock
        precision: SystemClock.Seconds
    }

    text: Qt.formatDateTime(clock.date, "hh:mm")
    color: Colors.base0A
    font.family: Fonts.monospace
    font.pixelSize: Sizes.textSize
}
