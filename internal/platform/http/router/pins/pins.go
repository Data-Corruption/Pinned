package pins

import (
	"context"
	"net/http"
	"sprout/internal/app"
	"sprout/internal/platform/database/config"
	"sprout/internal/types"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/go-chi/chi/v5"
	"github.com/warthog618/go-gpiocdev"
)

type PinDirection = types.PinDirection
type PinState = types.PinState
type PinPull = types.PinPull
type PinSettings = types.PinSettings

const (
	DirInput  = types.DirInput
	DirOutput = types.DirOutput
	StateLow  = types.StateLow
	StateHigh = types.StateHigh
	PullNone  = types.PullNone
	PullUp    = types.PullUp
	PullDown  = types.PullDown
)

type PinSettingsPatch struct {
	Direction *PinDirection `json:"direction,omitempty"`
	State     *PinState     `json:"state,omitempty"`
	Pull      *PinPull      `json:"pull,omitempty"`
}

type SyncMsg struct {
	Type string              `json:"type"`
	Pins map[int]SyncPinData `json:"pins"`
}

type SyncPinData struct {
	Settings  PinSettings `json:"settings"`
	ValueHigh bool        `json:"valueHigh"`
}

type UpdateMsg struct {
	Type      string           `json:"type"`
	Pin       int              `json:"pin"`
	Patch     PinSettingsPatch `json:"patch"`
	ValueHigh *bool            `json:"valueHigh,omitempty"`
}

var headerToGPIO = map[int]int{
	3: 2, 5: 3, 7: 4, 8: 14, 10: 15, 11: 17, 12: 18, 13: 27, 15: 22, 16: 23, 18: 24, 19: 10,
	21: 9, 22: 25, 23: 11, 24: 8, 26: 7, 29: 5, 31: 6, 32: 12, 33: 13, 35: 19, 36: 16, 37: 26, 38: 20, 40: 21,
}

func Register(a *app.App, r chi.Router) {
	h := newHub(a)
	r.Get("/api/pins/ws", h.handleWS)
}

type client struct {
	conn *websocket.Conn
	send chan interface{}
}

type Hub struct {
	a        *app.App
	mu       sync.Mutex
	clients  map[*client]struct{}
	settings map[int]PinSettings
	values   map[int]bool
	lines    map[int]*gpiocdev.Line
	chipName string
}

// newHub creates and initializes a new GPIO WebSocket Hub. It loads persisted UI configurations
// off the disk, enforces physical hardware constraints (like I2C pull-ups), and spawns event listeners on boot.
func newHub(a *app.App) *Hub {
	h := &Hub{
		a:        a,
		clients:  make(map[*client]struct{}),
		settings: make(map[int]PinSettings),
		values:   make(map[int]bool),
		lines:    make(map[int]*gpiocdev.Line),
	}

	// Figure out the best GPIO chip (mock if not on Pi)
	chips := gpiocdev.Chips()
	if len(chips) > 0 {
		var bestChip string
		var maxLines int
		
		for _, chipName := range chips {
			c, err := gpiocdev.NewChip(chipName)
			if err == nil {
				lines := c.Lines()
				c.Close()
				if lines > maxLines {
					maxLines = lines
					bestChip = chipName
				}
			}
		}
		
		if bestChip != "" {
			h.chipName = bestChip
			a.Log.Infof("Using GPIO chip: %s (%d lines)", h.chipName, maxLines)
		} else {
			a.Log.Warnf("Found GPIO chips but could not access them (try checking groups). Using mock GPIO.")
		}
	} else {
		a.Log.Warnf("No GPIO chips found (are you running on a Pi?). Using mock GPIO.")
	}

	// Load persisted settings
	cfg, err := config.View(a.DB)
	if err != nil {
		a.Log.Warnf("failed to view config for pins db state: %v", err)
	}

	// Initialize state
	for header, gpio := range headerToGPIO {
		pull := PullNone
		// GPIO 2 and 3 have physical hardwired 1.8k pull-ups
		if gpio == 2 || gpio == 3 {
			pull = PullUp
		}

		settings := PinSettings{
			Direction: DirInput,
			State:     StateLow,
			Pull:      pull,
		}

		// Apply persisted settings if available
		if cfg != nil && cfg.Pins != nil {
			if saved, ok := cfg.Pins[header]; ok {
				// Enforce hardwired hardware limitations
				if gpio == 2 || gpio == 3 {
					saved.Pull = PullUp
				}
				settings = saved
			}
		}

		h.settings[header] = settings
		h.values[header] = false // Corrected instantly by applyHardware

		go h.applyHardware(header, settings)
	}

	return h
}

