// VoiceServer connects to Discord as a bot and serves the voice channel
// occupancy of every guild the bot is in over a local-only WebSocket, for the
// VoiceStatus Plasma widget to display.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/olahol/melody"
)

const (
	defaultPort = 5462

	// listenHost keeps the server reachable from this machine only. The
	// voice data includes user IDs and names, so it must not be exposed to
	// the network.
	listenHost = "127.0.0.1"

	// updateDebounce coalesces bursts of Discord events (e.g. someone
	// joining a channel triggers several) into a single broadcast.
	updateDebounce = 250 * time.Millisecond
)

// Config is read from ~/.config/VoiceStatus.json. Keys match
// case-insensitively, so "PORT" and "port" both work.
type Config struct {
	Port  flexibleInt `json:"PORT"`
	Token string      `json:"TOKEN"`
}

// flexibleInt accepts both a JSON number and a numeric string, since the
// documented example config uses a string ("5462").
type flexibleInt int

func (f *flexibleInt) UnmarshalJSON(b []byte) error {
	s := strings.Trim(string(b), `"`)
	if s == "" || s == "null" {
		return nil
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return fmt.Errorf("expected a number, got %s", b)
	}
	*f = flexibleInt(n)
	return nil
}

// VoiceUser is the per-user payload sent to the widget. The JSON field names
// are part of the wire protocol and must stay stable.
type VoiceUser struct {
	UserID        string `json:"user_id"`
	Avatar        string `json:"avatar"`
	Nick          string `json:"nick"`
	GlobalName    string `json:"global_name"`
	Username      string `json:"username"`
	Discriminator string `json:"discriminator"`
	Suppress      bool   `json:"suppress"`
	SelfMute      bool   `json:"self_mute"`
	SelfDeaf      bool   `json:"self_deaf"`
	Mute          bool   `json:"mute"`
	Deaf          bool   `json:"deaf"`
}

type channelUsers struct {
	ChannelID string
	Users     []*VoiceUser
}

// orderedChannels marshals to a JSON object whose keys (channel IDs) keep the
// order of the slice, i.e. the order the channels appear in the Discord
// client. A Go map would sort the keys alphabetically instead.
type orderedChannels []channelUsers

func (o orderedChannels) MarshalJSON() ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteByte('{')
	for i, c := range o {
		if i > 0 {
			buf.WriteByte(',')
		}
		key, err := json.Marshal(c.ChannelID)
		if err != nil {
			return nil, err
		}
		users, err := json.Marshal(c.Users)
		if err != nil {
			return nil, err
		}
		buf.Write(key)
		buf.WriteByte(':')
		buf.Write(users)
	}
	buf.WriteByte('}')
	return buf.Bytes(), nil
}

// info is the message broadcast to every widget. voiceChannels is a JSON
// encoded string (not a nested object) for compatibility with existing
// widget versions.
type info struct {
	DiscordAPI    string  `json:"discordAPI"`
	VoiceChannels *string `json:"voiceChannels"`
}

type server struct {
	ws *melody.Melody

	mu      sync.Mutex
	state   info
	payload []byte // state marshalled, sent to new clients

	updateMu      sync.Mutex
	updatePending bool
}

func newServer() *server {
	srv := &server{ws: melody.New()}
	// Browsers always send an Origin header with WebSocket handshakes, while
	// the widget (Qt) does not. Rejecting any Origin stops web pages from
	// reading the voice data through the user's browser.
	srv.ws.Upgrader.CheckOrigin = func(r *http.Request) bool {
		return r.Header.Get("Origin") == ""
	}
	srv.ws.HandleConnect(func(s *melody.Session) {
		// Hold the lock so a concurrent broadcast cannot overtake this
		// initial message. Write only queues, so this does not block.
		srv.mu.Lock()
		defer srv.mu.Unlock()
		if err := s.Write(srv.payload); err != nil {
			log.Printf("websocket write: %v", err)
		}
	})
	srv.set(func(i *info) { i.DiscordAPI = "disconnected" })
	return srv
}

// set applies fn to the shared state and broadcasts the result if it changed.
func (srv *server) set(fn func(*info)) {
	srv.mu.Lock()
	defer srv.mu.Unlock()
	fn(&srv.state)
	payload, err := json.Marshal(srv.state)
	if err != nil {
		log.Printf("marshal state: %v", err)
		return
	}
	if bytes.Equal(payload, srv.payload) {
		return
	}
	srv.payload = payload
	if err := srv.ws.Broadcast(payload); err != nil && !errors.Is(err, melody.ErrClosed) {
		log.Printf("websocket broadcast: %v", err)
	}
}

func (srv *server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Guard against DNS rebinding: a malicious site resolving its own
	// domain to 127.0.0.1 would otherwise pass the same-host checks.
	if !isLoopbackHost(r.Host) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	if err := srv.ws.HandleRequest(w, r); err != nil {
		log.Printf("websocket upgrade: %v", err)
	}
}

