package web

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"hadoop-dev/internal/cluster"
	"hadoop-dev/internal/config"
)

// ─── Index ────────────────────────────────────────────────────────────────

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	data, err := staticFS.ReadFile("static/index.html")
	if err != nil {
		http.Error(w, "dashboard not found", 500)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(data)
}

// ─── Cluster Control ──────────────────────────────────────────────────────

func (s *Server) handleStart(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	if s.starting {
		s.mu.Unlock()
		http.Error(w, "already starting", http.StatusConflict)
		return
	}
	s.starting = true
	s.lastErr = ""
	s.mu.Unlock()

	// Reset progress log for fresh start
	s.progressMu.Lock()
	s.progressLog = nil
	s.progressMu.Unlock()

	preset := r.FormValue("preset")
	if preset == "" {
		preset = "minimal"
	}
	dnCount := 2
	fmt.Sscanf(r.FormValue("datanodes"), "%d", &dnCount)
	if dnCount < 1 {
		dnCount = 1
	}

	p, err := cluster.ParsePreset(preset)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	cfg := cluster.Config{
		WorkDir:  s.workDir,
		Preset:   p,
		DNCount:  dnCount,
		NoAttach: true,
	}

	// Build an emitting manager that publishes events to subscribers
	emitting := s.mgr.WithEmitter(func(kind, msg string) {
		s.publishProgress(ProgressEvent{Kind: kind, Msg: msg})
	})

	go func() {
		err := emitting.Start(context.Background(), cfg)
		s.mu.Lock()
		s.starting = false
		if err != nil {
			s.lastErr = err.Error()
			s.publishProgress(ProgressEvent{Kind: "error", Msg: err.Error()})
		}
		s.mu.Unlock()
	}()

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(w, `<div class="toast toast-info">⬀ Starting %s cluster (%d DataNodes)&hellip;<br><small>Watch the <strong>Startup Progress</strong> panel below.</small></div>`, preset, dnCount)
}

func (s *Server) handleStop(w http.ResponseWriter, r *http.Request) {
	if err := s.mgr.Stop(r.Context()); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	s.publishProgress(ProgressEvent{Kind: "warn", Msg: "Cluster stopped by user"})
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, `<div class="toast toast-stop">🛑 Cluster stopped.</div>`)
}

// ─── Startup Progress SSE ────────────────────────────────────────────────

func (s *Server) handleStartProgress(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	if _, ok := w.(http.Flusher); !ok {
		http.Error(w, "streaming unsupported", 500)
		return
	}

	// Get existing log + subscribe for new events
	history, ch := s.subscribeProgress()
	defer s.unsubscribeProgress(ch)

	// Send historical events first
	for _, e := range history {
		sseEvent(w, e)
	}

	// Stream new events until client disconnects or "done"
	for {
		select {
		case <-r.Context().Done():
			return
		case e, ok := <-ch:
			if !ok {
				return
			}
			sseEvent(w, e)
			if e.Kind == "done" || e.Kind == "error" {
				return
			}
		}
	}
}

// ─── Status Cards (HTMX partial) ──────────────────────────────────────────

const cardsTmpl = `
{{range .}}
<div class="container-card state-{{.State}}">
  <span class="status-dot"></span>
  <div class="card-body">
    <p class="card-name">{{.Name}}</p>
    <p class="card-state">{{stateLabel .State}} · {{.Status}}</p>
    <p class="card-image">{{shortImage .Image}}</p>
  </div>
  <div class="card-actions">
    <button class="btn-log"
            hx-get="/api/logs/{{.Name}}"
            hx-target="#log-output"
            hx-swap="innerHTML"
            title="View logs">≡</button>
  </div>
</div>
{{else}}
<div class="no-containers">
  No cluster containers found.<br>
  Hit <strong>Start</strong> to launch the cluster.
</div>
{{end}}`

type containerSummary struct {
	Name   string
	State  string
	Status string
	Image  string
}

