package app

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// bitwardenServer лениво поднимает локальный `bw serve`: мастер-пароль
// спрашивается только при первой операции с Bitwarden, а не при запуске.
// Ключ сессии передаётся дочернему процессу только через env и не хранится.
type bitwardenServer struct {
	mu   sync.Mutex
	port string
	send func(tea.Msg)
	proc *exec.Cmd
	done chan struct{}
}

type bitwardenUnlockResult struct {
	session string
	err     error
}

// bitwardenUnlockRequestMsg просит UI отдать терминал под `bw unlock`.
type bitwardenUnlockRequestMsg struct {
	reply chan<- bitwardenUnlockResult
}

type bitwardenUnlockFinishedMsg struct{}

func isBitwardenUnreachable(err error) bool {
	var opErr *net.OpError
	return errors.As(err, &opErr)
}

// start разблокирует хранилище и запускает bw serve, если сервер ещё не
// отвечает. reachable повторно проверяется под мьютексом: параллельная
// операция могла уже поднять сервер.
func (s *bitwardenServer) start(ctx context.Context, reachable func(context.Context) bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.send == nil || s.port == "" {
		return errors.New("Bitwarden недоступен: bw serve не запущен")
	}
	if reachable(ctx) {
		return nil
	}
	reply := make(chan bitwardenUnlockResult, 1)
	s.send(bitwardenUnlockRequestMsg{reply: reply})
	var result bitwardenUnlockResult
	select {
	case result = <-reply:
	case <-ctx.Done():
		return ctx.Err()
	}
	if result.err != nil {
		return result.err
	}
	proc := exec.Command("bw", "serve", "--hostname", "127.0.0.1", "--port", s.port)
	proc.Env = append(os.Environ(), "BW_SESSION="+result.session)
	bindToParent(proc)
	if err := proc.Start(); err != nil {
		return fmt.Errorf("Bitwarden: запуск bw serve: %w", err)
	}
	done := make(chan struct{})
	go func() {
		_ = proc.Wait()
		close(done)
	}()
	s.proc, s.done = proc, done
	for range 50 {
		select {
		case <-done:
			s.proc = nil
			return errors.New("Bitwarden: bw serve завершился сразу после запуска")
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(200 * time.Millisecond):
		}
		if reachable(ctx) {
			return nil
		}
	}
	s.stopLocked()
	return errors.New("Bitwarden: bw serve не ответил за 10 секунд")
}

// close останавливает bw serve, запущенный приложением.
func (s *bitwardenServer) close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.stopLocked()
}

func (s *bitwardenServer) stopLocked() {
	if s.proc == nil {
		return
	}
	_ = s.proc.Process.Kill()
	<-s.done
	s.proc = nil
}

// unlockBitwardenCmd временно отдаёт терминал `bw unlock --raw`: приглашение
// bw пишет в stderr, ключ сессии читается из stdout.
func unlockBitwardenCmd(reply chan<- bitwardenUnlockResult) tea.Cmd {
	var out bytes.Buffer
	cmd := exec.Command("bw", "unlock", "--raw")
	cmd.Stdout = &out
	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		session := strings.TrimSpace(out.String())
		out.Reset()
		if err == nil && session == "" {
			err = errors.New("ключ сессии пуст")
		}
		if err != nil {
			reply <- bitwardenUnlockResult{err: fmt.Errorf("Bitwarden не разблокирован: %w", err)}
		} else {
			reply <- bitwardenUnlockResult{session: session}
		}
		return bitwardenUnlockFinishedMsg{}
	})
}
