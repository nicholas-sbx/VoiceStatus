import QtQuick
import QtQuick.Controls as QQC2
import QtQuick.Layouts
import org.kde.kcmutils as KCM
import org.kde.kirigami as Kirigami

KCM.SimpleKCM {
    property alias cfg_serverId:                serverId.text
    property alias cfg_sortByNick:              sortByNick.checked
    property alias cfg_sortByUsername:          sortByUsername.checked
    property alias cfg_sortById:                sortById.checked
    property alias cfg_sortDirectionAscending:  sortDirectionAscending.checked
    property alias cfg_sortDirectionDescending: sortDirectionDescending.checked
    property alias cfg_sortStyleApp:            sortStyleApp.checked
    property alias cfg_sortStyleOverlay:        sortStyleOverlay.checked
    property alias cfg_userSpacing:             userSpacing.value
    property alias cfg_channelSpacing:          channelSpacing.value
    property alias cfg_avatarMipmap:            avatarMipmap.checked

    Kirigami.FormLayout {
        QQC2.TextField {
            id: serverId
            Kirigami.FormData.label: i18n("Discord server ID:")
            placeholderText: i18n("Right-click the server › Copy Server ID")
            validator: RegularExpressionValidator { regularExpression: /^\s*\d*\s*$/ }
        }

        Item {
            Kirigami.FormData.isSection: true
        }

        // Each radio group needs its own parent: RadioButtons are
        // auto-exclusive with all their siblings, so groups sharing the
        // FormLayout would uncheck each other when the settings load.
        ColumnLayout {
            Kirigami.FormData.label: i18n("Sort users by:")
            Kirigami.FormData.buddyFor: sortByNick
            spacing: Kirigami.Units.smallSpacing

            QQC2.RadioButton {
                id: sortByNick
                text: i18n("Nickname")
            }
            QQC2.RadioButton {
                id: sortByUsername
                text: i18n("Username")
            }
            QQC2.RadioButton {
                id: sortById
                text: i18n("User ID")
            }
        }

        Item {
            Kirigami.FormData.isSection: true
        }

        ColumnLayout {
            Kirigami.FormData.label: i18n("Sort direction:")
            Kirigami.FormData.buddyFor: sortDirectionAscending
            spacing: Kirigami.Units.smallSpacing

            QQC2.RadioButton {
                id: sortDirectionAscending
                text: i18n("Ascending")
            }
            QQC2.RadioButton {
                id: sortDirectionDescending
                text: i18n("Descending")
            }
        }

        Item {
            Kirigami.FormData.isSection: true
        }

        ColumnLayout {
            Kirigami.FormData.label: i18n("Sort style:")
            Kirigami.FormData.buddyFor: sortStyleApp
            enabled: !sortById.checked
            spacing: Kirigami.Units.smallSpacing

            QQC2.RadioButton {
                id: sortStyleApp
                text: i18n("Discord app")
            }
            QQC2.RadioButton {
                id: sortStyleOverlay
                text: i18n("Discord overlay")
            }
        }

        Item {
            Kirigami.FormData.isSection: true
        }

        QQC2.SpinBox {
            id: userSpacing
            Kirigami.FormData.label: i18n("Spacing between users:")
            from: 0
            to: 99
            textFromValue: (value, locale) => i18nc("@item:valuesuffix pixels", "%1 px", value)
            valueFromText: (text, locale) => parseInt(text.replace(/\D/g, ""), 10) || 0
        }

        QQC2.SpinBox {
            id: channelSpacing
            Kirigami.FormData.label: i18n("Spacing between channels:")
            from: 0
            to: 99
            textFromValue: (value, locale) => i18nc("@item:valuesuffix pixels", "%1 px", value)
            valueFromText: (text, locale) => parseInt(text.replace(/\D/g, ""), 10) || 0
        }

        Item {
            Kirigami.FormData.isSection: true
        }

        QQC2.CheckBox {
            id: avatarMipmap
            Kirigami.FormData.label: i18n("Avatars:")
            text: i18n("Smooth scaling (mipmap)")
        }
    }
}
