package main

import (
	"n2bot/ariactr"
	"n2bot/classr"
	"n2bot/fatalist"
	"n2bot/proxyurl"
	"n2bot/storage"
	"n2bot/tg"
)

type application struct {
	tgClient     *tg.Client
	ariaClient   *ariactr.Client
	classrClient *classr.Client
	db           storage.DBInstancer
	dirs         *downloadDirectories
	errHandler   *fatalist.Fatalist
	confThold    uint8
	users        []string
}

type config struct {
	ConfThold      uint8
	Users          []string
	Dirs           downloadDirectories `toml:"downloadDirectories"`
	TgClientConfig tg.Config           `toml:"tgClient"`
	ProxyConfig    proxyurl.Config
	AriaConfig     ariactr.Config `toml:"ariaClient"`
	ClassrConfig   classr.Config  `toml:"classificator"`
	StorageConfig  storage.Config
}

type downloadTaskInfo struct {
	TaskStage  taskStage
	MagnetHash string
	DLDir      string
	DLType     downloadType
	BTName     string
}

type downloadDirectories struct {
	Movies  string
	Series  string
	General string
}

type taskStage byte

const (
	stageMagnetMeta taskStage = iota
	stageBTDownload
	stageSeeding
)

type downloadType byte

func (t downloadType) String() string {
	switch t {
	case unknown:
		return "unknown"
	case series:
		return "series"
	case movies:
		return "movies"
	case common:
		return "common"
	default:
		return "error"
	}
}

// func (t *downloadType) UnmarshalJSON(b []byte) error {
// 	var val string
// 	err := json.Unmarshal(b, &val)
// 	if err != nil {
// 		return err
// 	}
// 	*t = stringToDlType(val)
// 	return nil
// }

// func (t *downloadType) MarshalJSON(b []byte) ([]byte, error) {
// 	return []byte(t.String()), nil
// }

const (
	unknown downloadType = iota
	series
	movies
	common
)
