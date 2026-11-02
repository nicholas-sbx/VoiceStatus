import QtQuick
import QtQuick.Controls as QQC2
import org.kde.kcmutils as KCM
import org.kde.kirigami as Kirigami

KCM.SimpleKCM {
    property alias cfg_localhostPort: localhostPort.value
    property alias cfg_reconnectDelay: reconnectDelay.value

    Kirigami.FormLayout {
        QQC2.SpinBox {
            id: localhostPort
            Kirigami.FormData.label: i18n("VoiceServer port:")
            from: 1024 // lowest unprivileged port
            to: 65535
            textFromValue: (value, locale) => String(value) // no thousands separator
            valueFromText: (text, locale) => parseInt(text, 10) || from
        }

        QQC2.SpinBox {
            id: reconnectDelay
            Kirigami.FormData.label: i18n("Reconnect delay:")
            from: 1
            to: 60
            textFromValue: (value, locale) => i18np("%1 second", "%1 seconds", value)
            valueFromText: (text, locale) => parseInt(text.replace(/\D/g, ""), 10) || from
        }

        Item {
            Kirigami.FormData.isSection: true
        }

        QQC2.Button {
            Kirigami.FormData.label: i18n("Server setup instructions:")
            text: "github.com/pipipear/VoiceStatus"
            icon.name: "internet-services"
            onClicked: Qt.openUrlExternally("https://github.com/pipipear/VoiceStatus")
        }
    }
}
