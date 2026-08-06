import QtQuick
import QtQuick.Controls
import QtQuick.Layouts
import ".."

// Left-rail navigation entry: equal-sized buttons with centered label.
ItemDelegate {
    id: root
    property string glyph: ""
    property bool current: false
    property bool compact: false

    Layout.fillWidth: true
    Layout.preferredHeight: 44
    height: 44
    width: parent ? parent.width : implicitWidth
    Accessible.name: text
    hoverEnabled: true

    background: Rectangle {
        color: root.current ? AppTheme.surfaceSelected
             : root.hovered ? AppTheme.surfaceHover : "transparent"
        radius: AppTheme.radius
        border.width: root.current ? 1 : 0
        border.color: root.current ? Qt.alpha(AppTheme.accent, 0.35) : "transparent"
    }

    contentItem: Item {
        anchors.fill: parent

        // Compact: centered glyph only.
        Text {
            visible: root.compact
            anchors.centerIn: parent
            text: root.glyph
            color: root.current ? AppTheme.accent : AppTheme.textDim
            font.pixelSize: 18
            horizontalAlignment: Text.AlignHCenter
            verticalAlignment: Text.AlignVCenter
        }

        // Expanded: glyph + label centered as one unit.
        Row {
            visible: !root.compact
            anchors.centerIn: parent
            spacing: 10
            Text {
                text: root.glyph
                color: root.current ? AppTheme.accent : AppTheme.textDim
                font.pixelSize: 16
                width: 20
                horizontalAlignment: Text.AlignHCenter
                anchors.verticalCenter: parent.verticalCenter
            }
            Text {
                text: root.text
                color: root.current ? AppTheme.text : AppTheme.textDim
                font.pixelSize: AppTheme.fontBody
                font.weight: root.current ? Font.DemiBold : Font.Normal
                horizontalAlignment: Text.AlignHCenter
                anchors.verticalCenter: parent.verticalCenter
            }
        }
    }
    ToolTip.visible: root.compact && hovered
    ToolTip.text: root.text
    ToolTip.delay: 400
}
