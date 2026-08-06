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
    property string saveNote: ""

    // Local form state — never bind Switch/SpinBox directly to page.config or
    // the 4s status poll will wipe in-progress edits (LAN toggle bug).
    property int formPort: 1235
    property bool formAllowLan: false
    property string formCors: ""
    property bool formAutostart: false
    property bool formDirty: false

    property var hostit: null
    property bool hostitEnabled: false
    property string hostitDomain: "" // "" | "auto"
    property string hostitAgentURL: "http://127.0.0.1:7003"
    property string hostitError: ""
    property bool hostitDirty: false

    function applyConfig(cfg) {
        if (!cfg) return
        page.config = cfg
        if (!page.formDirty) {
            page.formPort = cfg.port || 1235
            page.formAllowLan = !!cfg.allow_lan
            page.formCors = cfg.cors || ""
            page.formAutostart = !!cfg.autostart
            if (portSpin) portSpin.value = page.formPort
            if (lanSwitch) lanSwitch.checked = page.formAllowLan
            if (corsField) corsField.text = page.formCors
            if (autoSwitch) autoSwitch.checked = page.formAutostart
        }
    }

    function applyHostIt(data) {
        if (!data) return
        page.hostit = data
        page.hostitError = data.last_error || data.sync_error || ""
        if (!page.hostitDirty) {
            page.hostitEnabled = !!data.enabled
            page.hostitDomain = data.domain || ""
            page.hostitAgentURL = data.agent_url || "http://127.0.0.1:7003"
            if (hostitSwitch) hostitSwitch.checked = page.hostitEnabled
            if (hostitDomainBox)
                hostitDomainBox.currentIndex = page.hostitDomain === "auto" ? 1 : 0
        }
    }

    function reload() {
        api.get("/api/v1/server", function(st, data) {
            if (st === 200) {
                page.running = data.running
                page.applyConfig(data.config)
                page.clients = data.clients || []
            }
        })
        api.get("/api/v1/server/requests", function(st, data) {
            if (st === 200) page.requests = (data && data.requests) || []
        })
        page.reloadHostIt()
    }

    function reloadStatus() {
        api.get("/api/v1/server", function(st, data) {
            if (st === 200) {
                page.running = data.running
                page.clients = data.clients || []
                // Refresh config only when the form is clean.
                if (!page.formDirty)
                    page.applyConfig(data.config)
            }
        })
        api.get("/api/v1/server/requests", function(st, data) {
            if (st === 200) page.requests = (data && data.requests) || []
        })
        page.reloadHostIt()
    }

    function reloadHostIt() {
        api.get("/api/v1/hostit", function(st, data) {
            if (st !== 200 || !data) return
            page.applyHostIt(data)
        })
    }

    function baseUrl() {
        if (!page.config) return ""
        var bind = page.formAllowLan ? "0.0.0.0" : (page.config.bind || "127.0.0.1")
        // Prefer loopback for display when LAN is off.
        var host = page.formAllowLan ? (page.config.bind === "0.0.0.0" ? "<lan-ip>" : page.config.bind) : "127.0.0.1"
        if (page.formAllowLan) host = "0.0.0.0"
        else host = "127.0.0.1"
        return "http://" + host + ":" + page.formPort
    }

    function publicUrl() {
        if (!page.hostit) return ""
        if (page.hostit.public_domain)
            return "https://" + page.hostit.public_domain + "/v1"
        var addr = page.hostit.public_addr || ""
        // Defensive: combine server root IP with port-only route addr.
        if (addr.indexOf(":") === 0 && page.hostit.server_addr)
            addr = page.hostit.server_addr + addr
        if (!addr || addr.indexOf(":") === 0)
            return ""
        return "http://" + addr + "/v1"
    }

    ScrollView {
        anchors.fill: parent
        anchors.margins: AppTheme.pad
        clip: true
        ColumnLayout {
            width: page.width - AppTheme.pad * 2
            spacing: AppTheme.gap * 1.5

            PageHeader {
                title: "Developer API"
                subtitle: "Expose loaded models through an OpenAI-compatible local endpoint. Configure this only when another app needs access."
            }

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
                        AppButton {
                            text: page.running ? "Stop" : "Start"
                            primary: !page.running
                            onClicked: page.api.post("/api/v1/server/" + (page.running ? "stop" : "start"), {},
                                function(st, data) {
                                    if (st !== 200) page.errorText = (data && (data.detail || data.error)) || "failed"
                                    else if (data && data.hostit_error) page.hostitError = data.hostit_error
                                    page.formDirty = false
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
                        AppButton {
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
                        AppButton { text: page.keyVisible ? "Hide" : "Reveal"; flat: true; onClicked: page.keyVisible = !page.keyVisible }
                        AppButton {
                            text: "Copy"; flat: true
                            onClicked: { clip.text = page.config ? page.config.api_key : ""; clip.selectAll(); clip.copy() }
                        }
                        AppButton {
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
                        AppSpinBox {
                            id: portSpin
                            from: 1024; to: 65535; editable: true
                            value: 1235
                            onValueModified: {
                                page.formPort = value
                                page.formDirty = true
                            }
                        }
                    }
                    FormField {
                        Layout.fillWidth: true
                        label: "LAN access"
                        hint: "Warning: exposes loaded models to your network. The API key is still required. Off = loopback only."
                        AppSwitch {
                            id: lanSwitch
                            onToggled: {
                                page.formAllowLan = checked
                                page.formDirty = true
                            }
                        }
                    }
                    Label {
                        visible: page.formAllowLan
                        Layout.fillWidth: true
                        text: "⚠ LAN access exposes inference to other machines on your network. Restart the server after saving for bind changes to take effect."
                        color: AppTheme.warning
                        wrapMode: Text.WordWrap
                    }
                    FormField {
                        Layout.fillWidth: true
                        label: "CORS origins"
                        hint: "Comma-separated origins, empty = CORS disabled (recommended)."
                        AppTextField {
                            id: corsField
                            width: 320
                            onTextEdited: {
                                page.formCors = text
                                page.formDirty = true
                            }
                        }
                    }
                    FormField {
                        Layout.fillWidth: true
                        label: "Start automatically"
                        hint: "Start the API when OpenInfer Studio launches."
                        AppSwitch {
                            id: autoSwitch
                            onToggled: {
                                page.formAutostart = checked
                                page.formDirty = true
                            }
                        }
                    }
                    RowLayout {
                        AppButton {
                            text: "Save configuration"
                            primary: page.formDirty
                            onClicked: page.api.put("/api/v1/server", {
                                "port": page.formPort,
                                "bind": page.formAllowLan ? "0.0.0.0" : "127.0.0.1",
                                "allow_lan": page.formAllowLan,
                                "cors": page.formCors,
                                "autostart": page.formAutostart
                            }, function(st, data) {
                                if (st !== 200) {
                                    page.errorText = (data && (data.detail || data.error)) || ("HTTP " + st)
                                    return
                                }
                                page.errorText = ""
                                page.formDirty = false
                                if (data && data.config)
                                    page.applyConfig(data.config)
                                page.saveNote = data && data.restart_required
                                    ? "Saved. Restart the server for bind/port changes."
                                    : "Saved."
                                page.reload()
                            })
                        }
                        Label {
                            visible: page.saveNote !== ""
                            text: page.saveNote
                            color: AppTheme.success
                            font.pixelSize: AppTheme.fontSmall
                        }
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

            Card {
                Layout.fillWidth: true
                implicitHeight: hostCol.implicitHeight + 24
                ColumnLayout {
                    id: hostCol
                    anchors.fill: parent
                    anchors.margins: 12
                    spacing: 8
                    Label {
                        text: "HostIt public tunnel"
                        color: AppTheme.text
                        font.weight: Font.DemiBold
                    }
                    Label {
                        Layout.fillWidth: true
                        text: "Register this OpenAI-compatible API with a local HostIt agent so it can be reached over the wider internet. Requires the HostIt agent running (default http://127.0.0.1:7003)."
                        color: AppTheme.textDim
                        wrapMode: Text.WordWrap
                        font.pixelSize: AppTheme.fontSmall
                    }
                    FormField {
                        Layout.fillWidth: true
                        label: "Enable HostIt"
                        hint: "When enabled, starting the public API also registers a HostIt route."
                        AppSwitch {
                            id: hostitSwitch
                            onToggled: {
                                page.hostitEnabled = checked
                                page.hostitDirty = true
                            }
                        }
                    }
                    FormField {
                        Layout.fillWidth: true
                        label: "Domain"
                        hint: "Port-only tunnel, or ask HostIt to assign a domain automatically."
                        AppComboBox {
                            id: hostitDomainBox
                            model: [
                                { "v": "", "label": "Port only (no domain)" },
                                { "v": "auto", "label": "Auto domain" }
                            ]
                            textRole: "label"
                            onActivated: function(i) {
                                page.hostitDomain = model[i].v
                                page.hostitDirty = true
                            }
                        }
                    }
                    RowLayout {
                        Layout.fillWidth: true
                        Label {
                            text: {
                                var a = page.hostit && page.hostit.agent ? page.hostit.agent : null
                                if (!a) return "Agent: unknown"
                                if (!a.reachable) return "Agent: unreachable"
                                return a.connected
                                    ? ("Agent: connected" + (a.version ? (" · v" + a.version) : ""))
                                    : "Agent: reachable but not connected to server"
                            }
                            color: AppTheme.textDim
                            font.pixelSize: AppTheme.fontSmall
                            Layout.fillWidth: true
                        }
                    }
                    RowLayout {
                        visible: page.publicUrl() !== ""
                        Layout.fillWidth: true
                        Label { text: "Public URL:"; color: AppTheme.textDim }
                        Label {
                            text: page.publicUrl()
                            color: AppTheme.accent
                            font.family: "monospace"
                            elide: Text.ElideMiddle
                            Layout.fillWidth: true
                        }
                        AppButton {
                            text: "Copy"
                            flat: true
                            onClicked: { clip.text = page.publicUrl(); clip.selectAll(); clip.copy() }
                        }
                    }
                    Label {
                        visible: page.hostitError !== ""
                        Layout.fillWidth: true
                        text: page.hostitError
                        color: AppTheme.warning
                        wrapMode: Text.WordWrap
                        font.pixelSize: AppTheme.fontSmall
                    }
                    RowLayout {
                        AppButton {
                            text: "Save + sync"
                            primary: true
                            onClicked: page.api.put("/api/v1/hostit", {
                                "enabled": page.hostitEnabled,
                                "agent_url": page.hostitAgentURL,
                                "domain": page.hostitDomain,
                                "route_name": "openinfer-api"
                            }, function(st, data) {
                                if (st !== 200) {
                                    page.hostitError = (data && (data.detail || data.error)) || ("HTTP " + st)
                                    return
                                }
                                page.hostitDirty = false
                                page.applyHostIt(data)
                            })
                        }
                        AppButton {
                            text: "Refresh status"
                            flat: true
                            onClicked: page.reloadHostIt()
                        }
                    }
                }
            }

            AppGroupBox {
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

            AppGroupBox {
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
        onTriggered: page.reloadStatus()
    }

    Component.onCompleted: reload()
}