// handleWS upgrades an HTTP request to a WebSocket connection, injects the client into
// the hub, and immediately transmits a complete sync payload to bring the frontend up to speed.
// It then blocks, reading inbound messages indefinitely until disconnect.
func (h *Hub) handleWS(w http.ResponseWriter, r *http.Request) {
	c, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		InsecureSkipVerify: true,
	})
	if err != nil {
		h.a.Log.Errorf("websocket accept: %v", err)
		return
	}

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	cl := &client{
		conn: c,
		send: make(chan interface{}, 256),
	}

	h.mu.Lock()
	h.clients[cl] = struct{}{}

	// Watch for app shutdown to proactively kill stuck websocket reads
	go func() {
		select {
		case <-ctx.Done():
		case <-h.a.Context.Done():
			cancel()
			c.Close(websocket.StatusGoingAway, "server shutting down")
		}
	}()

	// Build sync message
	syncMsg := SyncMsg{
		Type: "sync",
		Pins: make(map[int]SyncPinData),
	}
	for header, settings := range h.settings {
		syncMsg.Pins[header] = SyncPinData{
			Settings:  settings,
			ValueHigh: h.values[header],
		}
	}
	h.mu.Unlock()

	// Start write pump
	go h.writePump(ctx, cl)

	// Send initial state via explicit non-blocking channel send
	select {
	case cl.send <- syncMsg:
	default:
	}

	for {
		var inMsg UpdateMsg
		err := wsjson.Read(ctx, c, &inMsg)
		if err != nil {
			break
		}
		if inMsg.Type == "pin_update" {
			h.applyClientPatch(inMsg.Pin, inMsg.Patch)
		}
	}
	h.removeClient(cl)
}

// writePump handles all outbound network transmission for a specific client.
// Confining websocket writes to a dedicated goroutine guarantees we don't naturally execute
// fatal concurrent writes to the same connection if multiple hardware edges trigger instantly.
func (h *Hub) writePump(ctx context.Context, c *client) {
	defer h.removeClient(c)
	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-c.send:
			if !ok {
				// channel closed by hub
				return
			}
			writeCtx, cancel := context.WithTimeout(ctx, time.Second*5)
			err := wsjson.Write(writeCtx, c.conn, msg)
			cancel()
			if err != nil {
				// Failed to write (timeout or closed). Connection will be forcefully dropped.
				return
			}
		}
	}
}

// removeClient safely extracts the client from the hub's synchronized map and cleans up resources.
func (h *Hub) removeClient(c *client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if _, ok := h.clients[c]; ok {
		delete(h.clients, c)
		close(c.send)
		c.conn.Close(websocket.StatusInternalError, "disconnected")
	}
}

// broadcast queues a message for transmission to every single connected browser tab.
// If a client is stalling and the channel buffer is maxed out, it aggressively forces a disconnect
// to prevent our hardware event-listeners from hitting blocked goroutine resource leaks.
func (h *Hub) broadcast(msg interface{}) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for c := range h.clients {
		select {
		case c.send <- msg:
		default:
			// Buffer full, connection too slow. Boot them.
			delete(h.clients, c)
			close(c.send)
			c.conn.Close(websocket.StatusPolicyViolation, "client too slow")
		}
	}
}

// applyClientPatch is triggered whenever the web dashboard sends a JSON mutation (e.g. user toggles a pin).
// It updates the transient hub state, fires off an async database commit, and kicks off actual
// hardware execution without blocking subsequent inbound messages.
func (h *Hub) applyClientPatch(header int, patch PinSettingsPatch) {
	h.mu.Lock()
	defer h.mu.Unlock()

	settings, ok := h.settings[header]
	if !ok {
		return
	}

	if patch.Direction != nil {
		settings.Direction = *patch.Direction
	}
	if patch.State != nil {
		settings.State = *patch.State
	}
	if patch.Pull != nil {
		settings.Pull = *patch.Pull
	}

	h.settings[header] = settings

	// Persist to database in the background without blocking the hub
	go func(h_copy int, s_copy PinSettings) {
		config.Update(h.a.DB, func(c *types.Configuration) error {
			if c.Pins == nil {
				c.Pins = make(map[int]PinSettings)
			}
			c.Pins[h_copy] = s_copy
			return nil
		})
	}(header, settings)

	// Apply to hardware asynchronously to avoid blocking the hub lock
	go h.applyHardware(header, settings)
}

