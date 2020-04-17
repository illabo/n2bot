package main

import (
	"encoding/json"
	"fmt"
	"n2bot/ariactr"
	"n2bot/classr"
	"n2bot/storage"
	"n2bot/tg"
	"net/url"
	"os"
	"regexp"
	"strings"
)

func handleNewIncomingTask(msg *tg.ChatMessage, app *application) {
	authorized := false
	ariaClt := app.ariaClient
	tgClt := app.tgClient
	db := app.db
	for _, usr := range app.users {
		if usr == msg.ChatID {
			authorized = true
		}
	}
	if authorized == false {
		tgClt.GetOutChan() <- tg.NewTextMessage(
			msg.ChatID,
			"You aren't my master, bag-o-flesh!",
		)
		return
	}
	if msg.Type == tg.MessageTypeFromString("callback") {
		handleCallback(msg, app)
		return
	}
	task := ParseIncomingMessage(msg.Text)
	if task.KillGID != "" {
		handleKillTask(msg.ChatID, task.KillGID, app)
		if task.Magnet == "" {
			return
		}
	}
	if task.TellActive {
		handleTellActive(msg.ChatID, app)
		if task.Magnet == "" {
			return
		}
	}
	if task.Magnet == "" {
		tgClt.GetOutChan() <- tg.NewTextMessage(
			msg.ChatID,
			"Gimme 🧲!",
		)
		return
	}
	gid, err := ariaClt.EnqueueMetadata(msg.ChatID, task.Magnet)
	if err != nil {
		tgClt.GetOutChan() <- tg.NewTextMessage(
			msg.ChatID,
			err.Error(),
		)
		return
	}
	dlTask := prepareMagnetInfo(task)
	err = saveNewTask(msg.ChatID, gid, &dlTask, db)
	if err != nil {
		tgClt.GetOutChan() <- tg.NewTextMessage(
			msg.ChatID,
			err.Error(),
		)
	}
}

func handleAriaUpdates(statusUpd *ariactr.TaskStatus, app *application) {
	tgClt := app.tgClient
	db := app.db
	infos, _ := getTaskInfosByUser(statusUpd.OwnerID, db)
	dInfo, ok := infos[statusUpd.GID]
	if ok == false {
		// tgClt.GetOutChan() <- tg.NewTextMessage(
		// 	statusUpd.OwnerID,
		// 	"😮Something stupid happened to one of your downloads.",
		// )
		return
	}

	status := statusUpd.Status
	if status == "error" {
		tgClt.GetOutChan() <- tg.NewTextMessage(
			statusUpd.OwnerID,
			fmt.Sprintf("Download of '%s' is failed! %s",
				dInfo.BTName,
				statusUpd.ErrorMessage,
			),
		)
		deleteTaskInfo(statusUpd.OwnerID, statusUpd.GID, db)
		return
	}
	if status == "removed" {
		app.tgClient.GetOutChan() <- tg.NewTextMessage(
			statusUpd.OwnerID,
			fmt.Sprintf("Task with GID %s removed.", statusUpd.GID),
		)
		deleteTaskInfo(statusUpd.OwnerID, statusUpd.GID, db)
		return
	}
	compLen, _ := statusUpd.CompletedLength.Int64()
	totlLen, _ := statusUpd.TotalLength.Int64()
	if dInfo.TaskStage == stageMagnetMeta {
		if status == "active" {
			tgClt.GetOutChan() <- tg.NewTyping(statusUpd.OwnerID)
		}
		if status == "complete" || (compLen != 0 && compLen == totlLen) {
			handleMagnetCompletion(&dInfo, statusUpd, app)
		}
	}

	if dInfo.TaskStage == stageBTDownload {
		if status == "active" &&
			statusUpd.Bittorrent.Info.Name != "" &&
			dInfo.BTName != statusUpd.Bittorrent.Info.Name {
			dInfo.BTName = statusUpd.Bittorrent.Info.Name
			saveNewTask(statusUpd.OwnerID, statusUpd.GID, &dInfo, db)
		}
		if status == "complete" || (compLen != 0 && compLen == totlLen) {
			if statusUpd.Bittorrent.Info.Name != "" {
				tgClt.GetOutChan() <- tg.NewTextMessage(
					statusUpd.OwnerID,
					fmt.Sprintf("Download of '%s' to '%s' category is complete!",
						statusUpd.Bittorrent.Info.Name,
						dInfo.DLType.String()),
				)
			}
			dInfo.TaskStage = stageSeeding
			saveNewTask(statusUpd.OwnerID, statusUpd.GID, &dInfo, db)
		}
	}

	if dInfo.TaskStage == stageSeeding && status == "complete" {
		deleteTaskInfo(statusUpd.OwnerID, statusUpd.GID, db)
	}
}

