pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Layouts
import QtWebSockets // not installed by default, see README
import org.kde.kirigami as Kirigami
import org.kde.plasma.core as PlasmaCore
import org.kde.plasma.plasmoid

PlasmoidItem {
    id: root

    readonly property bool vertical: Plasmoid.formFactor === PlasmaCore.Types.Vertical
    readonly property bool horizontal: Plasmoid.formFactor === PlasmaCore.Types.Horizontal

    readonly property string serverId: Plasmoid.configuration.serverId.trim()

    // Last data received from VoiceServer: guild ID -> { channel ID -> [user] }
    property var servers: null

    // Sorted users of every occupied voice channel of the configured server.
    // Re-evaluated whenever the data or any sorting option changes.
    readonly property var channels: buildChannels(servers, serverId)

    preferredRepresentation: compactRepresentation

    function displayName(user, preferNick) {
        return (preferNick && (user.nick || user.global_name)) || user.username || "";
    }

    // Discord IDs are 64-bit integers, too large for JS numbers: compare as
    // strings, shorter means smaller.
    function compareIds(a, b) {
        if (a.length !== b.length) {
            return a.length - b.length;
        }
        return a < b ? -1 : (a > b ? 1 : 0);
    }

    function sortUsers(users) {
        const cfg = Plasmoid.configuration;
        const order = cfg.sortDirectionAscending ? 1 : -1;
        const byId = (a, b) => compareIds(String(a.user_id), String(b.user_id));

        if (!cfg.sortByNick && !cfg.sortByUsername) {
            return users.sort((a, b) => byId(a, b) * order);
        }

        const named = users.map(u => ({ user: u, name: displayName(u, cfg.sortByNick) }));
        if (cfg.sortStyleOverlay) {
            named.sort((a, b) => a.name.localeCompare(b.name) * order);
        } else {
            // Discord app style: case-insensitive, ties broken by user ID
            named.forEach(n => n.key = n.name.toLocaleLowerCase());
            named.sort((a, b) => {
                if (a.key !== b.key) {
                    return (a.key < b.key ? -1 : 1) * order;
                }
                return byId(a.user, b.user) * order;
            });
        }
        return named.map(n => n.user);
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

        // Avatars fill the panel's thickness: its height when horizontal,
        // its width when vertical.
        readonly property real avatarSize: root.vertical ? width
            : root.horizontal ? height
            : Kirigami.Units.iconSizes.large

        readonly property real contentWidth: content.implicitWidth
        readonly property real contentHeight: content.implicitHeight

        // Grow along the panel, fill it across.
        Layout.minimumWidth: root.vertical ? 0 : contentWidth
        Layout.preferredWidth: root.vertical ? -1 : contentWidth
        Layout.maximumWidth: root.vertical ? Infinity : contentWidth
        Layout.minimumHeight: root.horizontal ? 0 : contentHeight
        Layout.preferredHeight: root.horizontal ? -1 : contentHeight
        Layout.maximumHeight: root.horizontal ? Infinity : contentHeight

        GridLayout {
            id: content
            anchors.centerIn: parent
            flow: root.vertical ? GridLayout.TopToBottom : GridLayout.LeftToRight
            rowSpacing: Plasmoid.configuration.channelSpacing
            columnSpacing: Plasmoid.configuration.channelSpacing

            Repeater {
                model: root.channels

                delegate: GridLayout {
                    id: channel
                    required property var modelData

                    flow: content.flow
                    rowSpacing: Plasmoid.configuration.userSpacing
                    columnSpacing: Plasmoid.configuration.userSpacing

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
