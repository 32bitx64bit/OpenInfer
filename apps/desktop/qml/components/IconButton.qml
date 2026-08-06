import QtQuick
import QtQuick.Controls
import ".."

ToolButton {
    id: root

    property string iconText: ""
    property string description: ""
    property bool selected: false

    text: iconText
    Accessible.name: description !== "" ? description : iconText
    hoverEnabled: true
    implicitWidth: 36
    implicitHeight: 36

    contentItem: Text {
        text: root.iconText
        color: root.selected ? AppTheme.accent : AppTheme.textDim
        font.pixelSize: 17
        horizontalAlignment: Text.AlignHCenter
        verticalAlignment: Text.AlignVCenter
    }
    background: Rectangle {
        radius: AppTheme.radiusSmall
        color: root.selected ? AppTheme.surfaceSelected
             : root.down ? AppTheme.surfaceHi
             : root.hovered ? AppTheme.surfaceHover : "transparent"
        border.color: root.activeFocus ? AppTheme.borderFocus : "transparent"
        border.width: root.activeFocus ? 1 : 0
    }
    ToolTip.visible: hovered && description !== ""
    ToolTip.text: description
}