func handleMagnetCompletion(dInfo *downloadTaskInfo, statusUpd *ariactr.TaskStatus, app *application) {
	tgClt := app.tgClient
	confTh := func() uint8 {
		if app.confThold > 100 {
			return 100
		}
		return app.confThold
	}()
	torrentFilename := strings.ToLower(dInfo.MagnetHash) + ".torrent"
	if dInfo.DLType == unknown {
		out, err := dlCategoryByTorrent(app.classrClient, torrentFilename) // ask script for some ML magic
		if err != nil || uint8(out.Confidence*100) < confTh {
			app.errHandler.LogError(err)
			tgClt.GetOutChan() <- tg.NewTextWithKeyboard(
				statusUpd.OwnerID,
				fmt.Sprintf("I'm not sure about category of '%s'. Could you please select it yourself?", dInfo.BTName),
				[]tg.InlineButton{
					{
						Text:         "series",
						CallbackData: fmt.Sprintf("-t=series -gid=%s", statusUpd.GID),
					},
					{
						Text:         "movies",
						CallbackData: fmt.Sprintf("-t=movies -gid=%s", statusUpd.GID),
					},
				},
			)
			return
		}
		dInfo.DLType = stringToDlType(out.Type)
		tgClt.GetOutChan() <- tg.NewTextMessage(
			statusUpd.OwnerID,
			fmt.Sprintf("Download category of '%s' is '%s', I'm %d%% sure ",
				dInfo.BTName,
				out.Type,
				int(out.Confidence*100)),
		)
	}
	startBTDownload(dInfo, statusUpd.OwnerID, statusUpd.GID, app)
}

func startBTDownload(dInfo *downloadTaskInfo, owner, gid string, app *application) {
	ariaClt := app.ariaClient
	tgClt := app.tgClient
	db := app.db
	dlDirs := app.dirs
	torrentFilename := strings.ToLower(dInfo.MagnetHash) + ".torrent"

	err := deleteTaskInfo(owner, gid, db)
	fullPath := fullDlPath(dInfo.DLType, dInfo.DLDir, dlDirs)
	newGid, err := ariaClt.EnqueueBT(owner, fullPath, torrentFilename)
	if err != nil {
		app.errHandler.LogError(err)
		tgClt.GetOutChan() <- tg.NewTextMessage(
			owner,
			err.Error(),
		)
		return
	}
	dInfo.TaskStage = stageBTDownload
	err = saveNewTask(owner, newGid, dInfo, db)
	if err != nil {
		app.errHandler.LogError(err)
		tgClt.GetOutChan() <- tg.NewTextMessage(
			owner,
			err.Error(),
		)
		return
	}
	tgClt.GetOutChan() <- tg.NewTextMessage(
		owner,
		fmt.Sprintf("Download of '%s' to '%s' category started.", dInfo.BTName, dInfo.DLType.String()),
	)
}

func handleCallback(msg *tg.ChatMessage, app *application) {
	cbTask := ParseCallbackQuery(msg.Text)
	app.tgClient.GetOutChan() <- tg.NewQueryAnswer(
		cbTask.CallbackID,
	)
	infos, err := getTaskInfosByUser(msg.ChatID, app.db)
	if err != nil {
		app.tgClient.GetOutChan() <- tg.NewTextMessage(
			msg.ChatID,
			err.Error(),
		)
		return
	}
	dInfo := infos[cbTask.GID]
	dInfo.DLType = stringToDlType(cbTask.DlType)
	startBTDownload(&dInfo, msg.ChatID, cbTask.GID, app)
}

func handleKillTask(chatID, gid string, app *application) {
	tasks, err := getTaskInfosByUser(chatID, app.db)
	if err != nil {
		app.tgClient.GetOutChan() <- tg.NewTextMessage(
			chatID,
			err.Error(),
		)
		return
	}
	if _, ok := tasks[gid]; ok == false {
		app.tgClient.GetOutChan() <- tg.NewTextMessage(
			chatID,
			fmt.Sprintf("You have no tasks with ID %s.", gid),
		)
		return
	}
	err = app.ariaClient.KillTask(gid)
	if err != nil {
		app.tgClient.GetOutChan() <- tg.NewTextMessage(
			chatID,
			err.Error(),
		)
		return
	}
}

