package web

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"hadoop-dev/internal/cluster"
)

//go:embed static
var staticFS embed.FS

// ProgressEvent represents a single startup step.
type ProgressEvent struct {
	Kind string `json:"kind"` // step | ok | warn | error | done
	Msg  string `json:"msg"`
}

// Server holds the web server and cluster state.
type Server struct {
	workDir string
	port    int
	mgr     *cluster.Manager

	// Cluster startup state
	mu       sync.RWMutex
	starting bool
	lastErr  string

	// Progress pub/sub for SSE
	progressMu  sync.Mutex
	progressLog []ProgressEvent
	progressSub []chan ProgressEvent

	mux *http.ServeMux
}

func NewServer(workDir string, port int) (*Server, error) {
	mgr, err := cluster.NewManager()
	if err != nil {
		return nil, fmt.Errorf("connect to Docker: %w", err)
	}
	s := &Server{workDir: workDir, port: port, mgr: mgr}
	s.mux = http.NewServeMux()
	s.routes()
	return s, nil
}

func (s *Server) Run(ctx context.Context) error {
	addr := fmt.Sprintf(":%d", s.port)
	srv := &http.Server{Addr: addr, Handler: s.mux}

	go func() {
		time.Sleep(300 * time.Millisecond)
		openBrowser(fmt.Sprintf("http://localhost:%d", s.port))
	}()

	fmt.Printf("\n🌐 Dashboard → http://localhost:%d\n", s.port)
	fmt.Println("   Ctrl+C to stop.")

	go func() {
		<-ctx.Done()
		shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutCtx)
		s.mgr.Close()
	}()

	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}

func (s *Server) routes() {
	s.mux.Handle("GET /static/", http.FileServer(http.FS(staticFS)))
	s.mux.HandleFunc("GET /", s.handleIndex)

	s.mux.HandleFunc("POST /api/start", s.handleStart)
	s.mux.HandleFunc("POST /api/stop", s.handleStop)
	s.mux.HandleFunc("GET /api/status/cards", s.handleStatusCards)
	s.mux.HandleFunc("GET /api/start/progress", s.handleStartProgress)

	s.mux.HandleFunc("GET /api/logs/{service}", s.handleLogsSSE)

	s.mux.HandleFunc("GET /api/files", s.handleFileList)
	s.mux.HandleFunc("POST /api/files/upload", s.handleUpload)
	s.mux.HandleFunc("POST /api/hdfs/put", s.handleHDFSPut)

	s.mux.HandleFunc("GET /api/config", s.handleConfigGet)
	s.mux.HandleFunc("POST /api/config", s.handleConfigSave)
}

// ─── Progress pub/sub ─────────────────────────────────────────────────────

func (s *Server) publishProgress(e ProgressEvent) {
	s.progressMu.Lock()
	defer s.progressMu.Unlock()
	s.progressLog = append(s.progressLog, e)
	for _, ch := range s.progressSub {
		select {
		case ch <- e:
		default:
		}
	}
}

// subscribeProgress returns the full existing log plus a live channel.
func (s *Server) subscribeProgress() ([]ProgressEvent, chan ProgressEvent) {
	s.progressMu.Lock()
	defer s.progressMu.Unlock()
	ch := make(chan ProgressEvent, 64)
	s.progressSub = append(s.progressSub, ch)
	snap := make([]ProgressEvent, len(s.progressLog))
	copy(snap, s.progressLog)
	return snap, ch
}

func (s *Server) unsubscribeProgress(ch chan ProgressEvent) {
	s.progressMu.Lock()
	defer s.progressMu.Unlock()
	for i, sub := range s.progressSub {
		if sub == ch {
			s.progressSub = append(s.progressSub[:i], s.progressSub[i+1:]...)
			close(ch)
			return
		}
	}
}

// sseEvent writes a single SSE data line.
func sseEvent(w http.ResponseWriter, v any) {
	data, _ := json.Marshal(v)
	fmt.Fprintf(w, "data: %s\n\n", data)
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
}