// applyHardware is the underlying system wrapper. It utilizes go-gpiocdev to open the Linux character device
// with the requisite options. If a pin is requested as an input, it automatically wires up a kernel event
// handler to rapidly react whenever the pin natively changes state (i.e. someone pushing a physical button).
func (h *Hub) applyHardware(header int, settings PinSettings) {
	h.mu.Lock()
	gpio, ok := headerToGPIO[header]
	if !ok {
		h.mu.Unlock()
		return
	}

	// Close old line
	if old, ok := h.lines[header]; ok {
		old.Close()
		delete(h.lines, header)
	}

	if h.chipName == "" {
		// Mock environment - simply reflect state
		valHigh := settings.State == StateHigh
		if settings.Direction == DirOutput {
			h.values[header] = valHigh
		} else {
			// Mock random inputs or just keep it flat
			valHigh = h.values[header]
		}
		h.mu.Unlock()
		h.broadcastHardwareChange(header, settings, valHigh)
		return
	}

	// Calculate gpiocdev options
	var opts []gpiocdev.LineReqOption

	if settings.Direction == DirOutput {
		val := 0
		if settings.State == StateHigh {
			val = 1
		}
		opts = append(opts, gpiocdev.AsOutput(val))
	} else {
		opts = append(opts, gpiocdev.AsInput)
		opts = append(opts, gpiocdev.WithBothEdges)
		if settings.Pull == PullUp {
			opts = append(opts, gpiocdev.WithPullUp)
		} else if settings.Pull == PullDown {
			opts = append(opts, gpiocdev.WithPullDown)
		}

		// Setup event handler
		opts = append(opts, gpiocdev.WithEventHandler(func(evt gpiocdev.LineEvent) {
			valHigh := evt.Type == gpiocdev.LineEventRisingEdge // if it isn't rising, it's falling (or no edge)
			if evt.Type == gpiocdev.LineEventRisingEdge {
				valHigh = true
			} else if evt.Type == gpiocdev.LineEventFallingEdge {
				valHigh = false
			}

			h.mu.Lock()
			h.values[header] = valHigh
			// Create a patch that represents the current state logic (no patch, just valueHigh)
			// Wait, the state doesn't change, just the read valueHigh
			msg := UpdateMsg{
				Type:      "pin_update",
				Pin:       header,
				Patch:     PinSettingsPatch{},
				ValueHigh: &valHigh,
			}
			h.mu.Unlock()
			h.broadcast(msg)
		}))
	}

	l, err := gpiocdev.RequestLine(h.chipName, gpio, opts...)
	if err != nil {
		h.a.Log.Errorf("failed to request gpio line %d on %s: %v", gpio, h.chipName, err)
		h.mu.Unlock()
		return
	}
	h.lines[header] = l

	valHigh := false
	if settings.Direction == DirOutput {
		valHigh = settings.State == StateHigh
	} else {
		v, err := l.Value()
		if err == nil {
			valHigh = v == 1
		}
	}
	h.values[header] = valHigh

	h.mu.Unlock()
	h.broadcastHardwareChange(header, settings, valHigh)
}

// broadcastHardwareChange propagates the final resolved hardware state downstream to the UI so it
// visually matches reality (for example, if a pull-up input instantly reads HIGH natively without any web interference).
func (h *Hub) broadcastHardwareChange(header int, settings PinSettings, valHigh bool) {
	msg := UpdateMsg{
		Type: "pin_update",
		Pin:  header,
		Patch: PinSettingsPatch{
			Direction: &settings.Direction,
			State:     &settings.State,
			Pull:      &settings.Pull,
		},
		ValueHigh: &valHigh,
	}
	h.broadcast(msg)
}
