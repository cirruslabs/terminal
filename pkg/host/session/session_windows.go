package session

import "time"

type Session struct {}

func (session *Session) LastActivity() time.Time {
	return time.Time{}
}
