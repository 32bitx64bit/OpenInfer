import QtQuick
import QtWebSockets

// Events: authenticated event WebSocket with bounded exponential backoff.
// The first frame after connect carries the session token; the backend
// replies with backend.ready. After any reconnect the app reloads
// authoritative state (handled in Main.qml via reconnected()).
Item {
    id: events

    signal eventReceived(string name, var payload)
    signal reconnected()

    property bool connected: false
    property int _retries: 0

    WebSocket {
        id: ws
        url: wsBase + "/api/v1/events"
        active: true
        onTextMessageReceived: function(message) {
            var env
            try { env = JSON.parse(message) } catch (e) { return }
            if (env.event === "backend.ready") {
                var wasDown = !events.connected
                events.connected = true
                events._retries = 0
                if (wasDown) events.reconnected()
                return
            }
            events.eventReceived(env.event, env.payload)
        }
        onStatusChanged: function(status) {
            if (status === WebSocket.Open) {
                // Authentication handshake must be the first message.
                ws.sendTextMessage(JSON.stringify({ token: apiToken }))
            } else if (status === WebSocket.Closed || status === WebSocket.Error) {
                events.connected = false
                reconnectTimer.restart()
            }
        }
    }

    Timer {
        id: reconnectTimer
        // Bounded exponential backoff: 1s, 2s, 4s, ... capped at 30s.
        interval: Math.min(30000, 1000 * Math.pow(2, events._retries))
        onTriggered: {
            events._retries = Math.min(events._retries + 1, 5)
            ws.active = false
            ws.active = true
        }
    }
}