func isLoopbackHost(hostport string) bool {
	host, _, err := net.SplitHostPort(hostport)
	if err != nil {
		host = hostport
	}
	host = strings.Trim(host, "[]")
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// scheduleUpdate rebuilds the voice channel list shortly after the first of a
// burst of events, instead of once per event.
func (srv *server) scheduleUpdate(s *discordgo.Session) {
	srv.updateMu.Lock()
	defer srv.updateMu.Unlock()
	if srv.updatePending {
		return
	}
	srv.updatePending = true
	time.AfterFunc(updateDebounce, func() {
		srv.updateMu.Lock()
		srv.updatePending = false
		srv.updateMu.Unlock()
		srv.updateVoiceChannels(s)
	})
}

func (srv *server) updateVoiceChannels(s *discordgo.Session) {
	guilds := snapshotVoiceChannels(s.State)
	encoded, err := json.Marshal(guilds)
	if err != nil {
		log.Printf("marshal voice channels: %v", err)
		return
	}
	str := string(encoded)
	srv.set(func(i *info) { i.VoiceChannels = &str })
}

// snapshotVoiceChannels returns, per guild ID, every voice channel that has
// users in it, in the order the Discord client lists them.
func snapshotVoiceChannels(st *discordgo.State) map[string]orderedChannels {
	st.RLock()
	defer st.RUnlock()

	result := make(map[string]orderedChannels, len(st.Guilds))
	for _, g := range st.Guilds {
		result[g.ID] = guildVoiceChannels(g)
	}
	return result
}

// guildVoiceChannels must be called with the state read lock held.
func guildVoiceChannels(g *discordgo.Guild) orderedChannels {
	if len(g.VoiceStates) == 0 {
		return orderedChannels{}
	}

	// Look up only the members in voice, rather than indexing every member
	// of a possibly large guild on each update.
	members := make(map[string]*discordgo.Member, len(g.VoiceStates))
	for _, vs := range g.VoiceStates {
		members[vs.UserID] = nil
	}
	for _, m := range g.Members {
		if m.User != nil {
			if _, inVoice := members[m.User.ID]; inVoice {
				members[m.User.ID] = m
			}
		}
	}

	usersByChannel := make(map[string][]*VoiceUser)
	for _, vs := range g.VoiceStates {
		if vs.ChannelID == "" {
			continue
		}
		m := members[vs.UserID]
		if m == nil {
			m = vs.Member // VOICE_STATE_UPDATE events carry the member
		}
		if m == nil || m.User == nil {
			continue
		}
		usersByChannel[vs.ChannelID] = append(usersByChannel[vs.ChannelID], &VoiceUser{
			UserID:        vs.UserID,
			Avatar:        m.User.AvatarURL("128"),
			Nick:          m.Nick,
			GlobalName:    m.User.GlobalName,
			Username:      m.User.Username,
			Discriminator: m.User.Discriminator,
			Suppress:      vs.Suppress,
			SelfMute:      vs.SelfMute,
			SelfDeaf:      vs.SelfDeaf,
			Mute:          vs.Mute,
			Deaf:          vs.Deaf,
		})
	}

	// Order like the Discord client: channels without a category first,
	// then each category in turn, everything sorted by position.
	var categories []*discordgo.Channel
	voiceByParent := make(map[string][]*discordgo.Channel)
	for _, c := range g.Channels {
		switch c.Type {
		case discordgo.ChannelTypeGuildCategory:
			categories = append(categories, c)
		case discordgo.ChannelTypeGuildVoice, discordgo.ChannelTypeGuildStageVoice:
			if len(usersByChannel[c.ID]) > 0 {
				voiceByParent[c.ParentID] = append(voiceByParent[c.ParentID], c)
			}
		}
	}
	sortChannels(categories)

	out := make(orderedChannels, 0, len(usersByChannel))
	appendParent := func(parentID string) {
		chans := voiceByParent[parentID]
		sortChannels(chans)
		for _, c := range chans {
			out = append(out, channelUsers{ChannelID: c.ID, Users: usersByChannel[c.ID]})
		}
	}
	appendParent("")
	for _, cat := range categories {
		appendParent(cat.ID)
	}
	return out
}

// userInVoice reports whether a user is in a voice channel of a guild.
// discordgo's State.VoiceState reads the voice states without holding the
// state lock, so this takes the lock itself.
func userInVoice(st *discordgo.State, guildID, userID string) bool {
	st.RLock()
	defer st.RUnlock()
	for _, g := range st.Guilds {
		if g.ID != guildID {
			continue
		}
		for _, vs := range g.VoiceStates {
			if vs.UserID == userID && vs.ChannelID != "" {
				return true
			}
		}
		return false
	}
	return false
}

// sortChannels sorts a slice we own (never the shared state slices) by
// position, breaking ties by ID like Discord does.
func sortChannels(chans []*discordgo.Channel) {
	sort.SliceStable(chans, func(i, j int) bool {
		if chans[i].Position != chans[j].Position {
			return chans[i].Position < chans[j].Position
		}
		return snowflakeLess(chans[i].ID, chans[j].ID)
	})
}

func snowflakeLess(a, b string) bool {
	if len(a) != len(b) {
		return len(a) < len(b)
	}
	return a < b
}

func loadConfig() (Config, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return Config{}, err
	}
	path := filepath.Join(dir, "VoiceStatus.json")

	f, err := os.Open(path)
	if err != nil {
		return Config{}, err
	}
	defer f.Close()

	if fi, err := f.Stat(); err == nil && fi.Mode().Perm()&0o077 != 0 {
		log.Printf("warning: %s contains your bot token but is readable by other users; run: chmod 600 %q", path, path)
	}

	var cfg Config
	if err := json.NewDecoder(f).Decode(&cfg); err != nil {
		return Config{}, fmt.Errorf("parse %s: %w", path, err)
	}
	if cfg.Port == 0 {
		cfg.Port = defaultPort
	}
	if cfg.Port < 1 || cfg.Port > 65535 {
		return Config{}, fmt.Errorf("PORT %d out of range", cfg.Port)
	}
	cfg.Token = strings.TrimPrefix(strings.TrimSpace(cfg.Token), "Bot ")
	if cfg.Token == "" {
		return Config{}, fmt.Errorf("TOKEN is missing from %s", path)
	}
	return cfg, nil
}

