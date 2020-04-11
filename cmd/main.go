package main

import (
	"log"
	"n2bot/ariactr"
	"n2bot/classr"
	"n2bot/fatalist"
	"n2bot/proxyurl"
	"n2bot/storage"
	"n2bot/tg"

	"github.com/BurntSushi/toml"
)

func main() {
	var err error
	var cfg config

	_, err = toml.DecodeFile("config.toml", &cfg)
	if err != nil {
		log.Fatal(err)
	}

	tc := tg.NewClient(&cfg.TgClientConfig)
	ac, err := ariactr.NewClient(&cfg.AriaConfig)
	if err != nil {
		log.Fatal(err)
	}
	if cfg.ProxyConfig.ProviderAPIURLTemplate != "" {
		tc.SetProxy(proxyurl.NewRandomProxy(&cfg.ProxyConfig))
	}
	db, err := storage.NewInstance(&cfg.StorageConfig)
	if err != nil {
		log.Fatal(err)
	}
	cc := classr.NewClient(&cfg.ClassrConfig)

	fatal := fatalist.New()
	app := application{
		tgClient:     tc,
		ariaClient:   ac,
		classrClient: cc,
		db:           db,
		dirs:         &cfg.Dirs,
		errHandler:   &fatal,
		confThold:    cfg.ConfThold,
		users:        cfg.Users,
	}

	tc.SetErrorHandler(&fatal)
	ac.SetErrorHandler(&fatal)
	cc.SetErrorHandler(&fatal)

	tc.Run(
		func(msg tg.ChatMessage) {
			handleNewIncomingTask(&msg, &app)
		},
	)
	ac.Run(
		func(ts ariactr.TaskStatus) {
			handleAriaUpdates(&ts, &app)
		},
	)

	if err = pollSavedTasks(&app); err != nil {
		log.Fatal(err)
	}

	logChan := fatal.GetLogChan()
	fatalChan := fatal.GetFatalChan()
	for {
		select {
		case e := <-logChan:
			log.Println(e)
		case e := <-fatalChan:
			log.Fatal(e)
		}
	}
}
