# Changelog

## 2.0.1 — 2026-11-10

- Restore size-aware avatar loading so smooth scaling preserves detail instead of blurring small panel avatars.
- Account for display scaling when choosing the loaded image size, including fractional and HiDPI displays.
- Keep the existing mipmap preference; smooth filtering remains enabled with either setting.
- Verify avatar rendering at 100%, 125%, 150%, and 200% display scaling before publishing releases.

## 2.0.0 — 2026-11-03

**Requires Plasma 6.** Plasma 5 users should stay on 1.0.1.

### Widget
- Ported to Plasma 6.
- Works in vertical panels: avatars fill the panel's width and stack top to bottom.
- Hover tooltip lists who is in each channel, or why nothing is shown.
- Hides itself while nobody is in voice; shows its icon in panel edit mode and while no server ID is set (click it to open the settings).
- Sorting fixes: "User ID" now sorts by ID, name ties are broken by ID, and nicknames fall back to Discord display names.
- Changing settings no longer reconnects to the server.
- The port setting can be typed in.
- Avatars are only loaded from Discord's CDN.

### Server (VoiceServer v1.1.0)
- **Security:** listens on 127.0.0.1 only (previously every network interface), rejects connections from web pages and DNS rebinding, and requests only the Discord intents it needs. Warns if the config file holding the token is readable by other users.
- Fixed data races that could crash the server.
- Nickname and avatar changes are no longer dropped; updates are batched instead.
- Channel order matches the Discord client, stage channels are included, and empty channels are left out.
- Much smaller: gin and viper replaced by Go's standard library; dependencies updated. Requires Go 1.25 to build; prebuilt binaries are attached to the release.
- Module path is now `github.com/nicholas-sbx/VoiceStatus/VoiceServer`.

### Setup changes
- The bot needs the **Server Members Intent** enabled in the Discord Developer Portal.
- Plasma 6 needs the Qt 6 WebSockets QML module (`qml6-module-qtwebsockets`, `qt6-websockets`, or `qt6-qtwebsockets`).
