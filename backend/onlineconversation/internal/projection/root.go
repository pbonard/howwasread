package projection

import (
	"time"
)

type Detail struct {
	Novel, ShortStory, Poem, Play, Film, WrittenBy, Rule         string
	Capacity, LengthMinutes                                      int
	Time                                                         time.Time
	UpdatedAt                                                    *time.Time // nil until the conversation is updated
	IsModerator, IsRegistrant, IsBanned, IsNotificationScheduled bool
}

type Contents struct {
	Novel, ShortStory, Poem, Play, Film, WrittenBy, Rule string
}

type ReportTarget struct {
	Contents
	UpdatedAt   *time.Time // nil until the conversation is updated
	IsEvaluated bool
}
