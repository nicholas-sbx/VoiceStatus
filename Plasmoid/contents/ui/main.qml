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
    readonly property bool editMode: Plasmoid.containment?.corona?.editMode ?? false

    readonly property string serverId: Plasmoid.configuration.serverId.trim()
    readonly property bool connected: socket.status === WebSocket.Open
    property string discordStatus: "disconnected"

    // Last data received from VoiceServer: guild ID -> { channel ID -> [user] }
    property var servers: null

    // Sorted users of every occupied voice channel of the configured server.
    // Re-evaluated whenever the data or any sorting option changes.
    readonly property var channels: buildChannels(servers, serverId)

    // Without anything to show the widget would be invisible, so show its
    // icon while it is unconfigured or the panel is being edited.
    readonly property bool showPlaceholder: channels.length === 0 && (serverId === "" || editMode)

    // The widget has no popup: its content sits directly in the PlasmoidItem,
    // like the panel spacer or pager. (A compactRepresentation without a
    // fullRepresentation is never instantiated in Plasma 6.)
    preferredRepresentation: fullRepresentation

    // Hidden removes the widget from the panel while nobody is in voice;
    // panels still show hidden widgets in edit mode.
    Plasmoid.status: channels.length > 0 ? PlasmaCore.Types.ActiveStatus
        : serverId === "" ? PlasmaCore.Types.PassiveStatus
        : PlasmaCore.Types.HiddenStatus

    // Avatars fill the panel's thickness: its height when horizontal,
    // its width when vertical.
    readonly property real avatarSize: vertical ? width
        : horizontal ? height
        : Kirigami.Units.iconSizes.large

    readonly property real contentWidth: showPlaceholder ? avatarSize : content.implicitWidth
    readonly property real contentHeight: showPlaceholder ? avatarSize : content.implicitHeight

    // The panel reads these from the PlasmoidItem: grow along the panel,
    // fill it across.
    Layout.minimumWidth: vertical ? 0 : contentWidth
    Layout.preferredWidth: vertical ? -1 : contentWidth
    Layout.maximumWidth: vertical ? Infinity : contentWidth
    Layout.minimumHeight: horizontal ? 0 : contentHeight
    Layout.preferredHeight: horizontal ? -1 : contentHeight
    Layout.maximumHeight: horizontal ? Infinity : contentHeight

    toolTipMainText: Plasmoid.title
    toolTipTextFormat: Text.PlainText
    toolTipSubText: {
        if (!connected) {
            return i18n("Not connected to VoiceServer on port %1", Plasmoid.configuration.localhostPort);
        }
        if (discordStatus !== "connected") {
            return i18n("VoiceServer is not connected to Discord");
        }
        if (serverId === "") {
            return i18n("No server ID configured");
        }
        if (channels.length === 0) {
            return i18n("Nobody is in a voice channel");
        }
        return channels.map(users => users.map(u => displayName(u, true)).join(", ")).join("\n");
    }

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
        discordStatus = info.discordAPI || "disconnected";
        servers = parsed || null;
    }

    // Only load avatars from Discord's CDN, whatever the local server sends.
    function avatarUrl(url) {
        return typeof url === "string" && url.startsWith("https://cdn.discordapp.com/") ? url : "";
    }

    WebSocket {
        id: socket
        url: `ws://127.0.0.1:${Plasmoid.configuration.localhostPort}/ws`
        active: true

        onTextMessageReceived: message => root.handleMessage(message)
        onStatusChanged: status => {
            if (status === WebSocket.Error) {
                console.warn("VoiceStatus: websocket error:", socket.errorString);
            }
            if (status === WebSocket.Error || status === WebSocket.Closed) {
                root.servers = null;
                root.discordStatus = "disconnected";
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

    // Only the popup host (CompactApplet) draws the PlasmoidItem tooltip, and
    // this widget has none, so draw it here.
    PlasmaCore.ToolTipArea {
        anchors.fill: parent
        mainText: root.toolTipMainText
        subText: root.toolTipSubText
        textFormat: root.toolTipTextFormat
        location: Plasmoid.location

        Kirigami.Icon {
            anchors.centerIn: parent
            width: root.avatarSize
            height: root.avatarSize
            visible: root.showPlaceholder
            source: Plasmoid.icon
            active: placeholderMouse.containsMouse

            MouseArea {
                id: placeholderMouse
                anchors.fill: parent
                hoverEnabled: true
                onClicked: Plasmoid.internalAction("configure").trigger()
            }
        }

        GridLayout {
            id: content
            anchors.centerIn: parent
            visible: !root.showPlaceholder
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
                            id: avatar
                            required property var modelData

                            Layout.preferredWidth: root.avatarSize
                            Layout.preferredHeight: root.avatarSize
                            source: root.avatarUrl(modelData.avatar)
                            fillMode: Image.PreserveAspectFit
                            asynchronous: true
                            smooth: true
                            mipmap: Plasmoid.configuration.avatarMipmap

                            Kirigami.Icon {
                                anchors.fill: parent
                                visible: avatar.status !== Image.Ready
                                source: "user-identity"
                            }
                        }
                    }
                }
            }
        }
    }
}
