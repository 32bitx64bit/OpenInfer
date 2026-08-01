import QtQuick
import QtQuick.Controls
import QtQuick.Layouts
import Qt.labs.settings
import ".."
import "../components"

// Model load configuration: one scrollable page. Advanced/Expert sections
// collapse behind persisted toggles. Memory estimate updates live; the
// generated command preview sits at the very end.
Dialog {
    id: root
    property var api
    property string modelId: ""
    property var model: null
    property var runtimes: []
    property var presets: []

    signal loaded()

    title: model ? ("Load — " + (model.alias || model.id)) : "Load model"
    modal: true
    width: Math.min(680, parent ? parent.width - 64 : 680)
    height: Math.min(720, parent ? parent.height - 48 : 720)
    anchors.centerIn: parent

    // Persisted expand/collapse state.
    Settings {
        category: "OpenInferStudio/LoadDialog"
        property alias advancedOpen: advancedToggle.checked
        property alias expertOpen: expertToggle.checked
    }

    property var settings: ({
        "context_length": 4096, "gpu_offload": "all", "gpu_layers": 0,
        "threads": 0, "flash_attention": "auto", "parallel": 0,
        "batch_size": 0, "ubatch_size": 0, "cache_type_k": "", "cache_type_v": "",
        "no_mmap": false, "mlock": false, "main_gpu": -1, "split_mode": "",
        "rope_scaling": "", "alias": "", "raw_args": "",
        "save_on_success": true
    })
    property string selectedRuntime: ""   // "" = auto
    property var preview: null
    property var estimate: null
    property string loadError: ""

    function openFor(m) {
        model = m
        modelId = m.id
        settings["alias"] = m.alias || ""
        if (!settings["context_length"] || settings["context_length"] <= 0)
            settings["context_length"] = 4096
        selectedRuntime = m.pinned_runtime || ""
        loadError = ""
        preview = null
        estimate = null
        api.get("/api/v1/runtimes", function(st, data) {
            if (st === 200) root.runtimes = (data && data.runtimes) || []
        })
        api.get("/api/v1/models/" + m.id + "/presets", function(st, data) {
            if (st !== 200) return
            root.presets = (data && data.presets) || []
            // Prefill from last-known-good, else the default preset.
            var applied = false
            for (var i = 0; i < root.presets.length; i++) {
                if (root.presets[i].name === "Last known good") {
                    root.applyPreset(root.presets[i])
                    applied = true
                    break
                }
            }
            if (!applied) {
                for (var j = 0; j < root.presets.length; j++) {
                    if (root.presets[j].is_default) { root.applyPreset(root.presets[j]); break }
                }
            }
        })
        open()
        scheduleRefresh()
    }

    // Debounced refresh of preview + estimate on any settings change.
    Timer {
        id: refreshTimer
        interval: 350
        onTriggered: root.refreshNow()
    }
    function scheduleRefresh() { refreshTimer.restart() }

    function refreshNow() {
        if (!modelId) return
        api.post("/api/v1/models/" + modelId + "/preview", settings, function(st, data) {
            if (st === 200) root.preview = data
        })
        api.post("/api/v1/models/" + modelId + "/estimate", settings, function(st, data) {
            if (st === 200) root.estimate = data
        })
    }

    function setSetting(key, value) {
        settings[key] = value
        settingsChanged()
        scheduleRefresh()
    }

    function applyPreset(p) {
        try {
            var s = JSON.parse(JSON.stringify(p.settings))
            for (var k in s) root.settings[k] = s[k]
            settingsChanged()
            scheduleRefresh()
        } catch (e) {}
    }

    function maxCtx() {
        var mc = root.model ? root.model.context_length : 0
        if (mc <= 0) mc = 131072
        return Math.min(mc, 1048576)
    }

    function maxLayers() {
        var md = root.model ? root.model.metadata : null
        var n = md && md.block_count ? md.block_count : 0
        return n > 0 ? n : 99
    }

    contentItem: ColumnLayout {
        spacing: 0

        ScrollView {
            Layout.fillWidth: true
            Layout.fillHeight: true
            Layout.margins: AppTheme.pad
            clip: true
            ScrollBar.vertical.policy: ScrollBar.AsNeeded

            ColumnLayout {
                width: root.width - AppTheme.pad * 2 - 60
                spacing: AppTheme.gap

                // Corrupt-file warning
                Label {
                    Layout.fillWidth: true
                    visible: root.model && root.model.metadata && root.model.metadata.tensor_errors
                        && root.model.metadata.tensor_errors.length > 0
                    text: "⚠ This model file failed integrity validation: "
                        + (root.model ? (root.model.metadata.tensor_errors || []).join("; ") : "")
                        + "\nLoading will very likely fail. Re-download or pick another quantization."
                    color: AppTheme.danger
                    wrapMode: Text.WordWrap
                }

                // Memory estimate
                Card {
                    Layout.fillWidth: true
                    implicitHeight: estCol.implicitHeight + 20
                    ColumnLayout {
                        id: estCol
                        anchors.fill: parent
                        anchors.margins: 10
                        spacing: 2
                        RowLayout {
                            Label {
                                text: "Estimated memory"
                                color: AppTheme.textDim
                                font.pixelSize: AppTheme.fontSmall
                            }
                            Item { Layout.fillWidth: true }
                            Label {
                                text: root.estimate
                                    ? AppTheme.bytes(root.estimate.total_bytes) + " / "
                                      + AppTheme.bytes(root.estimate.budget_bytes) + " " + root.estimate.budget_kind
                                    : "…"
                                color: root.estimate ? (root.estimate.fits ? AppTheme.success : AppTheme.danger) : AppTheme.textDim
                                font.weight: Font.DemiBold
                            }
                        }
                        ProgressBar {
                            Layout.fillWidth: true
                            from: 0; to: 1
                            value: root.estimate && root.estimate.budget_bytes > 0
                                ? Math.min(1, root.estimate.total_bytes / root.estimate.budget_bytes) : 0
                        }
                        Label {
                            Layout.fillWidth: true
                            text: root.estimate
                                ? "weights " + AppTheme.bytes(root.estimate.weights_bytes)
                                  + " · KV cache " + AppTheme.bytes(root.estimate.kv_cache_bytes)
                                  + " · compute " + AppTheme.bytes(root.estimate.compute_bytes)
                                  + (root.estimate.note !== "" ? "  —  " + root.estimate.note : "")
                                : ""
                            color: AppTheme.textFaint
                            font.pixelSize: AppTheme.fontSmall
                            wrapMode: Text.WordWrap
                        }
                        Label {
                            visible: root.estimate && !root.estimate.fits
                            Layout.fillWidth: true
                            text: "Likely exceeds available memory — reduce context, KV type, or GPU offload."
                            color: AppTheme.warning
                            font.pixelSize: AppTheme.fontSmall
                            wrapMode: Text.WordWrap
                        }
                    }
                }

                // Presets
                RowLayout {
                    Layout.fillWidth: true
                    visible: root.presets.length > 0
                    Label { text: "Preset:"; color: AppTheme.textDim }
                    ComboBox {
                        Layout.fillWidth: true
                        model: root.presets
                        textRole: "name"
                        onActivated: function(i) { root.applyPreset(root.presets[i]) }
                    }
                }

                FormField {
                    Layout.fillWidth: true
                    label: "Runtime"
                    hint: "llama.cpp build used for this model. Auto uses the pinned or preferred runtime."
                    ComboBox {
                        width: parent.width
                        model: [{ "id": "", "build": "Auto (preferred)", "backend": "", "architecture": "" }]
                            .concat(root.runtimes)
                        textRole: "build"
                        currentIndex: 0
                        onActivated: function(i) {
                            root.selectedRuntime = model[i].id || ""
                            root.scheduleRefresh()
                        }
                        delegate: ItemDelegate {
                            text: modelData.id === "" ? modelData.build
                                : modelData.build + " · " + modelData.backend + " · " + modelData.architecture
                            width: parent.width
                        }
                    }
                }

                FormField {
                    Layout.fillWidth: true
                    label: "Context length"
                    hint: "Tokens of context. KV-cache memory grows linearly with this."
                    argName: "--ctx-size"
                    ColumnLayout {
                        width: parent.width
                        spacing: 4
                        RowLayout {
                            width: parent.width
                            spacing: 12
                            Slider {
                                id: ctxSlider
                                Layout.fillWidth: true
                                from: 512
                                to: root.maxCtx()
                                stepSize: 512
                                value: root.settings.context_length
                                onMoved: root.setSetting("context_length", Math.round(value))
                            }
                            SpinBox {
                                from: 512
                                to: 1048576
                                stepSize: 512
                                editable: true
                                value: root.settings.context_length
                                onValueModified: root.setSetting("context_length", value)
                            }
                        }
                        Label {
                            text: "model maximum: " + (root.model && root.model.context_length > 0
                                  ? root.model.context_length : "unknown")
                            color: AppTheme.textFaint
                            font.pixelSize: AppTheme.fontSmall
                        }
                    }
                }

                FormField {
                    Layout.fillWidth: true
                    label: "GPU offload"
                    hint: "Layers offloaded to the GPU. All is fastest when it fits; fewer layers spill the rest to system RAM."
                    argName: "--n-gpu-layers"
                    ColumnLayout {
                        width: parent.width
                        spacing: 4
                        Row {
                            spacing: 8
                            ComboBox {
                                id: offloadCombo
                                model: [
                                    { "text": "All layers", "value": "all" },
                                    { "text": "Custom", "value": "custom" },
                                    { "text": "None (CPU only)", "value": "none" }
                                ]
                                textRole: "text"
                                currentIndex: root.settings.gpu_offload === "custom" ? 1
                                    : root.settings.gpu_offload === "none" ? 2 : 0
                                onActivated: function(i) {
                                    root.setSetting("gpu_offload", model[i].value)
                                    if (model[i].value === "custom" && root.settings.gpu_layers <= 0)
                                        root.setSetting("gpu_layers", root.maxLayers())
                                }
                            }
                            Label {
                                visible: root.settings.gpu_offload === "custom"
                                text: root.settings.gpu_layers + " / " + root.maxLayers()
                                    + (root.settings.gpu_layers >= root.maxLayers() ? " (all)" : "")
                                color: AppTheme.textDim
                                anchors.verticalCenter: parent.verticalCenter
                            }
                        }
                        RowLayout {
                            width: parent.width
                            spacing: 12
                            visible: root.settings.gpu_offload === "custom"
                            Slider {
                                Layout.fillWidth: true
                                from: 0
                                to: root.maxLayers()
                                stepSize: 1
                                value: root.settings.gpu_layers
                                onMoved: root.setSetting("gpu_layers", Math.round(value))
                            }
                            SpinBox {
                                from: 0
                                to: root.maxLayers()
                                editable: true
                                value: root.settings.gpu_layers
                                onValueModified: root.setSetting("gpu_layers", value)
                            }
                        }
                        Label {
                            visible: root.settings.gpu_offload === "custom"
                            text: "model has " + root.maxLayers() + " layers"
                            color: AppTheme.textFaint
                            font.pixelSize: AppTheme.fontSmall
                        }
                    }
                }

                FormField {
                    Layout.fillWidth: true
                    label: "CPU threads"
                    hint: "0 = automatic."
                    argName: "--threads"
                    SpinBox {
                        from: 0; to: 1024; editable: true
                        value: root.settings.threads
                        onValueModified: root.setSetting("threads", value)
                    }
                }

                FormField {
                    Layout.fillWidth: true
                    label: "Flash Attention"
                    hint: "Reduces KV-cache memory. Auto = enabled."
                    argName: "--flash-attn"
                    ComboBox {
                        model: ["auto", "on", "off"]
                        currentIndex: Math.max(0, model.indexOf(root.settings.flash_attention))
                        onActivated: function(i) { root.setSetting("flash_attention", model[i]) }
                    }
                }

                FormField {
                    Layout.fillWidth: true
                    label: "Parallel slots"
                    hint: "Simultaneous requests. Each slot gets the full context length; KV memory = context × slots."
                    argName: "--parallel"
                    SpinBox {
                        from: 0; to: 64; editable: true
                        value: root.settings.parallel
                        onValueModified: root.setSetting("parallel", value)
                    }
                }

                FormField {
                    Layout.fillWidth: true
                    label: "Remember on success"
                    hint: "Save as this model's default after a successful load. Failed loads keep the previous default."
                    Switch {
                        checked: root.settings.save_on_success
                        onToggled: root.setSetting("save_on_success", checked)
                    }
                }

                FormField {
                    Layout.fillWidth: true
                    label: "Multimodal projector"
                    hint: root.model && root.model.projector_path !== ""
                          ? root.model.projector_path : "No projector paired with this model."
                    argName: "--mmproj"
                    supported: root.model ? root.model.projector_path !== "" : false
                    Label {
                        text: root.model && root.model.projector_path !== "" ? "Paired automatically" : "None"
                        color: AppTheme.textDim
                    }
                }

                ToolButton {
                    id: advancedToggle
                    checkable: true
                    text: (checked ? "▾" : "▸") + " Advanced"
                    font.weight: Font.DemiBold
                }
                ColumnLayout {
                    Layout.fillWidth: true
                    visible: advancedToggle.checked
                    spacing: AppTheme.gap

                    FormField {
                        Layout.fillWidth: true
                        label: "Batch size"; argName: "--batch-size"
                        hint: "Prompt processing batch. 0 = runtime default."
                        SpinBox { from: 0; to: 65536; editable: true; value: root.settings.batch_size
                            onValueModified: root.setSetting("batch_size", value) }
                    }
                    FormField {
                        Layout.fillWidth: true
                        label: "Micro-batch size"; argName: "--ubatch-size"
                        hint: "Physical batch. 0 = runtime default."
                        SpinBox { from: 0; to: 65536; editable: true; value: root.settings.ubatch_size
                            onValueModified: root.setSetting("ubatch_size", value) }
                    }
                    FormField {
                        Layout.fillWidth: true
                        label: "KV cache type K"; argName: "--cache-type-k"
                        hint: "Quantizing the K cache saves memory."
                        ComboBox { model: ["", "f16", "bf16", "q8_0", "q4_0", "q4_1", "iq4_nl", "q5_0", "q5_1"]
                            currentIndex: Math.max(0, model.indexOf(root.settings.cache_type_k))
                            onActivated: function(i) { root.setSetting("cache_type_k", model[i]) } }
                    }
                    FormField {
                        Layout.fillWidth: true
                        label: "KV cache type V"; argName: "--cache-type-v"
                        hint: "Quantizing the V cache saves memory."
                        ComboBox { model: ["", "f16", "bf16", "q8_0", "q4_0", "q4_1", "iq4_nl", "q5_0", "q5_1"]
                            currentIndex: Math.max(0, model.indexOf(root.settings.cache_type_v))
                            onActivated: function(i) { root.setSetting("cache_type_v", model[i]) } }
                    }
                    FormField {
                        Layout.fillWidth: true
                        label: "Memory mapping"; argName: "--no-mmap"
                        hint: "Disable to load the whole model into RAM up front."
                        Switch { checked: !root.settings.no_mmap
                            onToggled: root.setSetting("no_mmap", !checked) }
                    }
                    FormField {
                        Layout.fillWidth: true
                        label: "Memory locking"; argName: "--mlock"
                        hint: "Keep the model resident (prevents swapping)."
                        Switch { checked: root.settings.mlock
                            onToggled: root.setSetting("mlock", checked) }
                    }
                    FormField {
                        Layout.fillWidth: true
                        label: "Model alias"; argName: "--alias"
                        hint: "Name exposed through the API."
                        TextField { width: 320; text: root.settings.alias
                            onEditingFinished: root.setSetting("alias", text) }
                    }
                }

                ToolButton {
                    id: expertToggle
                    checkable: true
                    text: (checked ? "▾" : "▸") + " Expert"
                    font.weight: Font.DemiBold
                }
                ColumnLayout {
                    Layout.fillWidth: true
                    visible: expertToggle.checked
                    spacing: AppTheme.gap

                    FormField {
                        Layout.fillWidth: true
                        label: "Raw llama.cpp arguments"
                        hint: "Space-separated flags supported by the selected runtime. Unsafe input is rejected."
                        TextArea {
                            width: parent.width
                            height: 72
                            text: root.settings.raw_args
                            placeholderText: "--override-kv llama.attention.head_count=int:8"
                            onEditingFinished: root.setSetting("raw_args", text)
                        }
                    }
                    Label {
                        text: "Environment overrides are restricted to an allowlist (GGML_*, CUDA_VISIBLE_DEVICES, …)."
                        color: AppTheme.textFaint
                        font.pixelSize: AppTheme.fontSmall
                        wrapMode: Text.WordWrap
                        Layout.fillWidth: true
                    }
                }

                GroupBox {
                    Layout.fillWidth: true
                    title: "Generated command"
                    ColumnLayout {
                        width: parent.width
                        Repeater {
                            model: root.preview ? root.preview.resolutions || [] : []
                            Label {
                                text: modelData.setting + ": " + modelData.auto + " → " + modelData.resolved
                                color: AppTheme.textDim
                                font.pixelSize: AppTheme.fontSmall
                            }
                        }
                        Repeater {
                            model: root.preview ? root.preview.warnings || [] : []
                            Label {
                                text: "⚠ " + modelData
                                color: AppTheme.warning
                                font.pixelSize: AppTheme.fontSmall
                                wrapMode: Text.WordWrap
                                Layout.fillWidth: true
                            }
                        }
                        ScrollView {
                            Layout.fillWidth: true
                            Layout.preferredHeight: 56
                            clip: true
                            TextEdit {
                                width: parent.width
                                readOnly: true
                                text: root.preview ? "llama-server " + root.preview.command : "…"
                                color: AppTheme.text
                                font.family: "monospace"
                                font.pixelSize: AppTheme.fontSmall
                                wrapMode: Text.WrapAnywhere
                            }
                        }
                    }
                }
            }
        }

        Rectangle {
            Layout.fillWidth: true
            Layout.preferredHeight: footerRow.implicitHeight + 16
            color: AppTheme.bgAlt
            ColumnLayout {
                id: footerRow
                anchors.fill: parent
                anchors.margins: 8
                spacing: 6
                Label {
                    visible: root.loadError !== ""
                    Layout.fillWidth: true
                    text: root.loadError
                    color: AppTheme.danger
                    wrapMode: Text.WordWrap
                }
                RowLayout {
                    Layout.fillWidth: true
                    Button {
                        text: "Save preset…"
                        flat: true
                        onClicked: savePresetDialog.open()
                    }
                    Item { Layout.fillWidth: true }
                    Button { text: "Cancel"; onClicked: root.close() }
                    Button {
                        text: "Load model"
                        highlighted: true
                        enabled: root.estimate ? root.estimate.fits : true
                        ToolTip.visible: hovered && root.estimate && !root.estimate.fits
                        ToolTip.text: "Estimated to exceed available memory"
                        onClicked: {
                            root.loadError = ""
                            root.api.post("/api/v1/models/" + root.modelId + "/load", root.settings,
                                function(st, data) {
                                    if (st === 202) {
                                        root.loaded()
                                        root.close()
                                    } else {
                                        root.loadError = (data && (data.detail || data.error)) || ("HTTP " + st)
                                    }
                                })
                        }
                    }
                }
            }
        }
    }

    Dialog {
        id: savePresetDialog
        title: "Save preset"
        modal: true
        anchors.centerIn: root.parent
        standardButtons: Dialog.Save | Dialog.Cancel
        Column {
            spacing: 8
            TextField { id: presetNameField; placeholderText: "Preset name"; width: 280 }
            CheckBox { id: presetDefault; text: "Make default" }
        }
        onAccepted: {
            root.api.post("/api/v1/models/" + root.modelId + "/presets",
                { "name": presetNameField.text, "settings": root.settings, "is_default": presetDefault.checked },
                function(st, data) {
                    if (st === 200) {
                        root.api.get("/api/v1/models/" + root.modelId + "/presets", function(s2, d2) {
                            if (s2 === 200) root.presets = (d2 && d2.presets) || []
                        })
                    }
                })
        }
    }
}
