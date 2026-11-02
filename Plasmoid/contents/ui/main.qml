pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Layouts
import QtWebSockets // not installed by default, see README
import org.kde.plasma.plasmoid

PlasmoidItem {
    id: root

    readonly property string serverId: Plasmoid.configuration.serverId.trim()

    // Last data received from VoiceServer: guild ID -> { channel ID -> [user] }
    property var servers: null

    // Sorted users of every occupied voice channel of the configured server.
    // Re-evaluated whenever the data or any sorting option changes.
    readonly property var channels: buildChannels(servers, serverId)

    preferredRepresentation: compactRepresentation

    // Same order as the Plasma 5 version: by name with the chosen style,
    // or as received when sorting by ID.
    function sortUsers(users) {
        const cfg = Plasmoid.configuration;
        if (!cfg.sortByNick && !cfg.sortByUsername) {
            return cfg.sortDirectionDescending ? users.reverse() : users;
        }
        const order = cfg.sortDirectionAscending ? 1 : -1;
        const name = u => (cfg.sortByNick && u.nick) || u.username;
        if (cfg.sortStyleOverlay) {
            return users.sort((a, b) => name(a).localeCompare(name(b)) * order);
        }
        return users.sort((a, b) => {
            const an = name(a).toLocaleLowerCase(), bn = name(b).toLocaleLowerCase();
            if (an !== bn) {
                return (an < bn ? -1 : 1) * order;
            }
            return (a.id - b.id) * order;
        });
    }

    function buildChannels(allServers, id) {
        const guild = allServers && id ? allServers[id] : null;
        if (!guild) {
            return [];
        }
        const result = [];
        // The server sends channels in the order Discord lists them.
        for (const users of Object.values(guild)) {
            if (Array.isArray(users) && users.length > 0) {
                result.push(sortUsers(users.slice()));
            }
        }
        return result;
    }

    function handleMessage(message) {
        let info, parsed;
        try {
            info = JSON.parse(message);
            // voiceChannels is itself JSON encoded, or null before Discord is ready.
            parsed = typeof info.voiceChannels === "string" ? JSON.parse(info.voiceChannels) : info.voiceChannels;
        } catch (e) {
            console.warn("VoiceStatus: ignoring malformed message:", e);
            return;
        }
        servers = parsed || null;
    }

    WebSocket {
        id: socket
        url: `ws://localhost:${Plasmoid.configuration.localhostPort}/ws`
        active: true

        onTextMessageReceived: message => root.handleMessage(message)
        onStatusChanged: status => {
            if (status === WebSocket.Error) {
                console.warn("VoiceStatus: websocket error:", socket.errorString);
            }
            if (status === WebSocket.Error || status === WebSocket.Closed) {
                root.servers = null;
                socket.active = false;
                reconnectTimer.restart();
            }
        }
    }

    Timer {
        id: reconnectTimer
        interval: Plasmoid.configuration.reconnectDelay * 1000
        onTriggered: socket.active = true
    }

    compactRepresentation: Item {
        id: compact

        // Avatars fill the panel's height.
        readonly property real avatarSize: height

        // Grow along the panel as channels and users are added.
        Layout.minimumWidth: content.implicitWidth
        Layout.preferredWidth: content.implicitWidth
        Layout.maximumWidth: content.implicitWidth

        RowLayout {
            id: content
            anchors.centerIn: parent
            spacing: Plasmoid.configuration.channelSpacing

            Repeater {
                model: root.channels

                delegate: RowLayout {
                    id: channel
                    required property var modelData

                    spacing: Plasmoid.configuration.userSpacing

                    Repeater {
                        model: channel.modelData

                        delegate: Image {
                            required property var modelData

                            Layout.preferredWidth: compact.avatarSize
                            Layout.preferredHeight: compact.avatarSize
                            source: modelData.avatar
                            fillMode: Image.PreserveAspectFit
                            asynchronous: true
                            smooth: true
                            mipmap: Plasmoid.configuration.avatarMipmap
                        }
                    }
                }
            }
        }
    }
}
