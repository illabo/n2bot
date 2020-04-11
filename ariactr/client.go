package ariactr

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"n2bot/fatalist"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Client is the type to provide communications with aria2.
type Client struct {
	httpClient       *http.Client
	aria2ServerURL   string
	pollingInterval  uint
	pollingTaskChan  chan pollingTask
	taskStatusesChan chan TaskStatus
	errHandler       *fatalist.Fatalist
}

// EnqueueMetadata method consumes ownerID/chatID (it is the same for "private" single user communication)
// and a magnet link. The GID of created task will be returned on success as the first value.
// Error is the second return value.
// EnqueueMetadata starts the task of collecting torrent metadata (downloading .torrent file) and placing it at
// the current working dir.
func (c *Client) EnqueueMetadata(ownerID, magnet string) (string, error) {
	dir := getWorkdir()
	req, err := http.NewRequest(
		http.MethodPost,
		c.aria2ServerURL,
		bytes.NewBuffer([]byte(fmt.Sprintf(`{
			"jsonrpc": "2.0",
			"id": "%s",
			"method": "aria2.addUri",
			"params": [
			  ["%s"],
			  {
				"dir":"%s",
				"bt-metadata-only": "true",
				"bt-save-metadata": "true",
				"bt-stop-timeout":600,
			  }
			]
		  }`, uuid.New(), magnet, dir))),
	)
	if err != nil {
		if c.errHandler != nil {
			c.errHandler.LogError(err)
		}
		return "", err
	}

	return doDownloadRequest(c, req, ownerID)
}

// EnqueueBT method consumes ownerID/chatID (it is the same for "private" single user communication),
// a target download dir (where to save downloaded files)
// and name of the .torrent file in current dir to read.
// The GID of created task will be returned on success as the first value.
// Error is the second return value.
// EnqueueBT starts the task of downloading files described in a .torrent file.
func (c *Client) EnqueueBT(ownerID, dlDir, torrentFile string) (string, error) {
	f, err := ioutil.ReadFile(getWorkdir() + torrentFile)
	f64str := base64.StdEncoding.EncodeToString(f)
	if err != nil {
		if c.errHandler != nil {
			c.errHandler.LogError(err)
		}
		return "", err
	}
	err = mustMkdirAll(dlDir)
	if err != nil {
		if c.errHandler != nil {
			c.errHandler.LogError(err)
		}
		return "", err
	}
	req, err := http.NewRequest(
		http.MethodPost,
		c.aria2ServerURL,
		bytes.NewBuffer([]byte(fmt.Sprintf(`{
			"jsonrpc": "2.0",
			"id": "%s",
			"method": "aria2.addTorrent",
			"params": ["%s",
			  [],
			  {
				"dir":"%s",
				"check-integrity":"true",
				"continue":"true",
				"bt-stop-timeout":86400,
			  }
			]
		  }`, uuid.New(), f64str, dlDir))),
	)
	if err != nil {
		if c.errHandler != nil {
			c.errHandler.LogError(err)
		}
		return "", err
	}

	return doDownloadRequest(c, req, ownerID)
}

