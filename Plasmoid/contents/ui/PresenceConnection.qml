pragma ComponentBehavior: Bound

import QtCore
import QtQuick
import QtWebSockets

Item {
    id: connection

    required property int port
    required property int reconnectDelay
    readonly property bool authenticated: socket.authenticated
    signal snapshot(var info)

    Settings {
        id: credentials
        location: StandardPaths.writableLocation(StandardPaths.RuntimeLocation)
            + "/voicestatus-" + connection.port + ".ini"
    }

    function connectToServer() {
        retry.stop();
        credentials.sync();
        const keys = String(credentials.value("keys", "")).split(".");
        if (keys.length !== 2 || !keys[0] || !keys[1]) {
            retry.restart();
            return;
        }
        socket.serverProof = keys[1];
        socket.requestedSubprotocols = ["voicestatus." + keys[0]];
        socket.url = "ws://127.0.0.1:" + port + "/ws";
        socket.active = true;
    }

    function reconnect() {
        socket.authenticated = false;
        socket.active = false;
        retry.restart();
    }

    onPortChanged: Qt.callLater(reconnect)
    Component.onCompleted: Qt.callLater(connectToServer)

    Timer {
        id: retry
        interval: Math.max(1, connection.reconnectDelay) * 1000
        onTriggered: connection.connectToServer()
    }
    Timer {
        interval: 5000
        running: socket.active && !socket.authenticated
        onTriggered: connection.reconnect()
    }

    WebSocket {
        id: socket
        property bool authenticated: false
        property string serverProof
        active: false
        onStatusChanged: status => {
            if (status === WebSocket.Error || status === WebSocket.Closed) {
                connection.reconnect();
            }
        }
        onTextMessageReceived: message => {
            let info;
            try {
                info = JSON.parse(message);
            } catch (e) {
                connection.reconnect();
                return;
            }
            if (!active || !serverProof || info?.serverProof !== serverProof) {
                connection.reconnect();
                return;
            }
            authenticated = true;
            connection.snapshot(info);
        }
    }
}
