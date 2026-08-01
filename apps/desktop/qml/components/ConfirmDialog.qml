import QtQuick
import QtQuick.Controls
import ".."

// Destructive-action confirmation dialog.
Dialog {
    id: root
    property string confirmText: "Delete"
    property string message: ""
    property var paths: []      // exact affected paths, when relevant
    property bool destructive: true
    signal confirmed()

    title: "Confirm"
    modal: true
    anchors.centerIn: parent
    width: Math.min(480, parent ? parent.width - 64 : 480)
    standardButtons: Dialog.NoButton

    Column {
        width: parent.width
        spacing: AppTheme.gap
        Label { text: root.message; wrapMode: Text.WordWrap; width: parent.width; color: AppTheme.text }
        Column {
            visible: root.paths.length > 0
            width: parent.width
            spacing: 2
            Repeater {
                model: root.paths
                Label {
                    text: modelData
                    font.pixelSize: AppTheme.fontSmall
                    color: AppTheme.textDim
                    elide: Text.ElideMiddle
                    width: parent.width
                }
            }
        }
        Row {
            spacing: 8
            anchors.right: parent.right
            Button { text: "Cancel"; onClicked: root.close() }
            Button {
                text: root.confirmText
                highlighted: !root.destructive
                onClicked: { root.confirmed(); root.close() }
                palette.buttonText: root.destructive ? "white" : AppTheme.onAccent
                background: Rectangle {
                    radius: AppTheme.radiusSmall
                    color: root.destructive ? AppTheme.danger
                         : parent.down ? AppTheme.accentHi : AppTheme.accent
                }
            }
        }
    }
}
