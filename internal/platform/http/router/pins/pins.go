package pins

import (
	"context"
	"net/http"
	"sprout/internal/app"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/go-chi/chi/v5"
	"github.com/warthog618/go-gpiocdev"
)

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

type PinSettingsPatch struct {
	Direction *PinDirection `json:"direction,omitempty"`
	State     *PinState     `json:"state,omitempty"`
	Pull      *PinPull      `json:"pull,omitempty"`
}

type SyncMsg struct {
	Type string                 `json:"type"`
	Pins map[int]SyncPinData    `json:"pins"`
}

type SyncPinData struct {
	Settings  PinSettings `json:"settings"`
	ValueHigh bool        `json:"valueHigh"`
}

type UpdateMsg struct {
	Type      string            `json:"type"`
	Pin       int               `json:"pin"`
	Patch     PinSettingsPatch  `json:"patch"`
	ValueHigh *bool             `json:"valueHigh,omitempty"`
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
		// Just take the first one or logic it out
		// For Pi 4, typically "pinctrl-bcm2711" or gpiochip4. Let's just blindly use the last one which is usually user gpio
		h.chipName = chips[len(chips)-1]
		a.Log.Infof("Using GPIO chip: %s", h.chipName)
	} else {
		a.Log.Warnf("No GPIO chips found (are you running on a Pi?). Using mock GPIO.")
	}

	// Initialize state
	for header := range headerToGPIO {
		h.settings[header] = PinSettings{
			Direction: DirInput,
			State:     StateLow,
			Pull:      PullNone,
		}
		h.values[header] = false
	}

	return h
}

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

func (h *Hub) removeClient(c *client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if _, ok := h.clients[c]; ok {
		delete(h.clients, c)
		close(c.send)
		c.conn.Close(websocket.StatusInternalError, "disconnected")
	}
}

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
	
	// Apply to hardware asynchronously to avoid blocking the hub lock
	go h.applyHardware(header, settings)
}

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
				Type: "pin_update",
				Pin:  header,
				Patch: PinSettingsPatch{},
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
