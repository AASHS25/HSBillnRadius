// Package radiusserver is a minimal UDP RADIUS server primitive: it owns the
// socket and a bounded worker pool, and delegates packet handling (parse,
// decide, encode) to a PacketHandler. Bounding the workers prevents a goroutine
// explosion under a request storm; a panic in one handler never kills the loop.
package radiusserver

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"time"
)

// maxPacketSize is the RFC 2865 maximum RADIUS packet length.
const maxPacketSize = 4096

// PacketHandler processes one raw datagram and returns the bytes to reply with.
// Returning nil bytes (no error) means "drop silently" (e.g. unknown NAS).
type PacketHandler func(ctx context.Context, remote *net.UDPAddr, raw []byte) ([]byte, error)

// Server reads datagrams on a UDP address and dispatches them to a worker pool.
type Server struct {
	addr    string
	workers int
	timeout time.Duration
	handler PacketHandler
	log     *slog.Logger
}

// New builds a Server. workers bounds concurrent handlers; timeout bounds each.
func New(addr string, workers int, timeout time.Duration, handler PacketHandler, log *slog.Logger) *Server {
	if workers <= 0 {
		workers = 1
	}
	return &Server{addr: addr, workers: workers, timeout: timeout, handler: handler, log: log}
}

type job struct {
	remote *net.UDPAddr
	raw    []byte
}

// Run binds the socket, starts the worker pool and serves until ctx is done.
func (s *Server) Run(ctx context.Context) error {
	udpAddr, err := net.ResolveUDPAddr("udp", s.addr)
	if err != nil {
		return fmt.Errorf("resolve %q: %w", s.addr, err)
	}
	conn, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		return fmt.Errorf("listen %q: %w", s.addr, err)
	}

	jobs := make(chan job, s.workers*2)
	done := make(chan struct{})
	for i := 0; i < s.workers; i++ {
		go s.worker(ctx, conn, jobs)
	}

	// Closing the connection on shutdown unblocks ReadFromUDP.
	go func() {
		<-ctx.Done()
		_ = conn.Close()
		close(done)
	}()

	s.log.Info("radius server listening", slog.String("addr", s.addr), slog.Int("workers", s.workers))

	buf := make([]byte, maxPacketSize)
	for {
		n, remote, err := conn.ReadFromUDP(buf)
		if err != nil {
			select {
			case <-ctx.Done():
				close(jobs)
				<-done
				return nil
			default:
				s.log.Warn("radius read error", slog.Any("error", err))
				continue
			}
		}
		raw := make([]byte, n)
		copy(raw, buf[:n])
		select {
		case jobs <- job{remote: remote, raw: raw}:
		default:
			// Pool saturated: drop rather than block the read loop.
			s.log.Warn("radius worker pool saturated, dropping packet", slog.String("remote", remote.String()))
		}
	}
}

// worker processes jobs until the channel closes, recovering from panics.
func (s *Server) worker(ctx context.Context, conn *net.UDPConn, jobs <-chan job) {
	for j := range jobs {
		s.handle(ctx, conn, j)
	}
}

func (s *Server) handle(ctx context.Context, conn *net.UDPConn, j job) {
	defer func() {
		if p := recover(); p != nil {
			s.log.Error("radius handler panic recovered",
				slog.Any("panic", p), slog.String("remote", j.remote.String()))
		}
	}()

	reqCtx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	reply, err := s.handler(reqCtx, j.remote, j.raw)
	if err != nil {
		s.log.Warn("radius handler error", slog.Any("error", err), slog.String("remote", j.remote.String()))
		return
	}
	if reply == nil {
		return // silent drop
	}
	if _, err := conn.WriteToUDP(reply, j.remote); err != nil && !errors.Is(err, net.ErrClosed) {
		s.log.Warn("radius write error", slog.Any("error", err), slog.String("remote", j.remote.String()))
	}
}
