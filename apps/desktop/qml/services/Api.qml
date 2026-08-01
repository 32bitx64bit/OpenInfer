import QtQuick

// Api: tiny authenticated REST client for the Go control API.
// All calls are async; callbacks receive (status, data, rawText).
QtObject {
    id: api

    // Last transport-level failure, for the offline banner.
    property string lastError: ""

    function request(method, path, body, cb) {
        var xhr = new XMLHttpRequest()
        xhr.open(method, apiBase + path, true)
        xhr.setRequestHeader("Authorization", "Bearer " + apiToken)
        xhr.setRequestHeader("Content-Type", "application/json")
        xhr.timeout = 15000
        xhr.onreadystatechange = function() {
            if (xhr.readyState !== XMLHttpRequest.DONE) return
            var data = null
            try { data = JSON.parse(xhr.responseText) } catch (e) {}
            if (xhr.status === 0) {
                api.lastError = "backend unreachable"
            } else if (xhr.status >= 400) {
                api.lastError = (data && data.error) ? data.error : ("HTTP " + xhr.status)
            } else {
                api.lastError = ""
            }
            if (cb) cb(xhr.status, data, xhr.responseText)
        }
        xhr.ontimeout = function() {
            api.lastError = "request timed out"
            if (cb) cb(0, null, "")
        }
        xhr.send(body === undefined ? null : JSON.stringify(body))
    }

    function get(path, cb)        { request("GET", path, undefined, cb) }
    function post(path, body, cb) { request("POST", path, body, cb) }
    function put(path, body, cb)  { request("PUT", path, body, cb) }
    function patch(path, body, cb){ request("PATCH", path, body, cb) }
    function del(path, cb)        { request("DELETE", path, undefined, cb) }
}
