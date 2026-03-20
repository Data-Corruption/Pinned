package index

import (
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"os/exec"
	"sprout/internal/app"
	"sprout/internal/platform/database/config"
	"sprout/internal/types"
	"time"

	"github.com/Data-Corruption/stdx/xhttp"
	"github.com/go-chi/chi/v5"
	"github.com/warthog618/go-gpiocdev"
)

func Register(a *app.App, r chi.Router) {
	r.Get("/", handleGetIndex(a))
	r.Post("/settings", handleUpdateSettings(a))
	r.Post("/settings/stop", handleStop(a))
	r.Post("/settings/restart", handleRestart(a))
	r.Get("/settings/restart-status", handleRestartStatus(a))
}

func handleGetIndex(a *app.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cfg, err := config.View(a.DB)
		if err != nil {
			xhttp.Error(r.Context(), w, err)
			return
		}

		data := map[string]any{
			"CSS":             a.UI.CSS.URLPath,
			"JS":              a.UI.JS.URLPath,
			"Favicon":         template.URL(`data:image/svg+xml,<svg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 100 100'><text x='50%' y='.9em' font-size='90' text-anchor='middle'>📌</text></svg>`),
			"Version":         a.BuildInfo().Version,
			"UpdateAvailable": cfg.UpdateAvailable && (a.BuildInfo().Version != "vX.X.X"),
			"MockGPIO":        len(gpiocdev.Chips()) == 0,
			//  config fields
			"LogLevel":     cfg.LogLevel,
			"Port":         cfg.Port,
			"CameraWidth":  cfg.CameraWidth,
			"CameraHeight": cfg.CameraHeight,
			"CameraFPS":    cfg.CameraFPS,
		}
		if err := a.UI.Execute(w, "index.html", data); err != nil {
			xhttp.Error(r.Context(), w, err)
			return
		}
	}
}

func handleUpdateSettings(a *app.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		// Parse body - all fields are optional
		var body struct {
			LogLevel     *string `json:"logLevel"`
			Port         *int    `json:"port"`
			CameraWidth  *int    `json:"cameraWidth"`
			CameraHeight *int    `json:"cameraHeight"`
			CameraFPS    *int    `json:"cameraFps"`
		}
		dec := json.NewDecoder(r.Body)
		if err := dec.Decode(&body); err != nil {
			xhttp.Error(r.Context(), w, &xhttp.Err{Code: 400, Msg: "bad request", Err: err})
			return
		}

		// Update only the fields that were provided
		if _, err := config.Update(a.DB, func(cfg *types.Configuration) error {
			if body.LogLevel != nil {
				cfg.LogLevel = *body.LogLevel
			}
			if body.Port != nil {
				cfg.Port = *body.Port
			}
			if body.CameraWidth != nil {
				cfg.CameraWidth = *body.CameraWidth
			}
			if body.CameraHeight != nil {
				cfg.CameraHeight = *body.CameraHeight
			}
			if body.CameraFPS != nil {
				cfg.CameraFPS = *body.CameraFPS
			}
			return nil
		}); err != nil {
			xhttp.Error(r.Context(), w, &xhttp.Err{Code: 500, Msg: "failed to update config", Err: err})
			return
		}

		w.WriteHeader(http.StatusOK)
	}
}

func handleStop(a *app.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		w.WriteHeader(http.StatusAccepted)

		if a.BuildInfo().ServiceEnabled && a.BuildInfo().Version != "vX.X.X" {
			// Use systemd-run to create a transient unit that survives our process dying.
			// This ensures the stop command completes and logs reliably.
			go func() {
				serviceName := a.BuildInfo().Name + ".service"
				unitName := fmt.Sprintf("%s-stop-%s", a.BuildInfo().Name, time.Now().Format("20060102-150405"))
				syslogIdent := fmt.Sprintf("SyslogIdentifier=%s-stop", a.BuildInfo().Name)

				cmd := exec.CommandContext(
					a.Context,
					"systemd-run",
					"--user",
					"--unit="+unitName,
					"--quiet",
					"--no-block",
					"-p", "StandardOutput=journal",
					"-p", "StandardError=journal",
					"-p", syslogIdent,
					"systemctl", "--user", "stop", serviceName,
				)
				if err := cmd.Run(); err != nil {
					a.Log.Errorf("failed to start stop unit: %v", err)
				}
			}()
		} else {
			go a.Server.Shutdown()
		}
	}
}

func handleRestart(a *app.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		// parse body
		var body struct {
			Update bool `json:"update"`
		}
		dec := json.NewDecoder(r.Body)
		if err := dec.Decode(&body); err != nil {
			xhttp.Error(r.Context(), w, &xhttp.Err{Code: 400, Msg: "bad request", Err: err})
			return
		}

		// skip update if dev build
		var doUpdate bool
		if body.Update && a.BuildInfo().Version != "vX.X.X" {
			doUpdate = true
		}

		a.Log.Debugf("Restart requested. Update: %t, DoUpdate: %t", body.Update, doUpdate)

		// set StartCounter to 0 (post migrate restart will increment)
		if _, err := config.Update(a.DB, func(cfg *types.Configuration) error {
			cfg.StartCounter = 0
			return nil
		}); err != nil {
			xhttp.Error(r.Context(), w, &xhttp.Err{Code: 500, Msg: "failed to update config", Err: err})
			return
		}

		w.WriteHeader(http.StatusAccepted)

		// do the restart
		if doUpdate {
			// detach update will close us externally
			if err := a.DetachUpdate(); err != nil {
				a.Log.Errorf("failed to detach update: %v", err)
			}
		} else {
			// otherwise we need to close ourselves
			go a.Server.Shutdown()
		}
	}
}

func handleRestartStatus(a *app.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cfg, err := config.View(a.DB)
		if err != nil {
			xhttp.Error(r.Context(), w, err)
			return
		}

		restarted := cfg.StartCounter > 0
		updated := cfg.PreUpdateVersion != "" && cfg.PreUpdateVersion != a.BuildInfo().Version

		a.Log.Debugf("Restart status check: StartCounter=%d, PreUpdateVersion=%q, CurrentVersion=%q, Restarted=%t, Updated=%t",
			cfg.StartCounter, cfg.PreUpdateVersion, a.BuildInfo().Version, restarted, updated)

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(map[string]bool{"restarted": restarted, "updated": updated}); err != nil {
			xhttp.Error(r.Context(), w, err)
		}
	}
}
