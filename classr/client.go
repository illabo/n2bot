package classr

import (
	"encoding/json"
	"n2bot/fatalist"
	"net/http"
	"os"
)

// Client is the type to provide communications with classificator.
type Client struct {
	httpClient *http.Client
	url        string
	errHandler *fatalist.Fatalist
}

// PredictClass takes .torrent file path and calls 'classificator' service.
// Returns TypePrediction and error.
func (c *Client) PredictClass(fp string) (TypePrediction, error) {
	var prediction TypePrediction
	var err error

	f, err := os.Open(fp)
	if err != nil {
		if c.errHandler != nil {
			c.errHandler.LogError(err)
		}
		return prediction, err
	}
	defer f.Close()
	req, err := http.NewRequest("POST", c.url, f)
	if err != nil {
		if c.errHandler != nil {
			c.errHandler.LogError(err)
		}
		return prediction, err
	}
	req.Header.Add("Content-Type", "application/octet-stream")
	res, err := c.httpClient.Do(req)
	if err != nil {
		if c.errHandler != nil {
			c.errHandler.LogError(err)
		}
		return prediction, err
	}
	defer res.Body.Close()

	err = json.NewDecoder(res.Body).Decode(&prediction)
	if err != nil {
		if c.errHandler != nil {
			c.errHandler.LogError(err)
		}
		return prediction, err
	}

	return prediction, err
}

// SetErrorHandler sets a error handler function to Client.
func (c *Client) SetErrorHandler(h *fatalist.Fatalist) {
	c.errHandler = h
}

// TypePrediction is the handful representation of 'classificator' results.
// Contains a Type prediction for provided .torrent and a Confidence as a float32.
// However Confidence is over .5 and below 1.0 whenever everything's went smooth.
type TypePrediction struct {
	Type       string  `json:"prediction"`
	Confidence float32 `json:"confidence"`
}

// NewClient creates new Client from config.
func NewClient(cfg *Config) *Client {
	return &Client{
		&http.Client{},
		cfg.URL,
		nil,
	}
}
