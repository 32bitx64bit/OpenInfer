import QtQuick
import QtQuick.Controls
import QtQuick.Layouts
import ".."
import "../components"

Item {
    id: page
    property var api
    property var events

    property var results: []
    property bool searching: false
    property string searchError: ""
    property var detail: null
    property var detailGroups: []
    property var detailProjectors: []
    property bool withVision: true
    property bool detailLoading: false
    property bool hasToken: false

    function reload() {
        api.get("/api/v1/hf/token", function(st, data) {
            if (st === 200) page.hasToken = data && data.configured
        })
    }

    function search() {
        page.searching = true
        page.searchError = ""
        var q = encodeURIComponent(searchField.text)
        var sort = sortCombo.currentValue
        api.get("/api/v1/hf/search?q=" + q + "&sort=" + sort + "&limit=40", function(st, data) {
            page.searching = false
            if (st === 200) {
                page.results = (data && data.results) || []
            } else {
                page.searchError = (data && (data.detail || data.error)) || ("HTTP " + st)
            }
        })
    }

    function openRepo(repoId) {
        page.detailLoading = true
        page.detail = null
        page.withVision = true
        detailDialog.open()
        api.get("/api/v1/hf/repo/" + repoId, function(st, data) {
            page.detailLoading = false
            if (st === 200) {
                page.detail = data.repo
                page.detailGroups = data.groups || []
                page.detailProjectors = data.projectors || []
            } else {
                page.searchError = (data && (data.detail || data.error)) || ("HTTP " + st)
                detailDialog.close()
            }
        })
    }

    function projectorBytes() {
        var t = 0
        for (var i = 0; i < page.detailProjectors.length; i++) t += page.detailProjectors[i].size
        return t
    }

    function downloadGroup(group) {
        var files = group.files.map(function(f) { return { "path": f.path, "size": f.size } })
        var hasProjector = group.files.some(function(f) { return f.kind === "projector" })
        if (page.withVision && !hasProjector) {
            for (var i = 0; i < page.detailProjectors.length; i++)
                files.push({ "path": page.detailProjectors[i].path, "size": page.detailProjectors[i].size })
        }
        api.post("/api/v1/downloads", {
            "kind": "model",
            "label": (page.detail ? page.detail.id : "") + " " + group.label,
            "repo": page.detail.id,
            "group": group.id,
            "files": files
        }, function(st, data) {
            if (st !== 201)
                page.searchError = (data && (data.detail || data.error)) || "download failed"
        })
        detailDialog.close()
    }

    ColumnLayout {
        anchors.fill: parent
        anchors.margins: AppTheme.pad
        spacing: AppTheme.gap

        RowLayout {
            Layout.fillWidth: true
            spacing: 8
            TextField {
                id: searchField
                Layout.fillWidth: true
                placeholderText: "Search Hugging Face for GGUF models…"
                onAccepted: page.search()
            }
            ComboBox {
                id: sortCombo
                model: [
                    { "text": "Relevance", "value": "" },
                    { "text": "Downloads", "value": "downloads" },
                    { "text": "Likes", "value": "likes" },
                    { "text": "Trending", "value": "trending" },
                    { "text": "Recently updated", "value": "lastModified" }
                ]
                textRole: "text"
                valueRole: "value"
            }
            Button { text: "Search"; highlighted: true; onClicked: page.search() }
        }

        Label {
            visible: page.searchError !== ""
            Layout.fillWidth: true
            text: page.searchError
            color: AppTheme.danger
            wrapMode: Text.WordWrap
        }

        BusyIndicator { visible: page.searching; Layout.alignment: Qt.AlignHCenter }

        ListView {
            Layout.fillWidth: true
            Layout.fillHeight: true
            clip: true
            spacing: 8
            model: page.results

            EmptyState {
                visible: page.results.length === 0 && !page.searching
                anchors.centerIn: parent
                icon: "⌕"
                title: "Search for models"
                hint: "Search Hugging Face for GGUF repositories. Results are grouped by quantization, split set, and projector files."
            }

            delegate: Card {
                width: ListView.view.width
                implicitHeight: row.implicitHeight + 20
                RowLayout {
                    id: row
                    anchors.fill: parent
                    anchors.margins: 10
                    spacing: 12
                    Rectangle {
                        width: 40; height: 40; radius: 20
                        color: AppTheme.accent
                        Text {
                            anchors.centerIn: parent
                            text: (modelData.author || "?").substring(0, 2).toUpperCase()
                            color: AppTheme.onAccent
                            font.weight: Font.Bold
                        }
                    }
                    ColumnLayout {
                        Layout.fillWidth: true
                        spacing: 2
                        RowLayout {
                            Text { text: modelData.id; color: AppTheme.text; font.weight: Font.DemiBold; elide: Text.ElideRight; Layout.fillWidth: true }
                            Tag { visible: modelData.gated !== false && modelData.gated !== null; text: "gated"; tone: AppTheme.warning }
                            Tag { visible: modelData.private; text: "private"; tone: AppTheme.danger }
                        }
                        RowLayout {
                            spacing: 12
                            Text { text: "↓ " + modelData.downloads; color: AppTheme.textDim; font.pixelSize: AppTheme.fontSmall }
                            Text { text: "likes " + modelData.likes; color: AppTheme.textDim; font.pixelSize: AppTheme.fontSmall }
                            Text { text: (modelData.tags || []).slice(0, 5).join("  "); color: AppTheme.textFaint; font.pixelSize: AppTheme.fontSmall; elide: Text.ElideRight; Layout.fillWidth: true }
                        }
                    }
                    Button { text: "Details"; onClicked: page.openRepo(modelData.id) }
                }
            }
        }
    }

    // Repository detail popup — nearly full-window, modal, always closable.
    Dialog {
        id: detailDialog
        anchors.centerIn: parent
        width: page.width * 0.92
        height: page.height * 0.92
        modal: true
        standardButtons: Dialog.NoButton
        padding: 0

        background: Rectangle {
            color: AppTheme.bg
            radius: AppTheme.radius
            border.color: AppTheme.border
        }

        contentItem: ColumnLayout {
            spacing: 0

            Rectangle {
                Layout.fillWidth: true
                Layout.preferredHeight: 48
                color: AppTheme.bgAlt
                radius: AppTheme.radius
                RowLayout {
                    anchors.fill: parent
                    anchors.leftMargin: AppTheme.pad
                    anchors.rightMargin: 8
                    spacing: 8
                    Label {
                        Layout.fillWidth: true
                        text: page.detail ? page.detail.id : "Loading repository…"
                        font.pixelSize: AppTheme.fontTitle
                        font.weight: Font.DemiBold
                        color: AppTheme.text
                        elide: Text.ElideMiddle
                    }
                    Button {
                        text: "Open in browser"
                        flat: true
                        visible: page.detail !== null
                        onClicked: Qt.openUrlExternally("https://huggingface.co/" + page.detail.id)
                    }
                    ToolButton {
                        text: "✕"
                        Accessible.name: "Close"
                        onClicked: detailDialog.close()
                    }
                }
            }

            BusyIndicator {
                visible: page.detailLoading
                Layout.alignment: Qt.AlignHCenter
                Layout.topMargin: 40
            }

            ColumnLayout {
                visible: page.detail !== null
                Layout.fillWidth: true
                Layout.fillHeight: true
                Layout.margins: AppTheme.pad
                spacing: AppTheme.gap

                Label {
                    Layout.fillWidth: true
                    visible: page.detail && page.detail.gated !== false && page.detail.gated !== null
                    text: "This repository is gated: accept its terms on Hugging Face, then add your access token in Settings."
                    color: AppTheme.warning
                    wrapMode: Text.WordWrap
                }

                Label {
                    text: "Downloads: " + (page.detail ? page.detail.downloads : 0)
                        + "   Likes: " + (page.detail ? page.detail.likes : 0)
                    color: AppTheme.textDim
                    font.pixelSize: AppTheme.fontSmall
                }

                RowLayout {
                    Layout.fillWidth: true
                    visible: page.detailProjectors.length > 0
                    spacing: 8
                    Switch {
                        id: visionToggle
                        checked: page.withVision
                        onToggled: page.withVision = checked
                    }
                    Label {
                        text: "Download with vision (mmproj · " + AppTheme.bytes(page.projectorBytes()) + ")"
                        color: AppTheme.text
                        ToolTip.visible: visionHover.hovered
                        ToolTip.text: page.detailProjectors.map(function(p) { return p.path }).join("\n")
                        HoverHandler { id: visionHover }
                        MouseArea {
                            anchors.fill: parent
                            onClicked: visionToggle.toggle()
                        }
                    }
                    Item { Layout.fillWidth: true }
                }

                GroupBox {
                    Layout.fillWidth: true
                    Layout.fillHeight: true
                    title: "Available files (" + page.detailGroups.length + " groups)"
                    ListView {
                        anchors.fill: parent
                        clip: true
                        spacing: 8
                        model: page.detailGroups
                        delegate: Card {
                            width: ListView.view.width - 4
                            implicitHeight: gcol.implicitHeight + 20
                            ColumnLayout {
                                id: gcol
                                anchors.fill: parent
                                anchors.margins: 10
                                spacing: 6
                                RowLayout {
                                    Layout.fillWidth: true
                                    Label { text: modelData.label; color: AppTheme.text; font.weight: Font.DemiBold }
                                    Tag { visible: modelData.split; text: modelData.parts + " parts"; tone: AppTheme.info }
                                    Tag { visible: modelData.vision; text: "vision"; tone: AppTheme.success }
                                    Item { Layout.fillWidth: true }
                                    Label {
                                        text: {
                                            var t = modelData.total_bytes
                                            var hasProj = modelData.files.some(function(f) { return f.kind === "projector" })
                                            if (page.withVision && page.detailProjectors.length > 0 && !hasProj)
                                                return AppTheme.bytes(t + page.projectorBytes()) + " (incl. vision)"
                                            return AppTheme.bytes(t)
                                        }
                                        color: AppTheme.textDim
                                        font.pixelSize: AppTheme.fontSmall
                                    }
                                }
                                Label {
                                    text: "Estimated memory: ~" + AppTheme.bytes(modelData.est_memory_bytes) + " (estimate)"
                                    color: AppTheme.textFaint
                                    font.pixelSize: AppTheme.fontSmall
                                }
                                Repeater {
                                    model: modelData.files
                                    Label {
                                        text: "  " + modelData.path + "  ·  " + AppTheme.bytes(modelData.size)
                                        color: AppTheme.textDim
                                        font.pixelSize: AppTheme.fontSmall
                                        font.family: "monospace"
                                        elide: Text.ElideMiddle
                                        Layout.fillWidth: true
                                    }
                                }
                                RowLayout {
                                    Layout.fillWidth: true
                                    Item { Layout.fillWidth: true }
                                    Button {
                                        text: "Download"
                                        highlighted: true
                                        onClicked: page.downloadGroup(modelData)
                                    }
                                }
                            }
                        }
                    }
                }

                GroupBox {
                    Layout.fillWidth: true
                    Layout.preferredHeight: 160
                    title: "Model card"
                    ScrollView {
                        anchors.fill: parent
                        clip: true
                        TextEdit {
                            width: parent.width
                            readOnly: true
                            text: page.detail ? (page.detail.card || "No model card.") : ""
                            textFormat: TextEdit.MarkdownText
                            wrapMode: TextEdit.Wrap
                            color: AppTheme.textDim
                            font.pixelSize: AppTheme.fontSmall
                        }
                    }
                }
            }

            // Footer with an explicit close action (Escape also closes).
            Rectangle {
                Layout.fillWidth: true
                Layout.preferredHeight: 48
                color: AppTheme.bgAlt
                radius: AppTheme.radius
                RowLayout {
                    anchors.fill: parent
                    anchors.leftMargin: AppTheme.pad
                    anchors.rightMargin: AppTheme.pad
                    Item { Layout.fillWidth: true }
                    Button {
                        text: "Close"
                        onClicked: detailDialog.close()
                    }
                }
            }
        }
    }

    Component.onCompleted: reload()
}
