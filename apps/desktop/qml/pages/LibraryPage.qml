import QtQuick
import QtQuick.Controls
import QtQuick.Layouts
import QtQuick.Dialogs
import ".."
import "../components"
import "../dialogs"

Item {
    id: page
    property var api
    property var events
    property bool experimentalAudio: false

    property var models: []
    property var instances: ({})
    property var liveActivity: ({})
    property bool scanning: false
    property string filter: ""
    property var selected: null

    function modalityTag(m) {
        if (!m) return ""
        var meta = m.metadata || {}
        if (meta.speculative_draft) {
            if (meta.spec_type === "draft-mtp" || meta.has_mtp) return "mtp draft"
            if (meta.spec_type) return String(meta.spec_type).replace("draft-", "") + " draft"
            return "draft"
        }
        if (m.projector_path === "") {
            if (meta.has_mtp) return "mtp"
            return ""
        }
        var hasA = !!meta.has_audio
        var hasV = !!meta.has_vision
        if (page.experimentalAudio) {
            if (hasA && hasV) return "audio+vision"
            if (hasA) return "audio"
            if (hasV) return "vision"
            return "multimodal"
        }
        // Setting off: keep historical "vision" label for any projector pair.
        return "vision"
    }

    signal openDetail(string modelId)

    function reload() {
        api.get("/api/v1/models", function(st, data) {
            if (st === 200) page.models = (data && data.models) || []
        })
        api.get("/api/v1/instances", function(st, data) {
            if (st !== 200) return
            var byModel = {}
            var list = (data && data.instances) || []
            for (var i = 0; i < list.length; i++) byModel[list[i].model_id] = list[i]
            page.instances = byModel
            var next = {}
            for (var id in page.liveActivity)
                if (byModel[id]) next[id] = page.liveActivity[id]
            page.liveActivity = next
        })
    }

    Connections {
        target: page.events
        function onEventReceived(name, payload) {
            if (name === "instance.activity") {
                // Copy so QML bindings notice the change.
                var p = {}
                var old = page.liveActivity
                for (var k in old) p[k] = old[k]
                p[payload.model_id] = payload
                page.liveActivity = p
            } else if (name === "instance.state_changed" || name === "instance.updated"
                || name === "library.scanned") {
                page.reload()
            }
        }
    }

    function statusText(modelId) {
        var inst = page.instances[modelId]
        if (!inst) return ""
        var act = page.liveActivity[modelId]
        if (act && act.busy && (inst.state === "busy" || inst.state === "ready"))
            return "Processing · " + act.decoded_total + " tok · " + act.tokens_per_second.toFixed(1) + " tok/s"
        return inst.state
    }

    function filteredModels() {
        var f = page.filter.toLowerCase()
        return page.models.filter(function(m) {
            return f === "" || m.alias.toLowerCase().indexOf(f) >= 0
                || m.quantization.toLowerCase().indexOf(f) >= 0
                || m.architecture.toLowerCase().indexOf(f) >= 0
        })
    }

    RowLayout {
        anchors.fill: parent
        spacing: 0

        // Model list
        ColumnLayout {
            Layout.fillWidth: true
            Layout.fillHeight: true
            Layout.margins: AppTheme.pad
            spacing: AppTheme.gap

            RowLayout {
                Layout.fillWidth: true
                spacing: 8
                TextField {
                    Layout.fillWidth: true
                    placeholderText: "Filter by name, quantization, architecture…"
                    onTextChanged: page.filter = text
                }
                Button {
                    text: page.scanning ? "Scanning…" : "Rescan"
                    enabled: !page.scanning
                    onClicked: {
                        page.scanning = true
                        page.api.post("/api/v1/models/scan", {}, function() {
                            page.scanning = false
                            page.reload()
                        })
                    }
                }
                Button {
                    text: "Import file…"
                    onClicked: importDialog.open()
                }
            }

            ListView {
                Layout.fillWidth: true
                Layout.fillHeight: true
                clip: true
                spacing: 8
                model: page.filteredModels()

                EmptyState {
                    visible: page.models.length === 0
                    anchors.centerIn: parent
                    icon: "▤"
                    title: "No local models"
                    hint: "Download a model from Discover, or import an existing GGUF file."
                    actionText: "Rescan library"
                    onActionTriggered: page.api.post("/api/v1/models/scan", {}, function() { page.reload() })
                }

                delegate: Card {
                    width: ListView.view.width
                    implicitHeight: mrow.implicitHeight + 20
                    RowLayout {
                        id: mrow
                        anchors.fill: parent
                        anchors.margins: 10
                        spacing: 12

                        // Generated-initials icon (original artwork)
                        Rectangle {
                            width: 40; height: 40; radius: 8
                            color: modelData.favorite ? AppTheme.warning : AppTheme.accent
                            Text {
                                anchors.centerIn: parent
                                text: (modelData.alias || "?").substring(0, 2).toUpperCase()
                                color: AppTheme.onAccent
                                font.weight: Font.Bold
                            }
                        }

                        ColumnLayout {
                            Layout.fillWidth: true
                            spacing: 2
                            RowLayout {
                                spacing: 8
                                Text { text: modelData.alias; color: AppTheme.text; font.weight: Font.DemiBold; elide: Text.ElideRight; Layout.fillWidth: true }
                                Tag {
                                    visible: modelData.metadata && modelData.metadata.tensor_errors
                                        && modelData.metadata.tensor_errors.length > 0
                                    text: "corrupt file"
                                    tone: AppTheme.danger
                                    ToolTip.visible: corruptHover.hovered
                                    ToolTip.text: modelData.metadata && modelData.metadata.tensor_errors
                                        ? modelData.metadata.tensor_errors.join("\n") : ""
                                    HoverHandler { id: corruptHover }
                                }
                                Tag { visible: modelData.quantization !== ""; text: modelData.quantization; tone: AppTheme.info }
                                Tag { visible: modelData.architecture !== ""; text: modelData.architecture; tone: AppTheme.accent }
                                Tag {
                                    visible: page.modalityTag(modelData) !== ""
                                    text: page.modalityTag(modelData); tone: AppTheme.success
                                }
                            }
                            RowLayout {
                                spacing: 12
                                Text { text: AppTheme.bytes(modelData.size_bytes); color: AppTheme.textDim; font.pixelSize: AppTheme.fontSmall }
                                Text {
                                    visible: modelData.context_length > 0
                                    text: modelData.context_length + " ctx"
                                    color: AppTheme.textDim; font.pixelSize: AppTheme.fontSmall
                                }
                                Text {
                                    visible: (modelData.pinned_runtime || "") !== ""
                                    text: "pin · " + modelData.pinned_runtime
                                    color: AppTheme.textFaint; font.pixelSize: AppTheme.fontSmall
                                    elide: Text.ElideRight
                                }
                                Text {
                                    visible: (modelData.last_result || "") !== "" && modelData.last_result !== "ok"
                                    text: modelData.last_result
                                    color: AppTheme.danger; font.pixelSize: AppTheme.fontSmall
                                    elide: Text.ElideRight; Layout.fillWidth: true
                                }
                            }
                        }

                        // State + actions
                        RowLayout {
                            spacing: 6
                            StatusDot {
                                visible: page.instances[modelData.id] !== undefined
                                state: page.instances[modelData.id] ? page.instances[modelData.id].state : ""
                            }
                            Label {
                                visible: page.instances[modelData.id] !== undefined
                                text: page.statusText(modelData.id)
                                color: AppTheme.stateColor(page.instances[modelData.id] ? page.instances[modelData.id].state : "")
                                font.pixelSize: AppTheme.fontSmall
                            }
                            Button {
                                visible: page.instances[modelData.id] !== undefined
                                text: "Details"
                                flat: true
                                onClicked: page.openDetail(modelData.id)
                            }
                            Button {
                                visible: page.instances[modelData.id] === undefined
                                    || ["failed", "crashed"].indexOf(page.instances[modelData.id].state) >= 0
                                text: "Load…"
                                highlighted: true
                                onClicked: loadDialog.openFor(modelData)
                            }
                            Button {
                                visible: page.instances[modelData.id] !== undefined
                                    && ["ready", "busy", "sleeping"].indexOf(page.instances[modelData.id].state) >= 0
                                text: "Unload"
                                onClicked: page.api.post("/api/v1/models/" + modelData.id + "/unload", {}, function() { page.reload() })
                            }
                            Button {
                                visible: page.instances[modelData.id] !== undefined
                                    && ["failed", "crashed"].indexOf(page.instances[modelData.id].state) >= 0
                                text: "Diagnostics"
                                onClicked: failureDialog.openFor(modelData.id)
                            }
                            ToolButton {
                                text: "⋯"
                                onClicked: modelMenu.popup()
                                Menu {
                                    id: modelMenu
                                    MenuItem {
                                        text: modelData.favorite ? "Unfavorite" : "Favorite"
                                        onTriggered: page.api.patch("/api/v1/models/" + modelData.id,
                                            { "favorite": !modelData.favorite }, function() { page.reload() })
                                    }
                                    MenuItem {
                                        text: "Details / notes…"
                                        onTriggered: { page.selected = modelData; detailDrawer.open() }
                                    }
                                    MenuItem {
                                        text: "Reveal files"
                                        onTriggered: Qt.openUrlExternally("file://" + modelData.primary_path.substring(0, modelData.primary_path.lastIndexOf("/")))
                                    }
                                    MenuSeparator {}
                                    MenuItem {
                                        text: "Delete…"
                                        onTriggered: {
                                            page.selected = modelData
                                            page.api.del("/api/v1/models/" + modelData.id, function(st, data) {
                                                if (data && data.requires_confirmation) {
                                                    deleteDialog.paths = data.paths || []
                                                    deleteDialog.open()
                                                }
                                            })
                                        }
                                    }
                                }
                            }
                        }
                    }
                }
            }
        }

        // Detail drawer
        Drawer {
            id: detailDrawer
            edge: Qt.RightEdge
            width: 400
            height: page.height
            interactive: false
            ColumnLayout {
                anchors.fill: parent
                anchors.margins: AppTheme.pad
                spacing: AppTheme.gap
                visible: page.selected !== null
                Label {
                    text: page.selected ? page.selected.alias : ""
                    font.pixelSize: AppTheme.fontTitle
                    font.weight: Font.DemiBold
                    color: AppTheme.text
                }
                FormField {
                    Layout.fillWidth: true
                    label: "Alias"; hint: "Display and API name."
                    TextField {
                        width: parent.width
                        text: page.selected ? page.selected.alias : ""
                        onEditingFinished: page.api.patch("/api/v1/models/" + page.selected.id,
                            { "alias": text }, function() { page.reload() })
                    }
                }
                FormField {
                    Layout.fillWidth: true
                    label: "Notes"; hint: "Personal notes about this model."
                    TextArea {
                        width: parent.width
                        height: 80
                        text: page.selected ? page.selected.notes : ""
                        onEditingFinished: page.api.patch("/api/v1/models/" + page.selected.id,
                            { "notes": text }, function() { page.reload() })
                    }
                }
                GroupBox {
                    Layout.fillWidth: true
                    title: "Files"
                    Column {
                        width: parent.width
                        spacing: 2
                        Repeater {
                            model: page.selected ? page.selected.files : []
                            Label {
                                text: modelData
                                color: AppTheme.textDim
                                font.pixelSize: AppTheme.fontSmall
                                font.family: "monospace"
                                elide: Text.ElideMiddle
                                width: parent.width
                            }
                        }
                    }
                }
                Item { Layout.fillHeight: true }
                Button {
                    text: "Close"
                    Layout.alignment: Qt.AlignRight
                    onClicked: detailDrawer.close()
                }
            }
        }
    }

    FileDialog {
        id: importDialog
        title: "Import a GGUF model file"
        nameFilters: ["GGUF models (*.gguf)"]
        onAccepted: page.api.post("/api/v1/models/import", { "path": String(selectedFile).replace("file://", "") },
            function(st, data) {
                if (st === 201) page.reload()
            })
    }

    LoadConfigDialog {
        id: loadDialog
        api: page.api
        onLoaded: page.reload()
    }

    FailureDialog {
        id: failureDialog
        api: page.api
        onRetry: page.api.post("/api/v1/models/" + modelId + "/load", {}, function() { page.reload() })
        onRetrySafe: page.api.post("/api/v1/models/" + modelId + "/load",
            { "gpu_offload": "auto", "flash_attention": "auto", "context_length": 0 },
            function() { page.reload() })
        onRetryCpu: page.api.post("/api/v1/models/" + modelId + "/load",
            { "gpu_offload": "none" }, function() { page.reload() })
    }

    ConfirmDialog {
        id: deleteDialog
        message: "Delete this model? Files inside the managed model directory will be removed. Library entries only are removed otherwise."
        confirmText: "Delete"
        paths: []
        onConfirmed: page.api.del("/api/v1/models/" + page.selected.id + "?confirmed=1&delete_files=1",
            function() { page.reload() })
    }

    Component.onCompleted: reload()
}
