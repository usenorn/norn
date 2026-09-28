package telegram

import (
	"encoding/json"
	"fmt"
)

type Update struct {
	UpdateID      int64              `json:"update_id"`
	Message       *Message           `json:"message"`
	CallbackQuery *CallbackQuery     `json:"callback_query"`
	MyChatMember  *ChatMemberUpdated `json:"my_chat_member"`
}

type User struct {
	ID        int64  `json:"id"`
	IsBot     bool   `json:"is_bot"`
	Username  string `json:"username"`
	FirstName string `json:"first_name"`
}

type Chat struct {
	ID    int64  `json:"id"`
	Type  string `json:"type"`
	Title string `json:"title"`
}

type Entity struct {
	Type   string `json:"type"`
	Offset int    `json:"offset"`
	Length int    `json:"length"`
}

type Message struct {
	MessageID       int64    `json:"message_id"`
	From            *User    `json:"from"`
	SenderChat      *Chat    `json:"sender_chat"`
	Chat            Chat     `json:"chat"`
	Text            string   `json:"text"`
	Entities        []Entity `json:"entities"`
	ReplyToMessage  *Message `json:"reply_to_message"`
	MigrateToChatID int64    `json:"migrate_to_chat_id"`
}

type CallbackQuery struct {
	ID      string   `json:"id"`
	From    User     `json:"from"`
	Message *Message `json:"message"`
	Data    string   `json:"data"`
}

type ChatMember struct {
	Status string `json:"status"`
}

type ChatMemberUpdated struct {
	Chat          Chat       `json:"chat"`
	From          User       `json:"from"`
	NewChatMember ChatMember `json:"new_chat_member"`
}

func Decode(body []byte) (Update, error) {
	var update Update
	if err := json.Unmarshal(body, &update); err != nil {
		return Update{}, fmt.Errorf("decode telegram update: %w", err)
	}

	return update, nil
}
