package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/olahol/melody"
)

const defaultPort = 5462

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

var m = melody.New()
var info map[string]interface{}
var rl int64 = time.Now().Unix()

type CustomVoiceState struct {
	UserID        string `json:"user_id"`
	Avatar        string `json:"avatar"`
	Nick          string `json:"nick"`
	Username      string `json:"username"`
	Discriminator string `json:"discriminator"`
	Suppress      bool   `json:"suppress"`
	SelfMute      bool   `json:"self_mute"`
	SelfDeaf      bool   `json:"self_deaf"`
	Mute          bool   `json:"mute"`
	Deaf          bool   `json:"deaf"`
}

func main() {
	cfg, err := loadConfig()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	m.Upgrader.CheckOrigin = nil // prevent web pages from connecting
	info = map[string]interface{}{
		"discordAPI":    "disconnected",
		"voiceChannels": nil,
	}

	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		m.HandleRequest(w, r)
	})

	m.HandleConnect(func(s *melody.Session) {
		jsonInfo, _ := json.Marshal(info)
		s.Write([]byte(jsonInfo))
	})

	go func() {
		log.Fatal(http.ListenAndServe(":"+strconv.Itoa(int(cfg.Port)), nil))
	}()

	discord, dgerr := discordgo.New("Bot " + cfg.Token)
	if dgerr != nil {
		fmt.Println(dgerr)
		info["discordAPI"] = "error"
		return
	}

	discord.AddHandler(voiceStateUpdate)
	discord.AddHandler(guildMemberUpdate)
	discord.AddHandler(connect)
	discord.AddHandler(disconnect)
	discord.Identify.Intents = discordgo.IntentsAll

	dgerr = discord.Open()
	if dgerr != nil {
		fmt.Println(dgerr)
		info["discordAPI"] = "error"
		return
	}

	fmt.Println("Logged in to Discord")
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-sc
	fmt.Println("\nClosing connection with Discord")
	discord.Close()
}

func voiceStateUpdate(s *discordgo.Session, m *discordgo.VoiceStateUpdate) {
	updateInfo(s)
}

func guildMemberUpdate(s *discordgo.Session, m *discordgo.GuildMemberUpdate) {
	now := time.Now().Unix()
	if now-rl > 10 {
		rl = now
		for _, Guild := range s.State.Guilds {
			for _, VoiceState := range Guild.VoiceStates {
				if m.User.ID == VoiceState.UserID {
					updateInfo(s)
					return
				}
			}
		}
	}
}

func connect(s *discordgo.Session, m *discordgo.Connect) {
	setInfo("discordAPI", "connected")
	time.Sleep(50 * time.Millisecond)
	for i := 0; i < 5; i++ {
		updateInfo(s)
		time.Sleep(1 * time.Second)
	}
}

func disconnect(s *discordgo.Session, m *discordgo.Disconnect) {
	setInfo("discordAPI", "disconnected")
}

func updateInfo(s *discordgo.Session) {
	Channels := make(map[string]map[string][]*CustomVoiceState)
	for _, Guild := range s.State.Guilds {
		Channels[Guild.ID] = make(map[string][]*CustomVoiceState)
		sort.Slice(Guild.Channels, func(i, j int) bool {
			return Guild.Channels[i].Position < Guild.Channels[j].Position
		})

		count := 0
		for _, Channel := range Guild.Channels {
			if len(Channel.ParentID) == 0 && Channel.Type == discordgo.ChannelTypeGuildVoice {
				channelUsers := processVC(Guild, Channel)
				orderHack := "=" + string(rune(count)) + "=" + Channel.ID // hack to force positional sort order
				Channels[Guild.ID][orderHack] = make([]*CustomVoiceState, len(channelUsers))
				Channels[Guild.ID][orderHack] = channelUsers
				count++
			}
		}

		// These loops can likely be improved
		for _, Category := range Guild.Channels {
			if Category.Type == discordgo.ChannelTypeGuildCategory {
				for _, Channel := range Guild.Channels {
					if Channel.ParentID == Category.ID && Channel.Type == discordgo.ChannelTypeGuildVoice {
						channelUsers := processVC(Guild, Channel)
						orderHack := "=" + string(rune(count)) + "=" + Channel.ID
						Channels[Guild.ID][orderHack] = make([]*CustomVoiceState, len(channelUsers))
						Channels[Guild.ID][orderHack] = channelUsers
						count++
					}
				}
			}
		}
	}

	// remove sort order hack
	jsonChannels, _ := json.MarshalIndent(Channels, "", "  ")
	orderHackRegexp := regexp.MustCompile(`    "=.*=`)
	fixedJsonChannels := orderHackRegexp.ReplaceAllString(string(jsonChannels), `    "`)

	fmt.Println(fixedJsonChannels)
	setInfo("voiceChannels", fixedJsonChannels)
}

func processVC(gd *discordgo.Guild, vc *discordgo.Channel) []*CustomVoiceState {
	vs := gd.VoiceStates
	gm := gd.Members
	var channelUsers []*CustomVoiceState
	for _, VState := range vs {
		if VState.ChannelID == vc.ID {
			fmt.Println(VState)
			for _, member := range gm {
				if member.User.ID == VState.UserID {
					CustomVState := &CustomVoiceState{
						UserID:        VState.UserID,
						Avatar:        member.User.AvatarURL("128"),
						Nick:          member.Nick,
						Username:      member.User.Username,
						Discriminator: member.User.Discriminator,
						Suppress:      VState.Suppress,
						SelfMute:      VState.SelfMute,
						SelfDeaf:      VState.SelfDeaf,
						Mute:          VState.Mute,
						Deaf:          VState.Deaf,
					}
					channelUsers = append(channelUsers, CustomVState)
					break // without this, users are listed twice
				}
			}
		}
	}
	return channelUsers
}

func setInfo(k string, v interface{}) {
	info[k] = v
	jsonInfo, _ := json.Marshal(info)
	m.Broadcast([]byte(jsonInfo))
}
