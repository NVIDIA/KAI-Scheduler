package main

import (
	"context"
	"embed"
	"encoding/json"
	"flag"
	"io/fs"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	kaischedulerclientset "github.com/NVIDIA/KAI-scheduler/pkg/apis/client/clientset/versioned"
	snapshotplugin "github.com/NVIDIA/KAI-scheduler/pkg/scheduler/plugins/snapshot"
	"k8s.io/client-go/rest"

	"k8s.io/client-go/kubernetes"
)

//go:embed ui/*
var uiFS embed.FS

type config struct {
	listen       string
	snapshotURL  string
	snapshotFile string
	refresh      time.Duration
	live         bool

	kubeconfig    string
	schedulerName string
	defaultQueue  string
	defaultNS     string
	defaultImage  string
}

type server struct {
	cfg     config
	kube    kubernetes.Interface
	kai     *kaischedulerclientset.Clientset
	restCfg *rest.Config

	mu       sync.RWMutex
	lastViz  *Viz
	lastErr  error
	lastLoad time.Time
}

func main() {
	var cfg config
	flags := flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	flags.StringVar(&cfg.listen, "listen", ":8099", "HTTP listen address")
	flags.StringVar(&cfg.snapshotURL, "snapshot-url", "", "Scheduler snapshot URL (expects /get-snapshot zip)")
	flags.StringVar(&cfg.snapshotFile, "snapshot-file", "", "Path to a snapshot zip file (from /get-snapshot)")
	flags.BoolVar(&cfg.live, "live", false, "Load cluster state directly from the Kubernetes API instead of snapshot-url/file")
	flags.DurationVar(&cfg.refresh, "refresh", 5*time.Second, "How often to refresh snapshot/viz")
	flags.StringVar(&cfg.kubeconfig, "kubeconfig", "", "Path to kubeconfig (optional; defaults to in-cluster config or standard kubeconfig resolution)")
	flags.StringVar(&cfg.schedulerName, "scheduler-name", "kai-scheduler", "schedulerName to set on created pods")
	flags.StringVar(&cfg.defaultQueue, "default-queue", "default-queue", "Default queue label value for created pods (label key kai.scheduler/queue)")
	flags.StringVar(&cfg.defaultNS, "default-namespace", "default", "Default namespace for created pods")
	flags.StringVar(&cfg.defaultImage, "default-image", "busybox:1.36", "Default container image for created pods")
	_ = flags.Parse(os.Args[1:])

	if !cfg.live {
		if cfg.snapshotURL == "" && cfg.snapshotFile == "" {
			log.Fatal("must set --snapshot-url or --snapshot-file, or pass --live")
		}
		if cfg.snapshotURL != "" && cfg.snapshotFile != "" {
			log.Fatal("set only one of --snapshot-url or --snapshot-file")
		}
	}

	restCfg, err := BuildRestConfig(cfg.kubeconfig)
	if err != nil {
		log.Fatalf("failed to init kube rest config: %v", err)
	}
	kubeClient, err := kubernetes.NewForConfig(restCfg)
	if err != nil {
		log.Fatalf("failed to init kube client: %v", err)
	}

	kaiClient, err := kaischedulerclientset.NewForConfig(restCfg)
	if err != nil {
		log.Fatalf("failed to init kai client: %v", err)
	}

	srv := &server{cfg: cfg, kube: kubeClient, kai: kaiClient, restCfg: restCfg}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go srv.refreshLoop(ctx)

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("/viz.json", srv.handleViz)
	mux.HandleFunc("/api/pods", srv.handleCreatePod)
	mux.HandleFunc("/api/queues", srv.handleQueue)
	mux.HandleFunc("/api/topology", srv.handleTopology)

	uiSub, err := fs.Sub(uiFS, "ui")
	if err != nil {
		log.Fatal(err)
	}
	uiHandler := http.FileServer(http.FS(uiSub))
	mux.Handle("/ui/", http.StripPrefix("/ui", uiHandler))
	mux.Handle("/", uiHandler)

	log.Printf("gpu-tetris listening on %s", cfg.listen)
	log.Printf("open http://localhost%s/", cfg.listen)
	if err := http.ListenAndServe(cfg.listen, mux); err != nil {
		log.Fatal(err)
	}
}

func (s *server) refreshLoop(ctx context.Context) {
	ticker := time.NewTicker(s.cfg.refresh)
	defer ticker.Stop()

	// Initial load
	s.loadOnce(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.loadOnce(ctx)
		}
	}
}

func (s *server) loadOnce(ctx context.Context) {
	var (
		snap *snapshotplugin.Snapshot
		err  error
	)
	if s.cfg.live {
		snap, err = LoadLiveSnapshot(ctx, s.kube, s.kai)
	} else {
		snap, err = LoadSnapshot(ctx, s.cfg.snapshotURL, s.cfg.snapshotFile)
	}
	viz := (*Viz)(nil)
	if err == nil {
		viz, err = BuildViz(snap)
	}

	s.mu.Lock()
	s.lastViz = viz
	s.lastErr = err
	s.lastLoad = time.Now()
	s.mu.Unlock()
}

func (s *server) handleViz(w http.ResponseWriter, r *http.Request) {
	// Opportunistic refresh if the last load is stale.
	s.mu.RLock()
	stale := time.Since(s.lastLoad) > s.cfg.refresh*2
	s.mu.RUnlock()
	if stale {
		s.loadOnce(r.Context())
	}

	s.mu.RLock()
	viz := s.lastViz
	err := s.lastErr
	s.mu.RUnlock()

	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	if viz == nil {
		http.Error(w, "viz not ready", http.StatusServiceUnavailable)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(viz)
}
