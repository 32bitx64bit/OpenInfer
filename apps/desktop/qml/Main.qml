import QtQuick
import QtQuick.Controls
import QtQuick.Layouts
import "services"
import "components"
import "pages"
import "."

ApplicationWindow {
    id: window
    title: "OpenInfer Studio"
    width: 1280
    height: 800
    minimumWidth: 900
    minimumHeight: 600
    visible: true
    color: AppTheme.bg

    Api { id: api }
    Events {
        id: events
        onReconnected: window.reloadAll()
        onEventReceived: function(name, payload) {
            switch (name) {
            case "instance.state_changed":
            case "instance.updated":
                window.refreshInstances()
                break
            case "download.state_changed":
                window.refreshDownloadsBadge()
                break
            case "download.progress":
                // DownloadsPage handles live progress; badge only needs state changes.
                break
            case "runtime.installed":
                window.toast("Runtime installed: " + (payload.id || ""), "success")
                break
            case "library.scanned":
                break
            case "log.entry":
                break
            }
        }
    }

    property var instances: []
    property var hardware: null
    property var recommendation: null
    property int downloadCount: 0
    property bool experimentalAudioModels: false

    function refreshSettings() {
        api.get("/api/v1/settings", function(st, data) {
            if (st === 200 && data)
                window.experimentalAudioModels = (data["experimental.audio_models"] || "0") === "1"
        })
    }
    function refreshInstances() {
        api.get("/api/v1/instances", function(st, data) {
            if (st === 200 && data) window.instances = data.instances || []
        })
    }
    function refreshDownloadsBadge() {
        api.get("/api/v1/downloads", function(st, data) {
            if (st === 200 && data)
                window.downloadCount = (data.downloads || []).filter(
                    function(d) { return d.state === "active" || d.state === "queued" }).length
        })
    }
    function reloadAll() {
        refreshInstances()
        refreshDownloadsBadge()
        refreshSettings()
        api.get("/api/v1/hardware", function(st, data) {
            if (st === 200 && data) {
                window.hardware = data.hardware
                window.recommendation = data.recommendation
            }
        })
        for (var i = 0; i < stack.count; i++) {
            var p = stack.itemAt(i)
            if (p && p.reload) p.reload()
        }
    }

    ListModel { id: toastModel }
    function toast(text, kind) {
        toastModel.append({ "text": text, "kind": kind || "info" })
        toastTimer.restart()
    }
    Timer {
        id: toastTimer
        interval: 5000
        onTriggered: if (toastModel.count > 0) toastModel.remove(0)
    }

    ColumnLayout {
        anchors.fill: parent
        spacing: 0

        // Header bar
        Rectangle {
            Layout.fillWidth: true
            Layout.preferredHeight: 52
            color: AppTheme.bgAlt
            border.color: AppTheme.border

            RowLayout {
                anchors.fill: parent
                anchors.leftMargin: AppTheme.pad
                anchors.rightMargin: AppTheme.pad
                spacing: AppTheme.gap

                Text {
                    text: "OpenInfer Studio"
                    color: AppTheme.text
                    font.pixelSize: AppTheme.fontTitle
                    font.weight: Font.DemiBold
                }
                Text {
                    text: "local GGUF inference"
                    color: AppTheme.textFaint
                    font.pixelSize: AppTheme.fontSmall
                }

                Item { Layout.fillWidth: true }

                // Loaded-model chips
                Repeater {
                    model: window.instances
                    delegate: Row {
                        spacing: 6
                        StatusDot { state: modelData.state; anchors.verticalCenter: parent.verticalCenter }
                        Text {
                            text: modelData.model_alias || modelData.model_id
                            color: AppTheme.textDim
                            font.pixelSize: AppTheme.fontSmall
                            anchors.verticalCenter: parent.verticalCenter
                        }
                        Text {
                            visible: modelData.state === "failed" || modelData.state === "crashed"
                            text: "⚠"
                            color: AppTheme.danger
                            anchors.verticalCenter: parent.verticalCenter
                        }
                    }
                }

                Rectangle { Layout.preferredWidth: 1; Layout.preferredHeight: 24; color: AppTheme.border; visible: hwSummary.text !== "" }

                Text {
                    id: hwSummary
                    color: AppTheme.textDim
                    font.pixelSize: AppTheme.fontSmall
                    text: {
                        if (!window.hardware) return ""
                        var hw = window.hardware
                        var gpus = (hw.gpus || []).map(function(g) { return g.name }).join(", ")
                        return hw.cpu_model + "  ·  " + AppTheme.bytes(hw.ram_total) + " RAM"
                            + (gpus ? "  ·  " + gpus : "")
                    }
                    elide: Text.ElideRight
                    Layout.maximumWidth: 480
                }

                // Offline indicator
                Row {
                    visible: api.lastError !== "" && !events.connected
                    spacing: 6
                    Rectangle { width: 8; height: 8; radius: 4; color: AppTheme.danger; anchors.verticalCenter: parent.verticalCenter }
                    Text { text: "backend offline"; color: AppTheme.danger; font.pixelSize: AppTheme.fontSmall }
                }
            }
        }

        RowLayout {
            Layout.fillWidth: true
            Layout.fillHeight: true
            spacing: 0

            // Navigation rail
            Rectangle {
                Layout.fillHeight: true
                Layout.preferredWidth: 180
                color: AppTheme.bgAlt
                border.color: AppTheme.border

                Column {
                    anchors.fill: parent
                    anchors.margins: 8
                    spacing: 2
                    Repeater {
                        model: [
                            { "label": "Chat", "glyph": "◎" },
                            { "label": "Discover", "glyph": "⌕" },
                            { "label": "Library", "glyph": "▤" },
                            { "label": "Developer", "glyph": "⌘" },
                            { "label": "Runtimes", "glyph": "⚙" },
                            { "label": "Downloads", "glyph": "↓" },
                            { "label": "Logs", "glyph": "☰" },
                            { "label": "Settings", "glyph": "✦" }
                        ]
                        delegate: NavButton {
                            text: modelData.label
                            glyph: modelData.glyph
                            current: stack.currentIndex === index
                                || (stack.currentIndex === 8 && modelData.label === "Library")
                            onClicked: stack.currentIndex = index
                            Rectangle {
                                visible: modelData.label === "Downloads" && window.downloadCount > 0
                                width: badgeText.implicitWidth + 12
                                height: 18
                                radius: 9
                                color: AppTheme.accent
                                anchors.right: parent.right
                                anchors.rightMargin: 10
                                anchors.verticalCenter: parent.verticalCenter
                                Text {
                                    id: badgeText
                                    anchors.centerIn: parent
                                    text: window.downloadCount
                                    font.pixelSize: AppTheme.fontSmall
                                    color: AppTheme.onAccent
                                }
                            }
                        }
                    }
                }
            }

            // Pages
            StackLayout {
                id: stack
                Layout.fillWidth: true
                Layout.fillHeight: true

                ChatPage      { api: api; events: events; experimentalAudio: window.experimentalAudioModels }
                DiscoverPage  { api: api; events: events; experimentalAudio: window.experimentalAudioModels }
                LibraryPage   { api: api; events: events; experimentalAudio: window.experimentalAudioModels; onOpenDetail: function(modelId) { window.openInstanceDetail(modelId) } }
                DeveloperPage { api: api; events: events }
                RuntimesPage  { api: api; events: events; recommendation: window.recommendation }
                DownloadsPage { api: api; events: events }
                LogsPage      { api: api; events: events }
                SettingsPage  {
                    api: api
                    events: events
                    onSettingChanged: function(key, value) {
                        if (key === "experimental.audio_models")
                            window.experimentalAudioModels = value === "1"
                    }
                }
                InstanceDetailPage {
                    id: instanceDetailPage
                    api: api
                    events: events
                    onBack: stack.currentIndex = 2
                }
            }
        }
    }

    // Instance detail is stack index 8, reached from the Library page.
    function openInstanceDetail(modelId) {
        instanceDetailPage.modelId = modelId
        instanceDetailPage.enter()
        stack.currentIndex = 8
    }

    // Toast overlay
    Column {
        anchors.right: parent.right
        anchors.bottom: parent.bottom
        anchors.margins: 16
        spacing: 8
        z: 100
        Repeater {
            model: toastModel
            delegate: Rectangle {
                width: toastText.implicitWidth + 32
                height: 40
                radius: AppTheme.radius
                color: model.kind === "success" ? AppTheme.success
                     : model.kind === "error" ? AppTheme.danger : AppTheme.surfaceHi
                border.color: AppTheme.border
                Text {
                    id: toastText
                    anchors.centerIn: parent
                    text: model.text
                    color: model.kind === "info" ? AppTheme.text : "white"
                    font.pixelSize: AppTheme.fontBody
                }
                MouseArea { anchors.fill: parent; onClicked: toastModel.remove(index) }
            }
        }
    }

    Component.onCompleted: reloadAll()
}
