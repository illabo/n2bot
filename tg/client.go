package tg

import (
	"bytes"
	"encoding/json"
	"fmt"
	"n2bot/fatalist"
	"n2bot/proxyurl"
	"net/http"
)

// Client is a type providing the app core with the connectivity to Telegram
type Client struct {
	token       string
	httpClient  *http.Client
	connTimeout uint
	// inChan is the channel where to get new incoming Telegram messages outside of this package
	inChan chan ChatMessage
	// outChan is the channel to send messages to whenever you need to send it over Telegram
	outChan    chan ChatMessage
	errHandler *fatalist.Fatalist
}

// GetInChan returns client's inChan:
// the channel where to get new incoming Telegram messages outside of this package.
// Consumers can only read from this channel.
func (c *Client) GetInChan() <-chan ChatMessage {
	return c.inChan
}

// GetOutChan returns client's outChan:
// the channel to send messages to whenever you need to send it over Telegram.
// Consumers can only write to this channel.
func (c *Client) GetOutChan() chan<- ChatMessage {
	return c.outChan
}

// SetProxy lets to setup a random proxy provider.
// If SetProxy won't called default http.Transport used with systemwide proxy settings if any.
func (c *Client) SetProxy(p *proxyurl.RandomProxy) {
	c.httpClient.Transport = &http.Transport{
		Proxy: p.Get,
	}
}

// SetErrorHandler sets a error handler function to Client.
func (c *Client) SetErrorHandler(h *fatalist.Fatalist) {
	c.errHandler = h
}

type ChatMessage struct {
	ChatID        string                      `json:"chat_id,omitempty"`
	Text          string                      `json:"text,omitempty"`
	Type          ChatMessageType             `json:"-"`
	Keyboard      map[string][][]InlineButton `json:"reply_markup,omitempty"`
	AnswerQueryID string                      `json:"callback_query_id,omitempty"`
}

func (m *ChatMessage) toJSON() (msg []byte, err error) {
	switch m.Type {
	case typingType:
		msg, err = json.Marshal(map[string]string{
			"chat_id": m.ChatID, "action": "typing",
		})
	case textType, callbackType:
		msg, err = json.Marshal(m)
	}
	return
}

type ChatMessageType byte

const (
	textType ChatMessageType = iota
	typingType
	callbackType
)

type InlineButton struct {
	Text         string `json:"text"`
	CallbackData string `json:"callback_data"`
}

type apiUpdate struct {
	Ok     bool `json:"ok"`
	Result []struct {
		UpdateID      int           `json:"update_id"`
		Message       apiMessage    `json:"message"`
		EditedMessage apiMessage    `json:"edited_message"`
		CallbackQuery callbackQuery `json:"callback_query"`
	} `json:"result"`
}

type apiMessage struct {
	MessageID int    `json:"message_id"`
	From      user   `json:"from"`
	User      user   `json:"user"`
	Date      int    `json:"date"`
	Text      string `json:"text"`
}

type callbackQuery struct {
	ID   string `json:"id"`
	Data string `json:"data"`
	From user   `json:"from"`
}

type user struct {
	ID int64 `json:"id"`
}

// Run should be called on Client to start listening on inChan and outChan
// otherwise channels would be inoperable and Telegram API never would be polled
// if no onMessage function is passed to Run default listener would be set to keep the channel alive
// default listener just prints out messages' sender id and text out
func (c *Client) Run(onMessage ...func(msg ChatMessage)) {
	if onMessage == nil {
		c.runWithListener(
			func(msg ChatMessage) { fmt.Println(msg) },
		)
		return
	}
	c.runWithListener(onMessage[0])
	return
}

func (c *Client) runWithListener(l func(msg ChatMessage)) {
	go func() {
		for {
			msg := <-c.inChan
			l(msg)
		}
	}()
	go c.waitForOutgoing()
	go c.startPolling()
}

