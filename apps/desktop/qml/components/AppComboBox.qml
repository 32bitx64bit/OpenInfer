import QtQuick
import QtQuick.Controls
import ".."

ComboBox {
    id: root

    implicitHeight: 36
    leftPadding: 12
    rightPadding: 28
    hoverEnabled: true

    contentItem: Text {
        leftPadding: 0
        rightPadding: root.indicator.width + 8
        text: root.displayText
        font: root.font
        color: root.enabled ? AppTheme.text : AppTheme.textFaint
        verticalAlignment: Text.AlignVCenter
        elide: Text.ElideRight
    }

    background: Rectangle {
        radius: AppTheme.radiusSmall
        color: root.down ? AppTheme.surfaceHi
             : root.hovered ? AppTheme.surfaceHover : AppTheme.surface
        border.width: 1
        border.color: root.activeFocus ? AppTheme.borderFocus
                     : root.hovered ? AppTheme.textFaint : AppTheme.border
    }

    indicator: Text {
        x: root.width - width - 10
        y: root.topPadding + (root.availableHeight - height) / 2
        text: "▾"
        color: AppTheme.textDim
        font.pixelSize: AppTheme.fontSmall
    }

    popup: Popup {
        y: root.height + 4
        width: root.width
        implicitHeight: Math.min(contentItem.implicitHeight + 2, 280)
        padding: 1

        contentItem: ListView {
            clip: true
            implicitHeight: contentHeight
            model: root.popup.visible ? root.delegateModel : null
            currentIndex: root.highlightedIndex
            ScrollIndicator.vertical: ScrollIndicator { }
        }

        background: Rectangle {
            color: AppTheme.surface
            border.color: AppTheme.border
            radius: AppTheme.radiusSmall
        }
    }

    delegate: ItemDelegate {
        width: root.width
        hoverEnabled: true
        highlighted: root.highlightedIndex === index

        contentItem: Text {
            text: root.textRole ? (modelData[root.textRole] !== undefined
                ? modelData[root.textRole] : model[root.textRole]) : modelData
            color: parent.highlighted ? AppTheme.onAccent : AppTheme.text
            elide: Text.ElideRight
            verticalAlignment: Text.AlignVCenter
        }

        background: Rectangle {
            color: parent.highlighted ? AppTheme.accent
                 : parent.hovered ? AppTheme.surfaceHover : "transparent"
            radius: AppTheme.radiusSmall
        }
    }
}
