import QtQuick
import QtQuick.Window

Image {
    // Resize the bitmap once when loading it, instead of minifying the full
    // Discord texture with mipmaps at draw time. Keep enough pixels for HiDPI
    // screens; network bitmaps do not get Qt's automatic SVG/@2x scaling.
    sourceSize.width: Math.max(1, Math.ceil(width * Screen.devicePixelRatio))
    sourceSize.height: Math.max(1, Math.ceil(height * Screen.devicePixelRatio))

    fillMode: Image.PreserveAspectFit
    asynchronous: true
    smooth: true
}