// KillTask instructs aria2 to terminate task by provided GID.
func (c *Client) KillTask(gid string) error {
	req, err := http.NewRequest(
		http.MethodPost,
		c.aria2ServerURL,
		bytes.NewBuffer(
			[]byte(
				fmt.Sprintf(`{
			"jsonrpc": "2.0",
			"id": "someid",
			"method": "aria2.remove",
			"params": [
			  %q
			]
		  }`, gid),
			),
		),
	)
	if err != nil {
		if c.errHandler != nil {
			c.errHandler.FatalError(err)
		}
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	_, err = c.httpClient.Do(req)
	if err != nil {
		if c.errHandler != nil {
			c.errHandler.FatalError(err)
		}
	}
	return err
}

// TellActive reports about aria2 tasks in the current session.
// Returns an array of TaskStatus objects and a error.
func (c *Client) TellActive() ([]TaskStatus, error) {
	var statuses []TaskStatus
	var err error
	req, err := http.NewRequest(
		http.MethodPost,
		c.aria2ServerURL,
		bytes.NewBuffer([]byte(`{
			"jsonrpc": "2.0",
			"id": "tellActive",
			"method": "aria2.tellActive",
			"params": [
						[
						"gid", 
						"infohash", 
						"status", 
						"errorMessage",
						"completedLength", 
						"totalLength", 
						"bittorrent"
						]
					]
				}`)),
	)
	if err != nil {
		if c.errHandler != nil {
			c.errHandler.FatalError(err)
		}
		return statuses, err
	}
	req.Header.Set("Content-Type", "application/json")

	statuses, _, err = c.doStatusRequest(req)
	return statuses, err
}

// AddPollingTask starts polling for stored in database GIDs per owner.
func (c *Client) AddPollingTask(ownerID, gid string) {
	c.pollingTaskChan <- pollingTask{
		ownerID,
		gid,
	}
}

// SetErrorHandler sets a error handler function to Client.
func (c *Client) SetErrorHandler(h *fatalist.Fatalist) {
	c.errHandler = h
}

// Run polling and set a listener/handler if any.
// If no listener is provided results would be printed to stdout.
func (c *Client) Run(onStatus ...func(ts TaskStatus)) {
	if onStatus == nil {
		c.runWithListener(
			func(ts TaskStatus) { fmt.Println("TaskStatus", ts) },
		)
		return
	}
	c.runWithListener(onStatus[0])
	return
}

func (c *Client) runWithListener(l func(ts TaskStatus)) {
	go func() {
		for {
			ts := <-c.taskStatusesChan
			l(ts)
		}
	}()
	go c.startPolling()
}

func (c *Client) startPolling() {
	gidPerOwner := map[string]string{}
	timeoutChan := make(chan byte)
	go func() {
		for {
			time.Sleep(time.Duration(c.pollingInterval) * time.Second)
			select {
			case timeoutChan <- '1':
			default:
			}
		}
	}()
	for {
		select {
		case t := <-c.pollingTaskChan:
			gidPerOwner[t.gid] = t.ownerID
		case <-timeoutChan:
			if len(gidPerOwner) == 0 {
				break
			}
			calls := []string{}
			for k := range gidPerOwner {
				statusCall := fmt.Sprintf(`{
					"jsonrpc": "2.0",
					"id": "%s",
					"method": "aria2.tellStatus",
					"params": ["%s", 
								[
								"gid", 
								"infohash", 
								"status", 
								"errorMessage",
								"completedLength", 
								"totalLength", 
								"bittorrent"
								]
							]
						}`, k, k)
				calls = append(calls, statusCall)
			}
			req, err := http.NewRequest(
				http.MethodPost,
				c.aria2ServerURL,
				bytes.NewBuffer([]byte("["+strings.Join(calls, ",")+"]")),
			)
			if err != nil {
				if c.errHandler != nil {
					c.errHandler.FatalError(err)
				}
				return
			}
			req.Header.Set("Content-Type", "application/json")

			statuses, deleteGid, _ := c.doStatusRequest(req)
			for _, s := range statuses {
				s.OwnerID = gidPerOwner[s.GID]
				c.taskStatusesChan <- s
			}
			for _, g := range deleteGid {
				delete(gidPerOwner, g)
			}
		}
	}
}

func (c *Client) doStatusRequest(req *http.Request) ([]TaskStatus, []string, error) {
	statuses := []TaskStatus{}
	deleteGids := []string{}
	var err error

	res, err := c.httpClient.Do(req)
	if err != nil {
		if c.errHandler != nil {
			c.errHandler.FatalError(err)
		}
		return statuses, deleteGids, err
	}
	defer res.Body.Close()

	var statusesRaw []map[string]json.RawMessage //[]map[string][]TaskStatus
	bodyByt, _ := ioutil.ReadAll(res.Body)
	err = json.Unmarshal(bodyByt, &statusesRaw)
	if err != nil {
		var oneElRaw map[string]json.RawMessage
		err = json.Unmarshal(bodyByt, &oneElRaw)
		if err != nil {
			if c.errHandler != nil {
				c.errHandler.LogError(err)
			}
			return statuses, deleteGids, err
		}
		statusesRaw = []map[string]json.RawMessage{}
		var resultEls []json.RawMessage
		json.Unmarshal(oneElRaw["result"], &resultEls)
		for _, r := range resultEls {
			statusesRaw = append(statusesRaw, map[string]json.RawMessage{
				"result": r,
			})
		}

	}

	for _, s := range statusesRaw {
		if s["error"] != nil {
			var taskID string
			json.Unmarshal(s["id"], &taskID)
			var errorMsg map[string]string
			json.Unmarshal(s["error"], &errorMsg)
			statuses = append(statuses, TaskStatus{
				GID:          taskID,
				Status:       "error",
				ErrorMessage: errorMsg["message"],
			})
			deleteGids = append(deleteGids, taskID)
		}
		if s["result"] != nil {
			var ts TaskStatus
			json.Unmarshal(s["result"], &ts)
			statuses = append(statuses, ts)
			compLen, _ := ts.CompletedLength.Int64()
			totlLen, _ := ts.TotalLength.Int64()
			if ts.Status == "error" ||
				ts.Status == "complete" ||
				ts.Status == "removed" ||
				(compLen != 0 &&
					compLen == totlLen) {
				deleteGids = append(deleteGids, ts.GID)
			}
		}
	}
	return statuses, deleteGids, err
}

type pollingTask struct {
	ownerID string
	gid     string
}

// TaskStatus is an object for reporting the status and some additional metadata
// of a runing aria2 task.
type TaskStatus struct {
	OwnerID         string
	GID             string
	Infohash        string
	Status          string
	ErrorMessage    string
	CompletedLength json.Number
	TotalLength     json.Number
	Bittorrent      bittorrentInfo
}

type bittorrentInfo struct {
	Info struct {
		Name string
	}
}

// NewClient creates new Client from config.
func NewClient(cfg *Config) (*Client, error) {
	if cfg == nil {
		cfg = &Config{}
	}
	if cfg.Aria2RPCURL == "" {
		cfg.Aria2RPCURL = "http://localhost:6800/jsonrpc"
	}
	if cfg.PollingInterval == 0 {
		cfg.PollingInterval = 10
	}
	httpClient := &http.Client{}
	return &Client{
		httpClient,
		cfg.Aria2RPCURL,
		cfg.PollingInterval,
		make(chan pollingTask),
		make(chan TaskStatus),
		nil,
	}, checkConnectivity(httpClient, cfg.Aria2RPCURL)
}

func checkConnectivity(client *http.Client, rpcURL string) error {
	req, err := http.NewRequest(http.MethodPost, rpcURL, bytes.NewBuffer([]byte(`{
		"jsonrpc": "2.0",
		"id": "%s",
		"method": "aria2.getVersion"}`)))
	if err != nil {
		return err
	}
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	ioutil.ReadAll(res.Body)
	return nil
}

func getWorkdir() (dir string) {
	dir, _ = os.Getwd()
	if dir == "" {
		dir, _ = os.UserHomeDir()
	}
	if dir == "" {
		dir = "~/"
	}
	if strings.HasSuffix(dir, "/") == false {
		dir = dir + "/"
	}
	return
}

func doDownloadRequest(c *Client, req *http.Request, ownerID string) (string, error) {
	req.Header.Set("Content-Type", "application/json")

	res, err := c.httpClient.Do(req)
	if err != nil {
		if c.errHandler != nil {
			c.errHandler.LogError(err)
		}
		return "", err
	}
	defer res.Body.Close()
	var resultBody map[string]json.RawMessage
	err = json.NewDecoder(res.Body).Decode(&resultBody)
	if err != nil {
		if c.errHandler != nil {
			c.errHandler.LogError(err)
		}
		return "", err
	}

	if v, ok := resultBody["result"]; ok {
		var gid string
		json.Unmarshal(v, &gid)
		c.AddPollingTask(ownerID, gid)
		return gid, nil
	}
	if v, ok := resultBody["error"]; ok {
		var errorStruct map[string]json.RawMessage
		err = json.Unmarshal(v, &errorStruct)
		if err != nil {
			return "", err
		}
		if errMsg, ok := errorStruct["message"]; ok {
			var em string
			json.Unmarshal(errMsg, &em)
			return "", errors.New(em)
		}
	}
	return "", nil
}

func mustMkdirAll(dir string) error {
	err := os.MkdirAll(dir, os.ModePerm)
	if err == nil || os.IsExist(err) {
		return nil
	}
	return err
}