func (s *Server) handleStatusCards(w http.ResponseWriter, r *http.Request) {
	containers, err := s.mgr.ListContainers(r.Context())
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	// Add "starting" banner if cluster is booting
	s.mu.RLock()
	starting := s.starting
	lastErr := s.lastErr
	s.mu.RUnlock()

	var summaries []containerSummary
	for _, c := range containers {
		name := strings.TrimPrefix(c.Names[0], "/")
		summaries = append(summaries, containerSummary{
			Name:   name,
			State:  c.State,
			Status: c.Status,
			Image:  c.Image,
		})
	}

	funcMap := template.FuncMap{
		"stateLabel": func(state string) string {
			switch state {
			case "running":
				return "🟢 Running"
			case "restarting", "created":
				return "🟡 Starting"
			default:
				return "🔴 Stopped"
			}
		},
		"shortImage": func(img string) string {
			if idx := strings.LastIndex(img, "/"); idx >= 0 {
				return img[idx+1:]
			}
			return img
		},
	}

	tmpl := template.Must(template.New("cards").Funcs(funcMap).Parse(cardsTmpl))

	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	if starting {
		fmt.Fprint(w, `<div class="starting-banner">⏳ Cluster is starting — containers will appear below…</div>`)
	}
	if lastErr != "" {
		fmt.Fprintf(w, `<div class="error-banner">❌ %s</div>`, lastErr)
	}

	_ = tmpl.Execute(w, summaries)
}

// ─── Log SSE Stream ───────────────────────────────────────────────────────

func (s *Server) handleLogsSSE(w http.ResponseWriter, r *http.Request) {
	service := r.PathValue("service")
	if service == "" {
		service = "hadoop-namenode"
	}
	// Strip leading slash Docker adds to names
	service = strings.TrimPrefix(service, "/")

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", 500)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Minute)
	defer cancel()

	reader, err := s.mgr.LogStream(ctx, service)
	if err != nil {
		fmt.Fprintf(w, "data: error: %s\n\n", err.Error())
		flusher.Flush()
		return
	}
	defer reader.Close()

	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return
		default:
		}
		line := scanner.Text()
		// Docker log multiplexing header is 8 bytes; strip it for display
		if len(line) > 8 {
			line = line[8:]
		}
		line = strings.ReplaceAll(line, "<", "&lt;")
		fmt.Fprintf(w, "data: %s\n\n", line)
		flusher.Flush()
	}
}

// ─── File Manager ─────────────────────────────────────────────────────────

const fileTmpl = `
{{range .}}
<div class="file-row">
  <span class="file-icon">{{if .IsDir}}📁{{else}}📄{{end}}</span>
  <span class="file-name">{{.Name}}</span>
  <span class="file-size">{{.Size}}</span>
  {{if not .IsDir}}
  <button class="btn-hdfs"
          hx-post="/api/hdfs/put"
          hx-vals='{"filename": "{{.Name}}"}'
          hx-target="#hdfs-status"
          hx-swap="innerHTML">→ HDFS</button>
  {{end}}
</div>
{{else}}
<p class="empty-dir">No files yet. Drop files above to add them.</p>
{{end}}`

type fileEntry struct {
	Name  string
	IsDir bool
	Size  string
}

func (s *Server) handleFileList(w http.ResponseWriter, r *http.Request) {
	sharedDir := filepath.Join(s.workDir, "shared")
	entries, err := os.ReadDir(sharedDir)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	var files []fileEntry
	for _, e := range entries {
		info, _ := e.Info()
		size := ""
		if !e.IsDir() && info != nil {
			size = humanBytes(info.Size())
		}
		files = append(files, fileEntry{Name: e.Name(), IsDir: e.IsDir(), Size: size})
	}

	funcMap := template.FuncMap{
		"not": func(b bool) bool { return !b },
	}
	tmpl := template.Must(template.New("files").Funcs(funcMap).Parse(fileTmpl))
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = tmpl.Execute(w, files)
}

func (s *Server) handleUpload(w http.ResponseWriter, r *http.Request) {
	r.ParseMultipartForm(512 << 20) // 512 MB max
	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "no file in request", http.StatusBadRequest)
		return
	}
	defer file.Close()

	sharedDir := filepath.Join(s.workDir, "shared")
	dst, err := os.Create(filepath.Join(sharedDir, filepath.Base(header.Filename)))
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	defer dst.Close()

	if _, err := io.Copy(dst, file); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	// Return refreshed file list
	s.handleFileList(w, r)
}

