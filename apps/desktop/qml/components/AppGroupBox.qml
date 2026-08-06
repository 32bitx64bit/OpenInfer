import QtQuick
import QtQuick.Controls
import QtQuick.Layouts
import ".."

GroupBox {
    id: root

    topPadding: title.length ? 36 : AppTheme.padSmall
    leftPadding: AppTheme.pad
    rightPadding: AppTheme.pad
    bottomPadding: AppTheme.pad

    label: Label {
        x: root.leftPadding
        width: root.availableWidth
        text: root.title
        color: AppTheme.text
        font.pixelSize: AppTheme.fontTitle
        font.weight: Font.DemiBold
        elide: Text.ElideRight
    }

    background: Rectangle {
        y: root.topPadding - root.bottomPadding / 2
        width: parent.width
        height: parent.height - root.topPadding + root.bottomPadding / 2
        color: AppTheme.surface
        border.color: AppTheme.border
        radius: AppTheme.radius
    }
}
