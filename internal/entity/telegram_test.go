package entity_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

func TestStartReadsTheCodeOnlyWhenTheCommandIsForThisBot(t *testing.T) {
	cases := []struct {
		text string
		code string
		ok   bool
	}{
		{"/start abc123", "abc123", true},
		{"/start@Ada_Bot abc123", "abc123", true},
		{"/start@other_bot abc123", "", false},
		{"/start", "", false},
		{"/start abc 123", "", false},
		{"/help abc", "", false},
		{"start abc", "", false},
	}

	for _, tc := range cases {
		code, ok := entity.TelegramMessage{Text: tc.text}.Start("ada_bot")
		if code != tc.code || ok != tc.ok {
			t.Errorf("Start(%q) = %q, %v; want %q, %v", tc.text, code, ok, tc.code, tc.ok)
		}
	}
}

func TestAGroupMessageIsAddressedOnlyByMentionOrReplyToTheBot(t *testing.T) {
	bot := entity.TelegramBot{BotUserID: 42, Username: "ada_bot"}

	cases := []struct {
		name    string
		message entity.TelegramMessage
		want    bool
	}{
		{"private chat", entity.TelegramMessage{ChatType: entity.TelegramChatPrivate}, true},
		{"unaddressed group chatter", entity.TelegramMessage{ChatType: entity.TelegramChatSupergroup}, false},
		{"mention", entity.TelegramMessage{ChatType: entity.TelegramChatGroup, Mentions: []string{"ADA_bot"}}, true},
		{"someone else mentioned", entity.TelegramMessage{ChatType: entity.TelegramChatGroup, Mentions: []string{"bob"}}, false},
		{"reply to the bot", entity.TelegramMessage{ChatType: entity.TelegramChatGroup, ReplyToSenderID: 42}, true},
		{"reply to a person", entity.TelegramMessage{ChatType: entity.TelegramChatGroup, ReplyToSenderID: 7}, false},
	}

	for _, tc := range cases {
		if got := tc.message.AddressedTo(bot); got != tc.want {
			t.Errorf("%s: AddressedTo = %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestPromptDropsTheBotMentionButKeepsTheQuestion(t *testing.T) {
	message := entity.TelegramMessage{Text: "@Ada_Bot what is blocking\nNORN-12? cc @ada_botany"}

	got := message.Prompt("ada_bot")
	if got != "what is blocking\nNORN-12? cc @ada_botany" {
		t.Errorf("Prompt = %q", got)
	}
}

func TestCallbackOptionAcceptsOnlyAnAnswerIndex(t *testing.T) {
	cases := []struct {
		data  string
		index int
		ok    bool
	}{
		{entity.TelegramOptionData(3), 3, true},
		{"a:0", 0, true},
		{"a:-1", 0, false},
		{"a:x", 0, false},
		{"b:1", 0, false},
		{"", 0, false},
	}

	for _, tc := range cases {
		index, ok := entity.TelegramCallback{Data: tc.data}.Option()
		if index != tc.index || ok != tc.ok {
			t.Errorf("Option(%q) = %d, %v; want %d, %v", tc.data, index, ok, tc.index, tc.ok)
		}
	}
}

func TestOnlyAPersonCanActThroughTelegram(t *testing.T) {
	cases := []struct {
		sender entity.TelegramSender
		want   bool
	}{
		{entity.TelegramSender{ID: 7}, true},
		{entity.TelegramSender{ID: 7, IsBot: true}, false},
		{entity.TelegramSender{ID: 7, Anonymous: true}, false},
		{entity.TelegramSender{}, false},
	}

	for _, tc := range cases {
		if got := tc.sender.Person(); got != tc.want {
			t.Errorf("%+v.Person() = %v, want %v", tc.sender, got, tc.want)
		}
	}
}

func TestBotTokenValidation(t *testing.T) {
	cases := []struct {
		token string
		code  string
	}{
		{"123456789:AAHdqTcvCH1vGWJxfSeofSAs0K5PALDsaw", ""},
		{"  123456789:AAHdqTcvCH1vGWJxfSeofSAs0K5PALDsaw  ", ""},
		{"", entity.ValidationCodeRequired},
		{"AAHdqTcvCH1vGWJxfSeofSAs0K5PALDsaw", entity.ValidationCodeMalformed},
		{"123456789:short", entity.ValidationCodeMalformed},
		{"123456789:AAHdqTcvCH1vGWJxfSeofSAs0K5PAL/saw", entity.ValidationCodeMalformed},
	}

	for _, tc := range cases {
		if got := entity.ValidateTelegramBotToken("token", tc.token).Code; got != tc.code {
			t.Errorf("ValidateTelegramBotToken(%q) = %q, want %q", tc.token, got, tc.code)
		}
	}
}

func TestDeepLinksUseStartForPeopleAndStartgroupForGroups(t *testing.T) {
	bot := entity.TelegramBot{Username: "ada_bot"}

	if got := bot.DeepLink(entity.TelegramLinkPrivate, "c0de"); got != "https://t.me/ada_bot?start=c0de" {
		t.Errorf("private link = %q", got)
	}

	if got := bot.DeepLink(entity.TelegramLinkGroup, "c0de"); got != "https://t.me/ada_bot?startgroup=c0de" {
		t.Errorf("group link = %q", got)
	}
}

func TestLinkCodesFitTheStartParameterAndHashToTheirStoredForm(t *testing.T) {
	code, hash, err := entity.NewTelegramLinkCode()
	if err != nil {
		t.Fatalf("NewTelegramLinkCode: %v", err)
	}

	if len(code) > 64 || strings.Trim(code, "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789_-") != "" {
		t.Errorf("code %q is not a valid start parameter", code)
	}

	if !bytes.Equal(entity.HashTelegramSecret(code), hash) {
		t.Error("the returned hash is not the hash of the code")
	}
}

func TestALinkedAccountActsAsItselfThroughTheTelegramConnection(t *testing.T) {
	account := entity.TelegramAccount{BotID: uuid.New(), AccountID: uuid.New()}

	actor := account.Actor()

	if actor.Kind != entity.ActorKindToken || actor.AccountID != account.AccountID ||
		actor.Authority() != account.AccountID {
		t.Errorf("actor = %+v", actor)
	}

	if actor.ConnectionID == nil || *actor.ConnectionID != account.BotID ||
		actor.ConnectionName != entity.TelegramConnectionName {
		t.Errorf("actor connection = %v %q", actor.ConnectionID, actor.ConnectionName)
	}
}

func TestTokenHintKeepsOnlyTheTail(t *testing.T) {
	if got := entity.TelegramTokenHint("123:abcdefghWXYZ"); got != "WXYZ" {
		t.Errorf("hint = %q", got)
	}
}
