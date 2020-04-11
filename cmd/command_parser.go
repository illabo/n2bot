package main

import (
	"fmt"
	"regexp"
)

type botTask struct {
	TellActive bool
	DlType     string
	DlSubdir   string
	KillGID    string
	Magnet     string
}
type callbackTask struct {
	DlType     string
	GID        string
	CallbackID string
}

// ParseIncomingMessage gets all the known to the bot flags from provided text.
// Returns *botTask with the values of parsed flags.
func ParseIncomingMessage(text string) *botTask {
	return &botTask{
		flagMatcher(text,
			"-a", "—tellactive", "--tellactive", "—tell-active", "--tell-active",
		),
		keyMatcher(text, "-t=", "-t:", "—type", "--type"),
		keyMatcher(text, "-d=", "-d:", "—dir", "--dir"),
		keyMatcher(text, "-k=", "-k:", "—kill", "--kill"),
		func() string {
			re := regexp.MustCompile(`magnet:\?\S+`)
			return fmt.Sprintf("%s", re.Find([]byte(text)))
		}(),
	}
}

// ParseCallbackQuery gets all the known to the bot flags for callback query from provided text.
// Returns *callbackTask with the values of parsed flags.
func ParseCallbackQuery(text string) *callbackTask {
	return &callbackTask{
		keyMatcher(text, "-t="),
		keyMatcher(text, "-gid="),
		keyMatcher(text, "-query_id="),
	}
}

func keyMatcher(text string, keys ...string) string {
	for _, k := range keys {
		re := regexp.MustCompile(
			fmt.Sprintf(
				`(^|\s)%s\s?[“”‘’‛‟„'"′″]?([^<>:;“”‘’‛‟„'"′″\/\\|\?\*\+%%=\s]+)[“”‘’‛‟„'"′″]?($|\s)`,
				k,
			),
		)
		smch := re.FindSubmatch([]byte(text))
		if len(smch) > 2 {
			return fmt.Sprintf("%s", smch[2])
		}
	}
	return ""
}

func flagMatcher(text string, keys ...string) bool {
	for _, k := range keys {
		re := regexp.MustCompile(
			fmt.Sprintf(
				`(^|\s)%s($|\s)`,
				k,
			),
		)
		smch := re.FindSubmatch([]byte(text))
		if len(smch) > 2 {
			return true
		}
	}
	return false
}
