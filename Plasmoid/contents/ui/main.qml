import QtQuick
import QtQuick.Layouts
import QtWebSockets // not installed by default, see README
import org.kde.plasma.plasmoid

PlasmoidItem {
    preferredRepresentation: compactRepresentation
    compactRepresentation: RowLayout {
        id: container
        spacing: Plasmoid.configuration.channelSpacing

        property var vclist: []

        property bool firstChangeCompleted: false // hacky solution

        property var sortByNick:              Plasmoid.configuration.sortByNick
        property var sortByUsername:          Plasmoid.configuration.sortByUsername
        property var sortById:                Plasmoid.configuration.sortById
        property var serverId:                Plasmoid.configuration.serverId
        property var sortDirectionAscending:  Plasmoid.configuration.sortDirectionAscending
        property var sortDirectionDescending: Plasmoid.configuration.sortDirectionDescending
        property var sortStyleApp:            Plasmoid.configuration.sortStyleApp
        property var sortStyleOverlay:        Plasmoid.configuration.sortStyleOverlay
        property var localhostPort:           Plasmoid.configuration.localhostPort
        property var reconnectDelay:          Plasmoid.configuration.reconnectDelay

        onSortByNickChanged:              applyTimer.running = true
        onSortByUsernameChanged:          applyTimer.running = true
        onSortByIdChanged:                applyTimer.running = true
        onServerIdChanged:                applyTimer.running = true
        onSortDirectionAscendingChanged:  applyTimer.running = true
        onSortDirectionDescendingChanged: applyTimer.running = true
        onSortStyleAppChanged:            applyTimer.running = true
        onSortStyleOverlayChanged:        applyTimer.running = true
        onLocalhostPortChanged:           applyTimer.running = true

        Timer {
            id: applyTimer
            interval: 10
            onTriggered: {
                if (container.firstChangeCompleted) {
                    socket.active = false
                    socket.active = true
                } else {
                    container.firstChangeCompleted = true
                }
            }
        }

        Timer {
            id: reconnectTimer
            interval: container.reconnectDelay * 1000
            onTriggered: socket.active = true
        }

        WebSocket {
            id: socket
            active: true
            url: `ws://localhost:${container.localhostPort}/ws`

            onTextMessageReceived: message => {
                container.clearChannels()
                const channel = Qt.createComponent("channel.qml")
                const servers = JSON.parse(JSON.parse(message)['voiceChannels'])
                const serverId = container.serverId
                if (!servers || servers[serverId] == null) return
                for (const value of Object.values(servers[serverId])) {
                    if (!value) continue
                    if (container.sortByNick || container.sortByUsername) {
                        const sortOrder = container.sortDirectionAscending ? 1 : -1
                        if (container.sortStyleApp) {
                            value.sort((a, b) => {
                                const aFriendly = container.sortByNick && a.nick || a.username
                                const bFriendly = container.sortByNick && b.nick || b.username
                                if (aFriendly.toLocaleLowerCase() < bFriendly.toLocaleLowerCase()) return -1 * sortOrder
                                if (aFriendly.toLocaleLowerCase() > bFriendly.toLocaleLowerCase()) return  1 * sortOrder
                                return (a.id - b.id) * sortOrder
                            })
                        } else if (container.sortStyleOverlay) {
                            value.sort((a, b) => (a.nick || a.username).localeCompare(b.nick || b.username) * sortOrder)
                        }
                    } else if (container.sortById && container.sortDirectionDescending) {
                        value.reverse()
                    }
                    container.vclist.push(channel.createObject(container, { users: value }))
                }
            }
            onStatusChanged: status => {
                if (status == WebSocket.Error) {
                    console.log(`ws error: ${socket.errorString}`)
                } else if (status == WebSocket.Open) {
                    console.log('ws connected')
                } else if (status == WebSocket.Closed) {
                    console.log('ws disconnected')
                    container.clearChannels()
                    socket.active = false
                    reconnectTimer.running = true
                }
            }
        }

        function clearChannels() {
            vclist.forEach(c => c.destroy())
            vclist = []
        }
    }
}
