import QtQuick
import QtQuick.Controls
import QtQuick.Layouts
import ".."
import "../components"

Item {
    id: page
    property var api
    property var events

    property var settings: ({})
    property var hardware: null
    property var recommendation: null
    property var directories: []
    property bool hfConfigured: false
    property string statusText: ""

    function reload() {
        api.get("/api/v1/settings", function(st, data) {
            if (st === 200) {
                page.settings = data || {}
                var theme = page.settings["ui.theme"] || "system"
                AppTheme.mode = theme
            }
        })
        api.get("/api/v1/hardware", function(st, data) {
            if (st === 200) {
                page.hardware = data.hardware
                page.recommendation = data.recommendation
            }
        })
        api.get("/api/v1/directories", function(st, data) {
            if (st === 200) page.directories = (data && data.directories) || []
        })
        api.get("/api/v1/hf/token", function(st, data) {
            if (st === 200) page.hfConfigured = data && data.configured
        })
    }

    function setSetting(key, value) {
        api.put("/api/v1/settings/" + key, { "value": String(value) }, function(st, data) {
            if (st === 200) {
                page.statusText = "Saved."
                statusClear.restart()
            }
        })
    }
    Timer { id: statusClear; interval: 2500; onTriggered: page.statusText = "" }

    ScrollView {
        anchors.fill: parent
        anchors.margins: AppTheme.pad
        clip: true
        ColumnLayout {
            width: page.width - AppTheme.pad * 2
            spacing: AppTheme.gap * 1.5

            RowLayout {
                Label { text: "Settings"; font.pixelSize: AppTheme.fontHero; font.weight: Font.DemiBold; color: AppTheme.text }
                Item { Layout.fillWidth: true }
                Label { text: page.statusText; color: AppTheme.success }
            }

            // Appearance
            GroupBox {
                Layout.fillWidth: true
                title: "Appearance"
                FormField {
                    width: parent.width
                    label: "Theme"
                    hint: "System follows the OS appearance."
                    ComboBox {
                        model: ["system", "dark", "light"]
                        currentIndex: Math.max(0, model.indexOf(page.settings["ui.theme"] || "system"))
                        onActivated: function(i) {
                            AppTheme.mode = model[i]
                            page.setSetting("ui.theme", model[i])
                        }
                    }
                }
            }

            // Hardware
            GroupBox {
                Layout.fillWidth: true
                title: "Hardware"
                ColumnLayout {
                    width: parent.width
                    spacing: 6
                    visible: page.hardware !== null
                    GridLayout {
                        columns: 2
                        columnSpacing: 20
                        rowSpacing: 4
                        width: parent.width
                        Repeater {
                            model: page.hardware ? [
                                ["OS", page.hardware.os + " " + (page.hardware.os_version || "")],
                                ["CPU", page.hardware.cpu_model + " (" + page.hardware.logical_cores + " threads)"],
                                ["Features", (page.hardware.cpu_features || []).join(", ")],
                                ["RAM", AppTheme.bytes(page.hardware.ram_total) + " total, " + AppTheme.bytes(page.hardware.ram_available) + " available"],
                                ["GPUs", (page.hardware.gpus || []).map(function(g) { return g.name }).join("; ") || "none detected"],
                                ["Vulkan / CUDA / HIP / Metal", [page.hardware.vulkan ? "Vulkan" : "", page.hardware.cuda ? "CUDA" : "", page.hardware.hip ? "HIP" : "", page.hardware.metal ? "Metal" : ""].filter(function(s){return s!==""}).join(" · ") || "none"],
                                ["Free disk (models)", AppTheme.bytes(page.hardware.disk_free_models)],
                                ["Free disk (runtimes)", AppTheme.bytes(page.hardware.disk_free_runtimes)]
                            ] : []
                            delegate: Row {
                                spacing: 8
                                Label { text: modelData[0]; color: AppTheme.textFaint; font.pixelSize: AppTheme.fontSmall; width: 160 }
                                Label { text: modelData[1]; color: AppTheme.textDim; font.pixelSize: AppTheme.fontSmall }
                            }
                        }
                    }
                    Label {
                        visible: page.recommendation !== null
                        Layout.fillWidth: true
                        text: page.recommendation ? "Recommended backend: " + page.recommendation.backend.toUpperCase()
                            + " — " + page.recommendation.reason : ""
                        color: AppTheme.accent
                        wrapMode: Text.WordWrap
                    }
                    RowLayout {
                        Button {
                            text: "Refresh"
                            onClicked: page.api.get("/api/v1/hardware?refresh=1", function(st, data) {
                                if (st === 200) { page.hardware = data.hardware; page.recommendation = data.recommendation }
                            })
                        }
                        Button {
                            text: "Copy report"
                            flat: true
                            onClicked: {
                                clip.text = JSON.stringify({ "hardware": page.hardware, "recommendation": page.recommendation }, null, 2)
                                clip.selectAll(); clip.copy()
                            }
                        }
                    }
                }
            }

            // Hugging Face token
            GroupBox {
                Layout.fillWidth: true
                title: "Hugging Face access"
                ColumnLayout {
                    width: parent.width
                    spacing: 6
                    Label {
                        Layout.fillWidth: true
                        text: "A token is only needed for gated or private repositories. It is stored in the operating system credential vault, never in plain text."
                        color: AppTheme.textDim
                        wrapMode: Text.WordWrap
                    }
                    RowLayout {
                        spacing: 8
                        TextField {
                            id: tokenField
                            Layout.fillWidth: true
                            echoMode: TextInput.Password
                            placeholderText: page.hfConfigured ? "Token configured (enter to replace)" : "hf_…"
                        }
                        Button {
                            text: "Save token"
                            onClicked: page.api.put("/api/v1/hf/token", { "token": tokenField.text }, function(st) {
                                if (st === 200) { page.hfConfigured = true; tokenField.text = "" }
                            })
                        }
                        Button {
                            text: "Remove"
                            visible: page.hfConfigured
                            onClicked: page.api.del("/api/v1/hf/token", function() { page.hfConfigured = false })
                        }
                    }
                }
            }

            // Limits
            GroupBox {
                Layout.fillWidth: true
                title: "Behavior"
                GridLayout {
                    columns: 2
                    width: parent.width
                    columnSpacing: 20
                    FormField {
                        label: "Concurrent downloads"
                        hint: "1–8 simultaneous downloads."
                        SpinBox {
                            from: 1; to: 8
                            value: parseInt(page.settings["downloads.concurrency"] || "2")
                            onValueModified: page.setSetting("downloads.concurrency", value)
                        }
                    }
                    FormField {
                        label: "Max loaded models"
                        hint: "Simultaneously loaded models (1–32)."
                        SpinBox {
                            from: 1; to: 32
                            value: parseInt(page.settings["instances.max_loaded"] || "8")
                            onValueModified: page.setSetting("instances.max_loaded", value)
                        }
                    }
                    FormField {
                        label: "Model startup timeout (s)"
                        hint: "How long to wait for a model to become ready."
                        SpinBox {
                            from: 30; to: 3600; stepSize: 30
                            value: parseInt(page.settings["instances.startup_timeout_sec"] || "600")
                            onValueModified: page.setSetting("instances.startup_timeout_sec", value)
                        }
                    }
                    FormField {
                        label: "Stream responses"
                        hint: "Show tokens as they generate. Disable to wait for the full reply."
                        Switch {
                            checked: (page.settings["chat.streaming"] || "1") !== "0"
                            onToggled: page.setSetting("chat.streaming", checked ? "1" : "0")
                        }
                    }
                    FormField {
                        label: "Runtime update checks"
                        hint: "Check llama.cpp releases when opening the Runtimes page."
                        Switch {
                            checked: (page.settings["runtimes.update_checks"] || "1") === "1"
                            onToggled: page.setSetting("runtimes.update_checks", checked ? "1" : "0")
                        }
                    }
                }
            }

            // Model directories
            GroupBox {
                Layout.fillWidth: true
                title: "Model directories"
                ColumnLayout {
                    width: parent.width
                    spacing: 4
                    Repeater {
                        model: page.directories
                        delegate: RowLayout {
                            Layout.fillWidth: true
                            Label {
                                text: modelData.path + (modelData.managed ? "  (managed)" : "")
                                color: AppTheme.textDim
                                font.family: "monospace"
                                font.pixelSize: AppTheme.fontSmall
                                elide: Text.ElideMiddle
                                Layout.fillWidth: true
                            }
                            Button {
                                visible: !modelData.managed
                                text: "Remove"
                                flat: true
                                onClicked: page.api.del("/api/v1/directories/" + modelData.id, function() { page.reload() })
                            }
                        }
                    }
                    RowLayout {
                        TextField { id: dirField; Layout.fillWidth: true; placeholderText: "/path/to/models" }
                        Button {
                            text: "Add directory"
                            onClicked: page.api.post("/api/v1/directories", { "path": dirField.text }, function(st, data) {
                                if (st !== 201) page.statusText = (data && (data.detail || data.error)) || "failed"
                                dirField.text = ""
                                page.reload()
                            })
                        }
                    }
                }
            }
        }
    }

    TextEdit { id: clip; visible: false }
    Component.onCompleted: reload()
}
