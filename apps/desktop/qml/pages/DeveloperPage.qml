import QtQuick
import QtQuick.Controls
import QtQuick.Layouts
import ".."
import "../components"

Item {
    id: page
    property var api
    property var events

    property bool running: false
    property var config: null
    property var requests: []
    property var clients: []
    property bool keyVisible: false
    property string errorText: ""

    function reload() {
        api.get("/api/v1/server", function(st, data) {
            if (st === 200) {
                page.running = data.running
                page.config = data.config
                page.clients = data.clients || []
            }
        })
        api.get("/api/v1/server/requests", function(st, data) {
            if (st === 200) page.requests = (data && data.requests) || []
        })
    }

    function baseUrl() {
        if (!page.config) return ""
        return "http://" + page.config.bind + ":" + page.config.port
    }

    ScrollView {
        anchors.fill: parent
        anchors.margins: AppTheme.pad
        clip: true
        ColumnLayout {
            width: page.width - AppTheme.pad * 2
            spacing: AppTheme.gap * 1.5

            Card {
                Layout.fillWidth: true
                implicitHeight: srvCol.implicitHeight + 24
                ColumnLayout {
                    id: srvCol
                    anchors.fill: parent
                    anchors.margins: 12
                    spacing: 8
                    RowLayout {
                        StatusDot { state: page.running ? "ready" : "" }
                        Label {
                            text: page.running ? "Server running" : "Server stopped"
                            color: AppTheme.text
                            font.weight: Font.DemiBold
                        }
                        Item { Layout.fillWidth: true }
                        Button {
                            text: page.running ? "Stop" : "Start"
                            highlighted: !page.running
                            onClicked: page.api.post("/api/v1/server/" + (page.running ? "stop" : "start"), {},
                                function(st, data) {
                                    if (st !== 200) page.errorText = (data && (data.detail || data.error)) || "failed"
                                    page.reload()
                                })
                        }
                    }
                    RowLayout {
                        spacing: 8
                        Label { text: "Base URL:"; color: AppTheme.textDim }
                        Label {
                            text: page.baseUrl() + "/v1"
                            color: AppTheme.accent
                            font.family: "monospace"
                        }
                        Button {
                            text: "Copy"
                            flat: true
                            onClicked: { clip.text = page.baseUrl() + "/v1"; clip.selectAll(); clip.copy() }
                        }
                    }
                    RowLayout {
                        spacing: 8
                        Label { text: "API key:"; color: AppTheme.textDim }
                        Label {
                            text: page.config
                                ? (page.keyVisible ? page.config.api_key : "••••••••••••••••••••")
                                : ""
                            color: AppTheme.text
                            font.family: "monospace"
                        }
                        Button { text: page.keyVisible ? "Hide" : "Reveal"; flat: true; onClicked: page.keyVisible = !page.keyVisible }
                        Button {
                            text: "Copy"; flat: true
                            onClicked: { clip.text = page.config ? page.config.api_key : ""; clip.selectAll(); clip.copy() }
                        }
                        Button {
                            text: "Regenerate"; flat: true
                            onClicked: regenDialog.open()
                        }
                    }
                    Label {
                        visible: page.clients.length > 0
                        text: "Recent clients: " + page.clients.join(", ")
                        color: AppTheme.textFaint
                        font.pixelSize: AppTheme.fontSmall
                    }
                }
            }

            Card {
                Layout.fillWidth: true
                implicitHeight: cfgCol.implicitHeight + 24
                ColumnLayout {
                    id: cfgCol
                    anchors.fill: parent
                    anchors.margins: 12
                    spacing: 8
                    Label { text: "Configuration"; color: AppTheme.text; font.weight: Font.DemiBold }

                    FormField {
                        Layout.fillWidth: true
                        label: "Port"; hint: "Local port for the OpenAI-compatible API."
                        SpinBox {
                            id: portSpin
                            from: 1024; to: 65535; editable: true
                            value: page.config ? page.config.port : 1235
                        }
                    }
                    FormField {
                        Layout.fillWidth: true
                        label: "LAN access"
                        hint: "Warning: exposes loaded models to your network. The API key is still required. Off = loopback only."
                        Switch {
                            id: lanSwitch
                            checked: page.config ? page.config.allow_lan : false
                        }
                    }
                    Label {
                        visible: lanSwitch.checked
                        Layout.fillWidth: true
                        text: "⚠ LAN access exposes inference to other machines on your network."
                        color: AppTheme.warning
                        wrapMode: Text.WordWrap
                    }
                    FormField {
                        Layout.fillWidth: true
                        label: "CORS origins"
                        hint: "Comma-separated origins, empty = CORS disabled (recommended)."
                        TextField { id: corsField; width: 320; text: page.config ? page.config.cors : "" }
                    }
                    FormField {
                        Layout.fillWidth: true
                        label: "Start automatically"
                        hint: "Start the API when OpenInfer Studio launches."
                        Switch { id: autoSwitch; checked: page.config ? page.config.autostart : false }
                    }
                    Button {
                        text: "Save configuration"
                        onClicked: page.api.put("/api/v1/server", {
                            "port": portSpin.value,
                            "bind": lanSwitch.checked ? "0.0.0.0" : "127.0.0.1",
                            "allow_lan": lanSwitch.checked,
                            "cors": corsField.text,
                            "autostart": autoSwitch.checked
                        }, function(st, data) { page.reload() })
                    }
                    Label {
                        visible: page.errorText !== ""
                        text: page.errorText
                        color: AppTheme.danger
                        wrapMode: Text.WordWrap
                        Layout.fillWidth: true
                    }
                }
            }

            GroupBox {
                Layout.fillWidth: true
                title: "Connection example"
                ScrollView {
                    anchors.fill: parent
                    implicitHeight: 130
                    clip: true
                    TextEdit {
                        readOnly: true
                        width: parent.width
                        color: AppTheme.textDim
                        font.family: "monospace"
                        font.pixelSize: AppTheme.fontSmall
                        text: page.config ? ("curl " + page.baseUrl() + "/v1/chat/completions \\\n"
                            + "  -H \"Authorization: Bearer " + page.config.api_key.substring(0, 10) + "…\" \\\n"
                            + "  -H \"Content-Type: application/json\" \\\n"
                            + "  -d '{\"model\": \"<model-alias>\", \"messages\": [{\"role\":\"user\",\"content\":\"Hello\"}]}'") : ""
                    }
                }
            }

            GroupBox {
                Layout.fillWidth: true
                Layout.preferredHeight: 220
                title: "Recent requests"
                ListView {
                    anchors.fill: parent
                    clip: true
                    model: page.requests
                    delegate: RowLayout {
                        width: ListView.view.width
                        spacing: 10
                        Label { text: modelData.time.substring(11, 19); color: AppTheme.textFaint; font.pixelSize: AppTheme.fontSmall }
                        Label { text: modelData.method + " " + modelData.path; color: AppTheme.textDim; font.pixelSize: AppTheme.fontSmall; font.family: "monospace" }
                        Label { text: modelData.model; color: AppTheme.accent; font.pixelSize: AppTheme.fontSmall }
                        Item { Layout.fillWidth: true }
                        Label {
                            text: modelData.status
                            color: modelData.status < 400 ? AppTheme.success : AppTheme.danger
                            font.pixelSize: AppTheme.fontSmall
                        }
                        Label { text: modelData.duration_ms + "ms"; color: AppTheme.textFaint; font.pixelSize: AppTheme.fontSmall }
                    }
                }
            }
        }
    }

    ConfirmDialog {
        id: regenDialog
        message: "Regenerate the API key? Existing clients will lose access."
        confirmText: "Regenerate"
        destructive: false
        onConfirmed: page.api.post("/api/v1/server/regenerate-key", {}, function() { page.reload() })
    }

    TextEdit { id: clip; visible: false }

    Timer {
        interval: 4000
        running: page.running
        repeat: true
        onTriggered: page.reload()
    }

    Component.onCompleted: reload()
}
