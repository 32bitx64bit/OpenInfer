import QtQuick
import QtQuick.Controls
import ".."

// Left-rail navigation entry with icon glyph, label and active indicator.
ItemDelegate {
    id: root
    property string glyph: ""
    property bool current: false

    width: parent ? parent.width : 0
    height: 44
    Accessible.name: text

    background: Rectangle {
        color: root.current ? AppTheme.surfaceHi
             : root.hovered ? AppTheme.surface : "transparent"
        radius: AppTheme.radius
        Rectangle {
            visible: root.current
            width: 3; radius: 2
            height: parent.height - 14
            anchors.verticalCenter: parent.verticalCenter
            anchors.left: parent.left
            anchors.leftMargin: 4
            color: AppTheme.accent
        }
    }

    contentItem: Row {
        spacing: 10
        leftPadding: 14
        Text {
            text: root.glyph
            color: root.current ? AppTheme.accent : AppTheme.textDim
            font.pixelSize: 16
            width: 22
            horizontalAlignment: Text.AlignHCenter
            anchors.verticalCenter: parent.verticalCenter
        }
        Text {
            text: root.text
            color: root.current ? AppTheme.text : AppTheme.textDim
            font.pixelSize: AppTheme.fontBody
            font.weight: root.current ? Font.DemiBold : Font.Normal
            anchors.verticalCenter: parent.verticalCenter
        }
    }
}
