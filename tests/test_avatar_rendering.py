"""Render the actual avatar component against Qt's smooth-resize reference.

Run with PySide6-Essentials 6.8.2 and an OpenGL-capable display, for example:
    xvfb-run -a python tests/test_avatar_rendering.py

The baseline option renders the 2.0.0 image path and must fail these checks:
    xvfb-run -a python tests/test_avatar_rendering.py --baseline
"""

import argparse
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import time

from PySide6.QtCore import QEventLoop, QObject, QPointF, QSize, Qt, QUrl
from PySide6.QtGui import QColor, QGuiApplication, QImage, QImageReader, QPainter, QPen
from PySide6.QtQuick import QQuickView, QQuickWindow, QSGRendererInterface


UI_DIR = Path(__file__).resolve().parents[1] / "Plasmoid/contents/ui"
BACKGROUND = QColor("#202225")


def fixture(path):
    """A Discord-sized bitmap with fine linework, curves, and transparency."""
    image = QImage(128, 128, QImage.Format.Format_ARGB32_Premultiplied)
    image.fill(Qt.GlobalColor.transparent)
    painter = QPainter(image)
    painter.setRenderHint(QPainter.RenderHint.Antialiasing)
    painter.setPen(Qt.PenStyle.NoPen)
    painter.setBrush(QColor("#403c94"))
    painter.drawEllipse(2, 2, 124, 124)
    painter.setBrush(QColor("#f2caa7"))
    painter.drawEllipse(28, 19, 73, 95)
    painter.setBrush(QColor("#211e35"))
    painter.drawEllipse(23, 9, 82, 48)
    painter.setPen(QPen(QColor("#aa9bff"), 1.5))
    for x in range(25, 106, 5):
        painter.drawLine(QPointF(x, 15), QPointF(x - 15, 49))
    painter.setPen(QPen(QColor("#20192b"), 2.5))
    painter.drawArc(40, 58, 15, 10, 0, 180 * 16)
    painter.drawArc(73, 58, 15, 10, 0, 180 * 16)
    painter.drawArc(51, 79, 25, 14, 180 * 16, 180 * 16)
    painter.setPen(QPen(QColor("#ffffff"), 1.5))
    for y in range(72, 120, 4):
        painter.drawLine(QPointF(8, y), QPointF(28, y - 12))
    painter.end()
    assert image.save(str(path))
    return image


def pixels(image):
    rgba = image.convertToFormat(QImage.Format.Format_RGBA8888)
    return bytes(rgba.constBits())


def mean_error(a, b):
    assert a.size() == b.size(), (a.size(), b.size())
    pairs = zip(pixels(a), pixels(b))
    return sum(abs(x - y) for x, y in pairs) / (a.width() * a.height() * 4)


def reference(source_path, size):
    # Match Qt's decode-and-resize path, including the PNG reader's alpha
    # conversion. Resizing the pre-encoded QPainter image can differ at >50%.
    reader = QImageReader(str(source_path))
    reader.setScaledSize(size)
    resized = reader.read()
    assert not resized.isNull(), reader.errorString()
    target = QImage(size, QImage.Format.Format_ARGB32_Premultiplied)
    target.fill(BACKGROUND)
    painter = QPainter(target)
    painter.drawImage(0, 0, resized)
    painter.end()
    return target


def wait_for(app, predicate):
    deadline = time.monotonic() + 5
    while time.monotonic() < deadline:
        app.processEvents(QEventLoop.ProcessEventsFlag.AllEvents, 20)
        if predicate():
            return
        time.sleep(0.005)
    raise AssertionError("Timed out waiting for the avatar to render")


def worker(args):
    QQuickWindow.setGraphicsApi(QSGRendererInterface.GraphicsApi.OpenGL)
    app = QGuiApplication([])
    with tempfile.TemporaryDirectory() as directory:
        directory = Path(directory)
        source_path = directory / "avatar.png"
        fixture(source_path)
        component = "Image" if args.baseline else "VoiceStatus.AvatarImage"
        qml_path = directory / "render.qml"
        qml_path.write_text(f'''
import QtQuick
import "{QUrl.fromLocalFile(str(UI_DIR)).toString()}" as VoiceStatus
Rectangle {{
    width: 28; height: 28
    color: "{BACKGROUND.name()}"
    {component} {{
        objectName: "avatar"
        property bool imageReady: status === Image.Ready
        anchors.fill: parent
        source: "{QUrl.fromLocalFile(str(source_path)).toString()}"
        fillMode: Image.PreserveAspectFit
        smooth: true
        mipmap: true
    }}
}}
''')
        view = QQuickView()
        view.setResizeMode(QQuickView.ResizeMode.SizeRootObjectToView)
        view.setSource(QUrl.fromLocalFile(str(qml_path)))
        assert not view.errors(), "\n".join(map(str, view.errors()))
        avatar = view.rootObject().findChild(QObject, "avatar")
        frames = []
        view.frameSwapped.connect(lambda: frames.append(True))
        view.show()
        wait_for(app, lambda: len(frames) > 0 and avatar.property("imageReady"))
        api = view.rendererInterface().graphicsApi()
        assert api == QSGRendererInterface.GraphicsApi.OpenGL, api
        scale = view.devicePixelRatio()
        failures = []
        for size in (24, 28, 32, 48):
            view.resize(QSize(size, size))
            for mipmap in (True, False):
                avatar.setProperty("mipmap", mipmap)
                previous = len(frames)
                view.requestUpdate()
                wait_for(app, lambda: len(frames) > previous
                         and avatar.property("imageReady"))
                actual = view.grabWindow()
                assert not actual.isNull()
                expected_size = QSize(round(size * scale), round(size * scale))
                assert actual.size() == expected_size, (actual.size(), expected_size)
                error = mean_error(actual, reference(source_path, actual.size()))
                print(f"scale={scale:g} size={size} mipmap={mipmap} error={error:.3f}", flush=True)
                if args.output:
                    output = Path(args.output)
                    output.mkdir(parents=True, exist_ok=True)
                    actual.save(str(output / f"avatar-{scale:g}-{size}-{mipmap}.png"))
                if error > 1.0:
                    failures.append((scale, size, mipmap, error))
        view.close()
        assert not failures, f"Avatar differs from a smooth resize: {failures}"


if __name__ == "__main__":
    parser = argparse.ArgumentParser()
    parser.add_argument("--worker", action="store_true")
    parser.add_argument("--baseline", action="store_true")
    parser.add_argument("--output")
    args = parser.parse_args()
    if args.worker:
        worker(args)
    else:
        for scale in ("1", "1.25", "1.5", "2"):
            env = dict(os.environ, QT_SCALE_FACTOR=scale, QSG_RHI_BACKEND="opengl")
            subprocess.run([sys.executable, __file__, "--worker", *sys.argv[1:]],
                           env=env, check=True)
