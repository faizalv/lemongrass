package usecase

import (
	"sync"
	"time"

	"github.com/faizalv/lemongrass/modules/lg/entity"
	ptyclient "github.com/faizalv/lemongrass/modules/pty/client"
)

const debugSystemPrompt = `You are inside Lemongrass. Your only way to communicate is by calling Bash with the command #lg.echo followed by your message. For example, to say hello you would call Bash with the exact text: #lg.echo hello there. Do not write anything directly to the user. Only use #lg.echo. Do not use any other bash command. You must call #lg.echo exactly once with your complete response. Do not split your response across multiple calls.`

type LgUsecase struct {
	pty   *ptyclient.PtyClient
	mu    sync.Mutex
	calls []entity.Call
}

func New(pty *ptyclient.PtyClient) *LgUsecase {
	return &LgUsecase{pty: pty}
}

func (u *LgUsecase) RecordCall(cmd, args string) {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.calls = append(u.calls, entity.Call{Cmd: cmd, Args: args, Timestamp: time.Now()})
	if len(u.calls) > 200 {
		u.calls = u.calls[len(u.calls)-200:]
	}
}

func (u *LgUsecase) ListCalls() []entity.Call {
	u.mu.Lock()
	defer u.mu.Unlock()
	result := make([]entity.Call, len(u.calls))
	copy(result, u.calls)
	return result
}

func (u *LgUsecase) Send(message string) {
	prompt := debugSystemPrompt + "\n\nThe user says: " + message
	go u.pty.Run(prompt)
}