func (c *Client) startPolling() {
	url := fmt.Sprintf("https://api.telegram.org/bot%s/getUpdates", c.token)
	offset := 0
	for {
		jsonBody := []byte(
			fmt.Sprintf(`{"timeout":%d,"offset":%d}`, c.connTimeout, offset),
		)
		req, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(jsonBody))
		if err != nil {
			if c.errHandler != nil {
				c.errHandler.FatalError(err)
			}
			return
		}
		req.Header.Set("Content-Type", "application/json")

		res, err := c.httpClient.Do(req)
		if err != nil {
			if c.errHandler != nil {
				c.errHandler.LogError(err)
			}
			//TODO: handle connectivity errors
		}

		var updateBody apiUpdate
		if res != nil {
			err = json.NewDecoder(res.Body).Decode(&updateBody)
			res.Body.Close()
			if err != nil {
				if c.errHandler != nil {
					c.errHandler.LogError(err)
				}
			}
		}

		updates := updateBody.Result
		updLen := len(updates)
		if updLen > 0 {
			offset = updates[updLen-1].UpdateID + 1
		}

		for _, m := range updates {
			if m.Message.MessageID > 0 {
				c.inChan <- ChatMessage{ChatID: fmt.Sprintf("%d", m.Message.From.ID), Text: m.Message.Text, Type: textType}
			}
			if m.EditedMessage.MessageID > 0 {
				c.inChan <- ChatMessage{ChatID: fmt.Sprintf("%d", m.EditedMessage.From.ID), Text: m.EditedMessage.Text, Type: textType}
			}
			if m.CallbackQuery.From.ID > 0 {
				c.inChan <- ChatMessage{
					ChatID: fmt.Sprintf("%d", m.CallbackQuery.From.ID),
					Text: fmt.Sprintf("%s -query_id=%s",
						m.CallbackQuery.Data,
						m.CallbackQuery.ID),
					Type: callbackType,
				}
			}
		}
	}
}

func (c *Client) waitForOutgoing() {
	textsURL := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", c.token)
	actionsURL := fmt.Sprintf("https://api.telegram.org/bot%s/sendChatAction", c.token)
	answerCallbackURL := fmt.Sprintf("https://api.telegram.org/bot%s/answerCallbackQuery", c.token)
	for {
		outMsg := <-c.outChan
		var url string
		msg, err := outMsg.toJSON()
		if err != nil {
			if c.errHandler != nil {
				c.errHandler.LogError(err)
			}
		}
		if outMsg.Type == textType {
			url = textsURL
		}
		if outMsg.Type == typingType {
			url = actionsURL
		}
		if outMsg.Type == callbackType {
			url = answerCallbackURL
		}

		req, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(msg))
		if err != nil {
			if c.errHandler != nil {
				c.errHandler.LogError(err)
			}
		}
		req.Header.Set("Content-Type", "application/json")

		res, err := c.httpClient.Do(req)
		if err != nil {
			if c.errHandler != nil {
				c.errHandler.LogError(err)
			}
			go func() { c.outChan <- outMsg }()
		}

		var resBody map[string]interface{}
		if res != nil {
			err = json.NewDecoder(res.Body).Decode(&resBody)
			if err != nil {
				if c.errHandler != nil {
					c.errHandler.LogError(err)
				}
				go func() { c.outChan <- outMsg }()
			}
			res.Body.Close()
		}
	}

}

// NewClient creates an instance of tg.Client
// Method should be provided with the bot token.
// Please note that the method doesn't return any error the client would be stuck if the sever is unavailable.
func NewClient(cfg *Config) *Client {
	if cfg == nil {
		cfg = &Config{}
	}
	return &Client{cfg.Token, &http.Client{}, cfg.ConnTimeout, make(chan ChatMessage), make(chan ChatMessage), nil}
}

func NewTextMessage(chatID, text string) ChatMessage {
	return ChatMessage{
		chatID,
		text,
		MessageTypeFromString("text"),
		nil,
		"",
	}
}

func NewTyping(chatID string) ChatMessage {
	return ChatMessage{
		chatID,
		"",
		MessageTypeFromString("typing"),
		nil,
		"",
	}
}

func NewTextWithKeyboard(chatID, text string, buttons []InlineButton) ChatMessage {
	return ChatMessage{
		chatID,
		text,
		MessageTypeFromString("text"),
		map[string][][]InlineButton{
			"inline_keyboard": [][]InlineButton{buttons},
		},
		"",
	}
}

func NewQueryAnswer(queryID string) ChatMessage {
	return ChatMessage{
		"",
		"",
		MessageTypeFromString("callback"),
		nil,
		queryID,
	}
}

func MessageTypeFromString(s string) ChatMessageType {
	switch s {
	case "typing":
		return typingType
	case "callback":
		return callbackType
	default:
		return textType
	}
}
