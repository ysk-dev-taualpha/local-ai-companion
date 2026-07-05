package pythonservice

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/ysk-dev-taualpha/local-ai-companion/runtime/internal/config"
)

type Logger interface {
	Info(format string, v ...interface{})
	Error(format string, v ...interface{})
}

type Service struct {
	cfg    config.PythonServiceConfig
	logger Logger

	mu     sync.Mutex
	cmd    *exec.Cmd
	waitCh chan error
}

func New(cfg config.PythonServiceConfig, logger Logger) *Service {
	return &Service{cfg: cfg, logger: logger}
}

func (s *Service) Start(ctx context.Context) error {
	if strings.TrimSpace(s.cfg.Command) == "" {
		if s.logger != nil {
			s.logger.Info("Python AI Service command is empty; using external service at %s", s.cfg.BaseURL)
		}
		return nil
	}

	cmd := shellCommand(s.cfg.Command)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if s.logger != nil {
		s.logger.Info("starting Python AI Service: %s", s.cfg.Command)
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start python service: %w", err)
	}

	s.mu.Lock()
	s.cmd = cmd
	s.waitCh = make(chan error, 1)
	waitCh := s.waitCh
	s.mu.Unlock()

	go func() {
		waitCh <- cmd.Wait()
	}()

	if err := s.waitReady(ctx); err != nil {
		stopCtx, cancel := context.WithTimeout(context.Background(), s.shutdownTimeout())
		defer cancel()
		_ = s.Stop(stopCtx)
		return err
	}
	if s.logger != nil {
		s.logger.Info("Python AI Service ready at %s", s.cfg.BaseURL)
	}
	return nil
}

func (s *Service) Stop(ctx context.Context) error {
	s.mu.Lock()
	cmd := s.cmd
	waitCh := s.waitCh
	s.cmd = nil
	s.waitCh = nil
	s.mu.Unlock()

	if cmd == nil || cmd.Process == nil {
		return nil
	}

	if s.logger != nil {
		s.logger.Info("stopping Python AI Service")
	}
	if err := signalProcess(cmd, os.Interrupt); err != nil {
		_ = killProcess(cmd)
	}

	select {
	case <-waitCh:
		return nil
	case <-ctx.Done():
		_ = killProcess(cmd)
		select {
		case <-waitCh:
			return nil
		case <-time.After(time.Second):
			return ctx.Err()
		}
	}
}

func (s *Service) waitReady(ctx context.Context) error {
	deadline := time.Now().Add(s.readyTimeout())
	client := &http.Client{Timeout: 500 * time.Millisecond}

	for {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.cfg.BaseURL, nil)
		if err != nil {
			return fmt.Errorf("build python service readiness request: %w", err)
		}
		resp, err := client.Do(req)
		if err == nil {
			resp.Body.Close()
			return nil
		}

		s.mu.Lock()
		waitCh := s.waitCh
		s.mu.Unlock()
		select {
		case waitErr := <-waitCh:
			s.mu.Lock()
			if s.waitCh == waitCh {
				s.cmd = nil
				s.waitCh = nil
			}
			s.mu.Unlock()
			if waitErr == nil {
				waitErr = fmt.Errorf("process exited before readiness")
			}
			return fmt.Errorf("python service exited before readiness: %w", waitErr)
		default:
		}

		if time.Now().After(deadline) {
			return fmt.Errorf("python service did not become ready at %s within %s", s.cfg.BaseURL, s.readyTimeout())
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}
}

func (s *Service) readyTimeout() time.Duration {
	if s.cfg.ReadyTimeoutMs <= 0 {
		return 10 * time.Second
	}
	return time.Duration(s.cfg.ReadyTimeoutMs) * time.Millisecond
}

func (s *Service) shutdownTimeout() time.Duration {
	if s.cfg.ShutdownTimeoutMs <= 0 {
		return 5 * time.Second
	}
	return time.Duration(s.cfg.ShutdownTimeoutMs) * time.Millisecond
}