func main() {
	log.SetFlags(0) // systemd's journal already timestamps lines

	cfg, err := loadConfig()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	srv := newServer()

	mux := http.NewServeMux()
	mux.Handle("/ws", srv)
	httpServer := &http.Server{
		Addr:              net.JoinHostPort(listenHost, strconv.Itoa(int(cfg.Port))),
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}
	listener, err := net.Listen("tcp", httpServer.Addr)
	if err != nil {
		log.Fatalf("listen: %v", err)
	}
	go func() {
		if err := httpServer.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("http: %v", err)
		}
	}()
	log.Printf("Listening on ws://%s/ws", httpServer.Addr)

	discord, err := discordgo.New("Bot " + cfg.Token)
	if err != nil {
		log.Fatalf("discord: %v", err)
	}
	// Only what is needed: guild/channel structure, voice states, and
	// member info (nicknames, avatars). Server Members is a privileged
	// intent and must be enabled in the Discord developer portal.
	discord.Identify.Intents = discordgo.IntentsGuilds |
		discordgo.IntentsGuildVoiceStates |
		discordgo.IntentsGuildMembers

	update := func(s *discordgo.Session) { srv.scheduleUpdate(s) }
	discord.AddHandler(func(s *discordgo.Session, _ *discordgo.Connect) {
		srv.set(func(i *info) { i.DiscordAPI = "connected" })
	})
	discord.AddHandler(func(s *discordgo.Session, _ *discordgo.Disconnect) {
		srv.set(func(i *info) { i.DiscordAPI = "disconnected" })
	})
	discord.AddHandler(func(s *discordgo.Session, _ *discordgo.Ready) { update(s) })
	discord.AddHandler(func(s *discordgo.Session, _ *discordgo.GuildCreate) { update(s) })
	discord.AddHandler(func(s *discordgo.Session, _ *discordgo.GuildDelete) { update(s) })
	discord.AddHandler(func(s *discordgo.Session, _ *discordgo.ChannelCreate) { update(s) })
	discord.AddHandler(func(s *discordgo.Session, _ *discordgo.ChannelUpdate) { update(s) })
	discord.AddHandler(func(s *discordgo.Session, _ *discordgo.ChannelDelete) { update(s) })
	discord.AddHandler(func(s *discordgo.Session, _ *discordgo.VoiceStateUpdate) { update(s) })
	discord.AddHandler(func(s *discordgo.Session, m *discordgo.GuildMemberUpdate) {
		// Only nickname/avatar changes of users currently in voice matter.
		if m.User == nil {
			return
		}
		if userInVoice(s.State, m.GuildID, m.User.ID) {
			update(s)
		}
	})

	if err := discord.Open(); err != nil {
		srv.set(func(i *info) { i.DiscordAPI = "error" })
		log.Fatalf("discord: %v", err)
	}
	log.Println("Logged in to Discord")

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	<-ctx.Done()

	log.Println("Shutting down")
	if err := discord.Close(); err != nil {
		log.Printf("discord close: %v", err)
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = srv.ws.Close()
	_ = httpServer.Shutdown(shutdownCtx)
}
