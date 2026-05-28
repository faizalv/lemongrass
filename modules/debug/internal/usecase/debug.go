package usecase

import (
	"sync"
	"time"

	ptyclient "github.com/faizalv/lemongrass/modules/pty/client"
)

const systemPrompt = `Lemongrass debug PTY. Direct text invisible -- only hook responses reach user. To respond: invoke #lg!.echo <message> as Bash tool call (# is hook trigger, not a comment). ! means fire-and-forget. One call per message.`

type ptyProvider interface {
	Open(prompt, sessionID, sessionType string) (ptyclient.Session, error)
}

type DebugUsecase struct {
	pty  ptyProvider
	mu   sync.Mutex
	sess ptyclient.Session
}

func New(pty ptyProvider) *DebugUsecase {
	return &DebugUsecase{pty: pty}
}

func (u *DebugUsecase) Send(message string) {
	go func() {
		if message == "exit" {
			u.mu.Lock()
			s := u.sess
			u.sess = nil
			u.mu.Unlock()
			if s != nil {
				s.Close()
			}
			return
		}

		u.mu.Lock()
		sess := u.sess
		u.mu.Unlock()

		if sess == nil {
			newSess, err := u.pty.Open(systemPrompt, "", "debug")
			if err != nil {
				return
			}
			u.mu.Lock()
			if u.sess == nil {
				u.sess = newSess
				sess = newSess
			} else {
				go newSess.Close()
				sess = u.sess
			}
			u.mu.Unlock()
		}

		sess.Write([]byte(message + "\r"))
		time.Sleep(300 * time.Millisecond)
		sess.Write([]byte("\r"))
	}()
}