func (s *Server) handleHDFSPut(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Filename string `json:"filename"`
		HDFSDest string `json:"hdfs_dest"`
	}
	json.NewDecoder(r.Body).Decode(&body)
	if body.Filename == "" {
		body.Filename = r.FormValue("filename")
	}
	if body.HDFSDest == "" {
		body.HDFSDest = "/user/root/input/" + body.Filename
	}

	if body.Filename == "" {
		http.Error(w, "filename required", http.StatusBadRequest)
		return
	}

	src := "/shared/" + filepath.Base(body.Filename)
	cmd := fmt.Sprintf(
		"export HADOOP_HOME=/opt/hadoop-3.2.1 && "+
			"export PATH=${HADOOP_HOME}/bin:${PATH} && "+
			"hdfs dfs -mkdir -p /user/root/input && "+
			"hdfs dfs -put -f %s %s",
		src, body.HDFSDest,
	)

	out, err := s.mgr.Exec(r.Context(), cluster.ContainerNameNode, []string{"bash", "-lc", cmd})
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err != nil {
		fmt.Fprintf(w, `<span class="hdfs-error">❌ %s</span>`, err.Error())
		return
	}
	_ = out
	fmt.Fprintf(w, `<span class="hdfs-ok">✅ %s → %s</span>`, body.Filename, body.HDFSDest)
}

// ─── Helpers ──────────────────────────────────────────────────────────────

func humanBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}

// ─── Config Editor ────────────────────────────────────────────────────────

const configTmpl = `
<form hx-post="/api/config" hx-target="#config-status" hx-swap="innerHTML">
  <div class="config-grid">
    {{range .}}
    <div class="config-row">
      <label class="config-key" title="{{.Key}}">{{.Key}}</label>
      <input class="config-val" type="text" name="{{.Key}}" value="{{.Val}}">
    </div>
    {{end}}
  </div>
  <div class="config-actions">
    <button class="btn btn-start" type="submit">💾 Save &amp; Restart Containers</button>
    <span id="config-status"></span>
  </div>
</form>`

type configEntry struct{ Key, Val string }

func (s *Server) handleConfigGet(w http.ResponseWriter, r *http.Request) {
	envPath := filepath.Join(s.workDir, "hadoop.env")
	envs, err := config.ParseHadoopEnvRaw(envPath)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	var entries []configEntry
	for _, line := range envs {
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			entries = append(entries, configEntry{Key: parts[0], Val: parts[1]})
		}
	}
	tmpl := template.Must(template.New("cfg").Parse(configTmpl))
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = tmpl.Execute(w, entries)
}

func (s *Server) handleConfigSave(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	envPath := filepath.Join(s.workDir, "hadoop.env")

	// Read existing lines to preserve comments and ordering
	envs, _ := config.ParseHadoopEnvRaw(envPath)

	// Build updated map from form
	updates := map[string]string{}
	for k, v := range r.Form {
		if len(v) > 0 {
			updates[k] = v[0]
		}
	}

	// Write back, preserving comments
	var lines []string
	seen := map[string]bool{}
	for _, line := range envs {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			lines = append(lines, line)
			continue
		}
		parts := strings.SplitN(trimmed, "=", 2)
		if len(parts) == 2 {
			key := parts[0]
			seen[key] = true
			if newVal, ok := updates[key]; ok {
				lines = append(lines, key+"="+newVal)
			} else {
				lines = append(lines, line)
			}
		}
	}
	// Append any new keys from the form that weren't in the file
	for k, v := range updates {
		if !seen[k] {
			lines = append(lines, k+"="+v)
		}
	}

	content := strings.Join(lines, "\n") + "\n"
	if err := os.WriteFile(envPath, []byte(content), 0o644); err != nil {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, `<span class="hdfs-error">❌ %s</span>`, err.Error())
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, `<span class="hdfs-ok">✅ Saved. Restart cluster to apply.</span>`)
}
