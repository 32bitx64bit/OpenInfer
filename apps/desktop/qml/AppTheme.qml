pragma Singleton
import QtQuick

// OpenInfer Studio theme: original dark-first identity with light and system
// modes. Restrained radii, dense information layout, strong focus contrast.
QtObject {
    // mode: "system" | "dark" | "light"
    property string mode: "system"
    readonly property bool dark: mode === "dark"
        || (mode === "system" && Qt.styleHints.colorScheme === Qt.ColorScheme.Dark)

    // Surfaces
    readonly property color bg:        dark ? "#101418" : "#f4f6f8"
    readonly property color bgAlt:     dark ? "#161b21" : "#e9edf1"
    readonly property color surface:   dark ? "#1c232b" : "#ffffff"
    readonly property color surfaceHi: dark ? "#242d37" : "#f0f3f6"
    readonly property color border:    dark ? "#2e3944" : "#d3dbe2"
    readonly property color overlay:   dark ? "#cc000000" : "#66000000"

    // Text
    readonly property color text:      dark ? "#e6edf3" : "#1a2330"
    readonly property color textDim:   dark ? "#9aa8b5" : "#55677a"
    readonly property color textFaint: dark ? "#67727f" : "#8fa0b0"

    // Accents (original OpenInfer identity: teal/cyan on dark)
    readonly property color accent:    dark ? "#35c4b5" : "#0d8a7d"
    readonly property color accentHi:  dark ? "#4dd8c8" : "#0a6e64"
    readonly property color onAccent:  dark ? "#06251f" : "#ffffff"

    // Status
    readonly property color success:   dark ? "#4cc38a" : "#18794e"
    readonly property color warning:   dark ? "#e5b45a" : "#a06a00"
    readonly property color danger:    dark ? "#e06c75" : "#c0392b"
    readonly property color info:      dark ? "#6aa8e0" : "#2266aa"

    // Metrics
    readonly property int radius: 8
    readonly property int radiusSmall: 5
    readonly property int gap: 12
    readonly property int pad: 16

    readonly property int fontBody: 13
    readonly property int fontSmall: 11
    readonly property int fontTitle: 16
    readonly property int fontHero: 20

    function stateColor(state) {
        switch (state) {
        case "ready": case "complete": case "busy": return success
        case "loading": case "starting": case "active": case "queued": return info
        case "sleeping": case "paused": return warning
        case "failed": case "crashed": return danger
        default: return textFaint
        }
    }

    function bytes(n) {
        if (n === undefined || n === null) return "—"
        const units = ["B", "KiB", "MiB", "GiB", "TiB"]
        let v = n, i = 0
        while (v >= 1024 && i < units.length - 1) { v /= 1024; i++ }
        return (i === 0 ? v : v.toFixed(1)) + " " + units[i]
    }

    function tokensPerSec(v) {
        return v ? v.toFixed(1) + " tok/s" : "—"
    }
}
