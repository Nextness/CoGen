// audit_relay.go owns the bounded background delivery of companion PDF audit
// events into metadata audit while the viewer is serving.
package server

import (
	"context"
	"time"
)

const (
	auditRelayInterval  = 5 * time.Second
	auditRelayMaxEvents = 100
)

// StartAuditRelay starts one bounded relay loop after the server is ready to
// serve. It is idempotent and does nothing when no writable companion exists.
func (s *Server) StartAuditRelay(ctx context.Context) {
	s.auditRelayMu.Lock()
	if s.auditRelayCancel != nil || s.pdfWriteStore == nil || s.writeDB == nil {
		s.auditRelayMu.Unlock()
		return
	}
	relayCtx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	s.auditRelayCancel = cancel
	s.auditRelayDone = done
	s.auditRelayMu.Unlock()
	go s.runAuditRelay(relayCtx, done)
}

// StopAuditRelay stops the relay loop and waits for its current bounded batch
// to finish before callers close the companion or metadata connections.
func (s *Server) StopAuditRelay() {
	s.auditRelayMu.Lock()
	cancel := s.auditRelayCancel
	done := s.auditRelayDone
	s.auditRelayCancel = nil
	s.auditRelayDone = nil
	s.auditRelayMu.Unlock()
	if cancel != nil {
		cancel()
	}
	if done != nil {
		<-done
	}
}

// runAuditRelay replays pending events immediately and then on a fixed interval.
func (s *Server) runAuditRelay(ctx context.Context, done chan struct{}) {
	defer close(done)
	s.relayAuditOnce(ctx)
	ticker := time.NewTicker(auditRelayInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.relayAuditOnce(ctx)
		}
	}
}

// relayAuditOnce delivers one bounded batch and retains only a safe failure message.
func (s *Server) relayAuditOnce(ctx context.Context) {
	if s.pdfWriteStore == nil || s.writeDB == nil {
		return
	}
	_, err := s.pdfWriteStore.RelayAuditOutbox(ctx, s.writeDB.PDFBindings, auditRelayMaxEvents)
	s.auditRelayMu.Lock()
	if err != nil {
		s.auditRelayError = err.Error()
	} else {
		s.auditRelayError = ""
	}
	s.auditRelayMu.Unlock()
}
