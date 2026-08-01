import QtQuick
import QtQuick.Controls
import QtQuick.Layouts
import ".."

ColumnLayout {
    id: root
    property string icon: "◇"
    property string title: "Nothing here yet"
    property string hint: ""
    property string actionText: ""
    signal actionTriggered()

    spacing: 8

    Label {
        text: root.icon
        font.pixelSize: 40
        color: AppTheme.textFaint
        Layout.alignment: Qt.AlignHCenter
    }
    Label {
        text: root.title
        font.pixelSize: AppTheme.fontTitle
        color: AppTheme.textDim
        Layout.alignment: Qt.AlignHCenter
    }
    Label {
        text: root.hint
        font.pixelSize: AppTheme.fontSmall
        color: AppTheme.textFaint
        wrapMode: Text.WordWrap
        horizontalAlignment: Text.AlignHCenter
        Layout.alignment: Qt.AlignHCenter
        Layout.maximumWidth: 360
        Layout.fillWidth: true
    }
    Button {
        visible: root.actionText !== ""
        text: root.actionText
        Layout.alignment: Qt.AlignHCenter
        onClicked: root.actionTriggered()
    }
}
