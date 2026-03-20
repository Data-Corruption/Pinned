package types

import (
	"sprout/internal/build"
	"time"
)

type Configuration struct {
	LogLevel string `json:"logLevel"`
	Port     int    `json:"port"` // port the server is listening on. 80/443 will be omitted from URLs

	UpdateNotifications bool      `json:"updateNotifications"`
	LastUpdateCheck     time.Time `json:"lastUpdateCheck"`
	UpdateAvailable     bool      `json:"updateAvailable"`

	// app version when update process was accepted. This is lazily used to determine if the update was successful after restart.
	PreUpdateVersion string `json:"preUpdateVersion"`
	// incremented on each service start (usually server listen or similar), used for detecting restarts
	StartCounter int `json:"startCounter"`

	CameraWidth  int `json:"cameraWidth"`
	CameraHeight int `json:"cameraHeight"`
	CameraFPS    int `json:"cameraFps"`

	// Persisted GPIO configuration map (header pin -> settings)
	Pins map[int]PinSettings `json:"pins"`
}

type PinDirection string
type PinState string
type PinPull string

const (
	DirInput  PinDirection = "input"
	DirOutput PinDirection = "output"

	StateLow  PinState = "low"
	StateHigh PinState = "high"

	PullNone PinPull = "none"
	PullUp   PinPull = "up"
	PullDown PinPull = "down"
)

type PinSettings struct {
	Direction PinDirection `json:"direction"`
	State     PinState     `json:"state"`
	Pull      PinPull      `json:"pull"`
}

func DefaultConfig() Configuration {
	return Configuration{
		LogLevel:            build.Info().DefaultLogLevel,
		Port:                build.Info().ServiceDefaultPort,
		UpdateNotifications: true,
		LastUpdateCheck:     time.Time{},
		CameraWidth:         640,
		CameraHeight:        480,
		CameraFPS:           15,
		Pins:                make(map[int]PinSettings),
	}
}
