import QtQuick
import QtQuick.Controls
import ".."

Column {
    id: root
    property string icon: "◇"
    property string title: "Nothing here yet"
    property string hint: ""
    property string actionText: ""
    signal actionTriggered()

    spacing: 8
    Text {
        text: root.icon
        font.pixelSize: 40
        color: AppTheme.textFaint
        anchors.horizontalCenter: parent.horizontalCenter
    }
    Text {
        text: root.title
        font.pixelSize: AppTheme.fontTitle
        color: AppTheme.textDim
        anchors.horizontalCenter: parent.horizontalCenter
    }
    Text {
        text: root.hint
        font.pixelSize: AppTheme.fontSmall
        color: AppTheme.textFaint
        wrapMode: Text.WordWrap
        horizontalAlignment: Text.AlignHCenter
        anchors.horizontalCenter: parent.horizontalCenter
        width: Math.min(360, parent ? parent.width : 360)
    }
    Button {
        visible: root.actionText !== ""
        text: root.actionText
        anchors.horizontalCenter: parent.horizontalCenter
        onClicked: root.actionTriggered()
    }
}
