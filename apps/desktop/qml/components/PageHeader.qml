import QtQuick
import QtQuick.Controls
import QtQuick.Layouts
import ".."

RowLayout {
    id: root

    property string title: ""
    property string subtitle: ""
    property string eyebrow: ""
    property alias actions: actionsSlot.data

    Layout.fillWidth: true
    spacing: AppTheme.gap

    ColumnLayout {
        Layout.fillWidth: true
        spacing: 2

        Label {
            visible: root.eyebrow !== ""
            text: root.eyebrow.toUpperCase()
            color: AppTheme.accent
            font.pixelSize: AppTheme.fontSmall
            font.weight: Font.DemiBold
            font.letterSpacing: 1.1
        }
        Label {
            text: root.title
            color: AppTheme.text
            font.pixelSize: AppTheme.fontHero
            font.weight: Font.DemiBold
        }
        Label {
            visible: root.subtitle !== ""
            Layout.fillWidth: true
            text: root.subtitle
            color: AppTheme.textDim
            font.pixelSize: AppTheme.fontBody
            wrapMode: Text.WordWrap
        }
    }

    RowLayout {
        id: actionsSlot
        spacing: AppTheme.gapTight
    }
}
