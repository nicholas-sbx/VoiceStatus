# VoiceStatus
A KDE Plasma 6 widget that shows the users in your Discord server's voice channels, for horizontal and vertical panels.

It has two parts:
- **VoiceServer**, a small background service that logs in to Discord with a bot account and serves voice channel data on `127.0.0.1` only.
- **The Plasmoid**, the panel widget that connects to VoiceServer and shows each occupied channel's users as avatars.

# Installation
## Server
1. Create a bot in the [Discord Developer Portal](https://discord.com/developers/applications) (Bot → Reset Token to get its token).
2. Under **Bot → Privileged Gateway Intents**, enable **Server Members Intent**. Nicknames and avatars come from it.
3. Invite the bot to your server. Under OAuth2 → URL Generator, select the `bot` scope and the **View Channels** permission.

Then install the server, either with Go 1.25 or newer:
```bash
go install github.com/nicholas-sbx/VoiceStatus/VoiceServer@latest
```
or with a prebuilt binary from the [latest release](https://github.com/nicholas-sbx/VoiceStatus/releases/latest) (`VoiceServer-linux-amd64` or `VoiceServer-linux-arm64`):
```bash
install -Dm755 VoiceServer-linux-amd64 ~/go/bin/VoiceServer
```
Set up the service and write the config file:
```bash
wget github.com/nicholas-sbx/VoiceStatus/raw/main/VoiceServer.service -P ~/.config/systemd/user/
nano ~/.config/VoiceStatus.json
chmod 600 ~/.config/VoiceStatus.json # the file holds your bot token
```
Add the following to the config file. Use a **bot** token; user tokens ("self-bots") are against Discord's Terms of Service and don't work.
```json
{
  "PORT": "5462",
  "TOKEN": "your-bot-token"
}
```
Start the server:
```bash
systemctl enable --now --user VoiceServer.service
```
Check it with `journalctl --user -u VoiceServer.service`.

## Plasmoid
The widget needs Plasma 6 and the Qt 6 WebSockets QML module, for example:
```bash
sudo apt install qml6-module-qtwebsockets  # Debian / Ubuntu
sudo pacman -S qt6-websockets              # Arch
sudo dnf install qt6-qtwebsockets          # Fedora
```
Install the widget from [the KDE Store](https://www.pling.com/p/1735750/), or download `VoiceStatus-<version>.plasmoid` from the [latest release](https://github.com/nicholas-sbx/VoiceStatus/releases/latest):
```bash
kpackagetool6 --type Plasma/Applet --install VoiceStatus-2.0.1.plasmoid  # first install
kpackagetool6 --type Plasma/Applet --upgrade VoiceStatus-2.0.1.plasmoid  # update
```
To install from a clone of this repository instead, pass the `Plasmoid` folder in place of the file.
Add **VoiceStatus** to a panel, then set your server's ID in its settings. To copy the ID, turn on Developer Mode in Discord (Settings → Advanced), then right-click the server and choose **Copy Server ID**.

The widget takes no space while nobody is in voice. To reach its settings then, enter panel edit mode, where its icon is shown.
