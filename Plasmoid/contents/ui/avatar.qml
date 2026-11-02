import QtQuick
import org.kde.plasma.plasmoid

Image {
    sourceSize.width: parent.height
    sourceSize.height: parent.height
    fillMode: Image.PreserveAspectFit
    mipmap: Plasmoid.configuration.avatarMipmap
}
