import QtQuick
import QtQuick.Layouts
import org.kde.plasma.plasmoid

RowLayout {
    id: channel
    Layout.minimumHeight: parent.height
    property var users: []
    spacing: Plasmoid.configuration.userSpacing
    Component.onCompleted: {
        const avatar = Qt.createComponent("avatar.qml");
        users.forEach(u => avatar.createObject(channel, { source: u.avatar }));
    }
}
