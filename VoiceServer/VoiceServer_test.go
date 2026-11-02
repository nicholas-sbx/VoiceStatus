package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bwmarrin/discordgo"
	"github.com/gorilla/websocket"
)

func TestGuildVoiceChannelsOrder(t *testing.T) {
	user := func(id string) *discordgo.Member {
		return &discordgo.Member{User: &discordgo.User{ID: id, Username: "u" + id}}
	}
	g := &discordgo.Guild{
		ID: "1",
		Channels: []*discordgo.Channel{
			{ID: "30", Type: discordgo.ChannelTypeGuildCategory, Position: 1},
			{ID: "20", Type: discordgo.ChannelTypeGuildCategory, Position: 0},
			{ID: "31", Type: discordgo.ChannelTypeGuildVoice, ParentID: "30", Position: 0},
			{ID: "22", Type: discordgo.ChannelTypeGuildVoice, ParentID: "20", Position: 1},
			{ID: "21", Type: discordgo.ChannelTypeGuildVoice, ParentID: "20", Position: 0},
			{ID: "10", Type: discordgo.ChannelTypeGuildVoice, Position: 5},
			{ID: "11", Type: discordgo.ChannelTypeGuildVoice, Position: 5}, // empty, omitted
		},
		Members: []*discordgo.Member{user("100"), user("101"), user("102")},
		VoiceStates: []*discordgo.VoiceState{
			{UserID: "100", ChannelID: "31"},
			{UserID: "101", ChannelID: "22"},
			{UserID: "102", ChannelID: "21"},
			{UserID: "103", ChannelID: "10", Member: user("103")}, // only on the voice state
			{UserID: "104", ChannelID: "10"},                      // unknown member, skipped
		},
	}

	got := guildVoiceChannels(g)
	var ids []string
	for _, c := range got {
		ids = append(ids, c.ChannelID)
	}
	if want := "10,21,22,31"; strings.Join(ids, ",") != want {
		t.Fatalf("channel order = %v, want %s", ids, want)
	}
	if n := len(got[0].Users); n != 1 {
		t.Fatalf("channel 10 has %d users, want 1", n)
	}

	b, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(b), `{"10":[{"user_id":"103"`) {
		t.Fatalf("unexpected JSON: %s", b)
	}
	var decoded map[string][]VoiceUser
	if err := json.Unmarshal(b, &decoded); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
}

func TestFlexibleInt(t *testing.T) {
	for in, want := range map[string]int{`{"PORT":"5462"}`: 5462, `{"port":1234}`: 1234, `{}`: 0} {
		var c Config
		if err := json.Unmarshal([]byte(in), &c); err != nil {
			t.Fatalf("%s: %v", in, err)
		}
		if int(c.Port) != want {
			t.Errorf("%s: got %d, want %d", in, c.Port, want)
		}
	}
	var c Config
	if err := json.Unmarshal([]byte(`{"PORT":"abc"}`), &c); err == nil {
		t.Error("expected error for non-numeric port")
	}
}

func TestWebSocketAccess(t *testing.T) {
	srv := newServer()
	ts := httptest.NewServer(srv)
	defer ts.Close()
	url := "ws" + strings.TrimPrefix(ts.URL, "http")

	// The widget: no Origin header, loopback Host.
	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		t.Fatalf("widget connection rejected: %v", err)
	}
	_, msg, err := conn.ReadMessage()
	conn.Close()
	if err != nil {
		t.Fatal(err)
	}
	if string(msg) != `{"discordAPI":"disconnected","voiceChannels":null}` {
		t.Fatalf("unexpected initial message %s", msg)
	}

	// A web page: browsers always send an Origin header.
	_, resp, err := websocket.DefaultDialer.Dial(url, http.Header{"Origin": {"http://evil.example"}})
	if err == nil || resp == nil || resp.StatusCode != http.StatusForbidden {
		t.Fatalf("browser origin should be rejected, got err=%v resp=%v", err, resp)
	}

	// DNS rebinding: non-loopback Host header.
	req, _ := http.NewRequest("GET", ts.URL+"/ws", nil)
	req.Host = "evil.example:5462"
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusForbidden {
		t.Fatalf("non-loopback Host got status %d, want 403", res.StatusCode)
	}
}

func TestUserInVoice(t *testing.T) {
	st := discordgo.NewState()
	if err := st.GuildAdd(&discordgo.Guild{
		ID:          "1",
		VoiceStates: []*discordgo.VoiceState{{UserID: "100", ChannelID: "10"}},
	}); err != nil {
		t.Fatal(err)
	}
	if !userInVoice(st, "1", "100") {
		t.Error("user 100 should be in voice")
	}
	if userInVoice(st, "1", "101") || userInVoice(st, "2", "100") {
		t.Error("unexpected match")
	}
}
