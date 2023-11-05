package webhook

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go.uber.org/zap"
	"net/http"
)

type Webhook struct {
	discordWebhookUrl string
	logger            *zap.SugaredLogger
	client            *http.Client
}

func NewWebhook(discordWebhookUrl string, logger *zap.SugaredLogger) *Webhook {
	w := &Webhook{
		discordWebhookUrl: discordWebhookUrl,
		logger:            logger,
		client:            &http.Client{},
	}

	return w
}

type jsonData struct {
	Username  string `json:"username"`
	Content   string `json:"content"`
	AvatarUrl string `json:"avatar_url"`
}

func (w *Webhook) sendWebhookMessage(payload []byte) {
	req, err := http.NewRequest("POST", w.discordWebhookUrl, bytes.NewBuffer(payload))
	if err != nil {
		w.logger.Errorw("error on http post", err)
		return
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Go-Discord")

	resp, err := w.client.Do(req)
	if err != nil {
		w.logger.Errorw("error on request", err)
		return
	}

	if err := resp.Body.Close(); err != nil {
		w.logger.Errorw("error closing body", err)
		return
	}
}

func (w *Webhook) SendPlayerJoinWebhook(username string, uuid string, plrCount int64) {
	playersText := "players"
	if plrCount == 1 {
		playersText = "player"
	}

	jsonData, err := json.Marshal(jsonData{
		username,
		fmt.Sprintf("Joined the server! (%d %s)", plrCount, playersText),
		fmt.Sprintf("https://mc-heads.net/avatar/%s/100", uuid),
	})
	if err != nil {
		w.logger.Errorw("error marshalling json", err)
		return
	}

	go w.sendWebhookMessage(jsonData)
}

func (w *Webhook) SendPlayerLeftWebhook(username string, uuid string, plrCount int64) {
	playersText := "players"
	if plrCount == 1 {
		playersText = "player"
	}

	jsonData, err := json.Marshal(jsonData{
		username,
		fmt.Sprintf("Left the server! (%d %s)", plrCount, playersText),
		fmt.Sprintf("https://mc-heads.net/avatar/%s/100", uuid),
	})
	if err != nil {
		w.logger.Errorw("error marshalling json", err)
		return
	}

	go w.sendWebhookMessage(jsonData)
}
