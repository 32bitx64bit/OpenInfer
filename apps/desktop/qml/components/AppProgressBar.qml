import QtQuick
import QtQuick.Controls
import ".."

ProgressBar {
    id: root

    implicitHeight: 8
    from: 0
    to: 1

    background: Rectangle {
        implicitHeight: 8
        radius: 4
        color: AppTheme.surfaceHi
        border.color: AppTheme.border
        border.width: 1
    }

    contentItem: Item {
        implicitHeight: 8
        Rectangle {
            width: root.visualPosition * parent.width
            height: parent.height
            radius: 4
            color: AppTheme.accent
        }
    }
}