func handleTellActive(chatID string, app *application) {
	statuses, err := app.ariaClient.TellActive()
	if err != nil {
		app.tgClient.GetOutChan() <- tg.NewTextMessage(
			chatID,
			err.Error(),
		)
		return
	}
	var message string
	for _, s := range statuses {
		gid := s.GID
		name := s.Bittorrent.Info.Name
		if name == "" {
			name = s.Infohash
		}
		if len(name) > 50 {
			name = name[:50]
		}
		compPerc := "completeness unknown"
		compLen, compErr := s.CompletedLength.Int64()
		totlLen, totlErr := s.TotalLength.Int64()
		if compErr == nil && totlErr == nil && totlLen != 0 {
			compPerc = fmt.Sprintf("downloaded %d%%", 100*compLen/totlLen)
		}
		message = fmt.Sprintf("%sGID: %s\t%s\t%s\n\n", message, gid, name, compPerc)
	}
	if message == "" {
		message = "No active tasks."
	}
	app.tgClient.GetOutChan() <- tg.NewTextMessage(
		chatID,
		message,
	)
}

func pollSavedTasks(app *application) error {
	data, err := app.db.GetAll()
	if err != nil {
		return err
	}
	for userID, v := range data {
		var dlTaskInfos map[string]downloadTaskInfo
		err = json.Unmarshal(v, &dlTaskInfos)
		if err != nil {
			return err
		}
		for gid := range dlTaskInfos {
			app.ariaClient.AddPollingTask(userID, gid)
		}
	}
	return nil
}

func prepareMagnetInfo(task *botTask) downloadTaskInfo {
	return downloadTaskInfo{
		stageMagnetMeta,
		hashFromMagnetLink(task.Magnet),
		task.DlSubdir,
		stringToDlType(task.DlType),
		nameFromMagnetLink(task.Magnet),
	}
}

func saveNewTask(chatID, gid string, task *downloadTaskInfo, db storage.DBInstancer) error {
	dlTaskInfos, err := getTaskInfosByUser(chatID, db)
	if err != nil {
		return err
	}
	dlTaskInfos[gid] = *task
	newVal, err := json.Marshal(dlTaskInfos)
	if err != nil {
		return err
	}
	return db.Set([]byte(chatID), newVal)
}

func getTaskInfosByUser(chatID string, db storage.DBInstancer) (map[string]downloadTaskInfo, error) {
	v, err := db.Get([]byte(chatID))
	if err != nil {
		return nil, err
	}
	var dlTaskInfos map[string]downloadTaskInfo
	err = json.Unmarshal(v, &dlTaskInfos)
	if err != nil {
		dlTaskInfos = map[string]downloadTaskInfo{}
	}

	return dlTaskInfos, nil
}

func deleteTaskInfo(chatID, gid string, db storage.DBInstancer) error {
	dlTaskInfos, err := getTaskInfosByUser(chatID, db)
	if err != nil {
		return err
	}
	delete(dlTaskInfos, gid)
	v, err := json.Marshal(dlTaskInfos)
	if err != nil {
		return err
	}
	err = db.Set([]byte(chatID), v)
	return err
}

func dlCategoryByTorrent(c *classr.Client, file string) (classr.TypePrediction, error) {
	var prediction classr.TypePrediction

	wd, err := os.Getwd()
	if err != nil {
		return prediction, err
	}
	// cmd := exec.Command("./main.py", fmt.Sprintf("%s/%s", wd, file))
	// cmd.Dir = fmt.Sprintf("%s/classificator/", wd)
	// out, err := cmd.Output()
	// if err != nil {
	// 	return
	// }
	// // Output isn't clean for some reason, there are some garbage bytes in the begining.
	// // Have to clean it forcibly.
	// re := regexp.MustCompile(`\{["',\.\s\w:]+\}`)
	// cleanOut := re.Find(out)
	// err = json.Unmarshal(cleanOut, &prediction)
	path := fmt.Sprintf("%s/%s", wd, file)
	prediction, err = c.PredictClass(path)

	return prediction, err
}

func fullDlPath(dlType downloadType, dir string, dirConfig *downloadDirectories) string {
	switch dlType {
	case series:
		return appendSlash(dirConfig.Series) + dir
	case movies:
		return appendSlash(dirConfig.Movies) + dir
	default:
		return appendSlash(dirConfig.General) + dir
	}
}

func appendSlash(pathStr string) string {
	if strings.HasSuffix(pathStr, "/") {
		return pathStr
	}
	return pathStr + "/"
}

func stringToDlType(s string) downloadType {
	switch strings.ToLower(s) {
	case "series", "tv", "show":
		return series
	case "movies", "film", "kino":
		return movies
	case "common", "general", "all":
		return common
	default:
		return unknown
	}
}

func hashFromMagnetLink(magnet string) string {
	re := regexp.MustCompile(`(\w+)($|&)`)
	result := re.FindSubmatch([]byte(magnet))
	if len(result) > 1 {
		return fmt.Sprintf("%s", result[1])
	}
	return ""
}

func nameFromMagnetLink(magnet string) string {
	vals, err := url.ParseQuery(magnet)
	if err != nil {
		return ""
	}
	dn := vals["dn"]
	if len(dn) > 0 {
		return strings.Join(dn, " ")
	}
	return ""
}
