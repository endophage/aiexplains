package cmd

import (
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/endophage/aiexplains/backend/internal"
	"github.com/endophage/aiexplains/backend/internal/db"
	"github.com/endophage/aiexplains/backend/internal/handlers"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	webview "github.com/webview/webview_go"
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the aiexplains server",
	RunE:  runServe,
}

func init() {
	rootCmd.AddCommand(serveCmd)
	serveCmd.Flags().Int("port", 0, "Port to listen on (0 = pick a free port automatically)")
	serveCmd.Flags().String("host", "127.0.0.1", "Host address to listen on (use 0.0.0.0 for all interfaces)")
	serveCmd.Flags().String("frontend-dir", "", "Path to the built frontend directory")
	serveCmd.Flags().String("mode", "exec", `AI mode: "exec" uses the local claude CLI, "api" uses the Anthropic SDK`)
	serveCmd.Flags().Bool("webview", false, "Open the app in an embedded webview window instead of the system browser")
	viper.BindPFlag("port", serveCmd.Flags().Lookup("port"))
	viper.BindPFlag("host", serveCmd.Flags().Lookup("host"))
	viper.BindPFlag("frontend_dir", serveCmd.Flags().Lookup("frontend-dir"))
	viper.BindPFlag("mode", serveCmd.Flags().Lookup("mode"))
	viper.BindPFlag("webview", serveCmd.Flags().Lookup("webview"))
}

func runServe(cmd *cobra.Command, args []string) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("getting home dir: %w", err)
	}

	dataDir := filepath.Join(homeDir, ".aiexplains")
	if err := os.MkdirAll(filepath.Join(dataDir, "explanations"), 0755); err != nil {
		return fmt.Errorf("creating data dir: %w", err)
	}

	database, err := db.New(filepath.Join(dataDir, "database.sqlite"))
	if err != nil {
		return fmt.Errorf("initializing database: %w", err)
	}
	defer database.Close()

	mux := http.NewServeMux()

	mode := viper.GetString("mode")
	if mode != internal.ModeExec && mode != internal.ModeAPI {
		return fmt.Errorf("invalid --mode %q: must be %q or %q", mode, internal.ModeExec, internal.ModeAPI)
	}
	log.Printf("AI mode: %s", mode)

	h := handlers.New(database, dataDir, mode)
	h.RegisterRoutes(mux)

	frontendDir := viper.GetString("frontend_dir")
	if frontendDir != "" {
		mux.Handle("/", newSPAHandler(os.DirFS(frontendDir)))
	} else if embFS, ok := embeddedFrontend(); ok {
		mux.Handle("/", newSPAHandler(embFS))
	} else {
		mux.Handle("/", newSPAHandler(os.DirFS("./frontend/dist")))
	}

	port := viper.GetInt("port")
	host := viper.GetString("host")
	if host == "" {
		host = "127.0.0.1"
	}

	ln, err := net.Listen("tcp", fmt.Sprintf("%s:%d", host, port))
	if err != nil {
		return fmt.Errorf("listening on %s:%d: %w", host, port, err)
	}
	addr := ln.Addr().String()
	log.Printf("Starting server on http://%s", addr)

	if viper.GetBool("webview") {
		go http.Serve(ln, mux) //nolint:errcheck

		w := webview.New(false)
		defer w.Destroy()
		w.SetTitle("AIExplains")
		w.SetSize(1280, 800, webview.HintNone)
		w.Navigate(fmt.Sprintf("http://%s", addr))
		w.Run()
		return nil
	}

	return http.Serve(ln, mux)
}

// spaHandler serves a React SPA, falling back to index.html for unknown routes.
type spaHandler struct {
	fsys       fs.FS
	fileServer http.Handler
}

func newSPAHandler(fsys fs.FS) *spaHandler {
	return &spaHandler{
		fsys:       fsys,
		fileServer: http.FileServerFS(fsys),
	}
}

func (h *spaHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
	if name == "" {
		name = "."
	}
	_, err := fs.Stat(h.fsys, name)
	if err != nil {
		http.ServeFileFS(w, r, h.fsys, "index.html")
		return
	}
	h.fileServer.ServeHTTP(w, r)
}
