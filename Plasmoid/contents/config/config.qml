import QtQuick
import org.kde.plasma.configuration

ConfigModel {
    ConfigCategory {
        name: i18n("Display")
        icon: "configure"
        source: "configDisplay.qml"
    }
    ConfigCategory {
        name: i18n("Backend")
        icon: "preferences-system-network-server"
        source: "configBackend.qml"
    }
}
