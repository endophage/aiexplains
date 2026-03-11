package cmd

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/endophage/aiexplains/backend/internal"
	"github.com/endophage/aiexplains/backend/internal/db"
	"github.com/endophage/aiexplains/backend/internal/handlers"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the aiexplains server",
	RunE:  runServe,
}

func init() {
	rootCmd.AddCommand(serveCmd)
	serveCmd.Flags().Int("port", 3000, "Port to listen on")
	serveCmd.Flags().String("host", "127.0.0.1", "Host address to listen on (use 0.0.0.0 for all interfaces)")
	serveCmd.Flags().String("frontend-dir", "", "Path to the built frontend directory")
	serveCmd.Flags().String("mode", "exec", `AI mode: "exec" uses the local claude CLI, "api" uses the Anthropic SDK`)
	viper.BindPFlag("port", serveCmd.Flags().Lookup("port"))
	viper.BindPFlag("host", serveCmd.Flags().Lookup("host"))
	viper.BindPFlag("frontend_dir", serveCmd.Flags().Lookup("frontend-dir"))
	viper.BindPFlag("mode", serveCmd.Flags().Lookup("mode"))
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
	if frontendDir == "" {
		frontendDir = "./frontend/dist"
	}

	mux.Handle("/", newSPAHandler(frontendDir))

	port := viper.GetInt("port")
	host := viper.GetString("host")
	if host == "" {
		host = "127.0.0.1"
	}
	addr := fmt.Sprintf("%s:%d", host, port)
	log.Printf("Starting server on http://%s", addr)

	return http.ListenAndServe(addr, mux)
}

// spaHandler serves a React SPA, falling back to index.html for unknown routes.
type spaHandler struct {
	dir      string
	fileServer http.Handler
}

func newSPAHandler(dir string) *spaHandler {
	return &spaHandler{
		dir:      dir,
		fileServer: http.FileServer(http.Dir(dir)),
	}
}

func (h *spaHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := filepath.Join(h.dir, filepath.Clean(r.URL.Path))
	_, err := os.Stat(path)
	if os.IsNotExist(err) {
		http.ServeFile(w, r, filepath.Join(h.dir, "index.html"))
		return
	}
	h.fileServer.ServeHTTP(w, r)
}
