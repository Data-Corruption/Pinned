# 📌 Pinned

`Pinned` is a GPIO pin control and webcam streaming interface for Raspberry Pi 4/5.

No auth or built in security, just a simple server you run on your Pi, then access from a browser on the same **local network**.

## Features
- **Instant GPIO Control:** View and toggle GPIO directions (Input/Output), pull resistors, and logic states directly from the web interface. 
- **MJPEG Camera Streaming:** Plop a USB webcam or Pi Camera into `/dev/video0` and it natively streams right over HTTP into the dashboard.
- **Multiplayer:** Instant synchronous UI updates via WebSockets. If someone else toggles a pin or a hardware button triggers an edge event, the dashboard updates in real-time.
- **Zero Config:** Auto-detects your system. Run it on your laptop and you get a safe "simulated" GPIO mock mode. Run it on your Pi and it grabs control of the actual hardware headers seamlessly!

## Quick Start

**Prerequisites**
- Raspberry Pi / any arm64 or x86_64 machine (for testing)
- Linux (any systemd based, Trixie is tested)
- A USB webcam or Pi Camera (optional)

**Permissions**  
Add your user to the `video` and `gpio` groups for hardware access:
```sh
sudo usermod -a -G video,gpio $USER
```
> *Log out and back in (or just restart) for permissions to take effect!*

**Install**
```sh
curl -fsSL https://cd.pinned.shdata.net/install.sh | sh
```
Then open `http://<Pi Local IP>:7727` in your browser.

**Uninstall**
```sh
pinned uninstall
```

## WebSocket API

To read or control the GPIO pins programmatically, connect your favorite client (Python, Node.js, Rust, etc.) to the WebSocket hub at `ws://<Pi Local IP>:7727/api/pins/ws`.

When you connect, you will immediately receive a complete `sync` snapshot of the board's state. It contains an entry for every usable GPIO pin:

```json
{
  "type": "sync",
  "pins": {
    "3": { ... },
    "5": { ... },
    "7": {
      "settings": {
        "direction": "input",
        "state": "low",
        "pull": "none"
      },
      "valueHigh": false
    },
    "...": { ... }
  }
}
```

> **Note:** The ID keys used here (e.g. `"7"`) represent the **physical header pin block** (1-40), not the internal BCM GPIO number. Pinned currently supports controlling the 26 general purpose pins out of the 40-pin header.

To mutate a pin, send a `pin_update` payload. The `patch` object is optional-chaining (you only need to include the fields you intend to change):

```json
{
  "type": "pin_update",
  "pin": 7,
  "patch": {
    "direction": "output",
    "state": "high"
  }
}
```

Any patch sent to the hub instantly updates the physical hardware, and the new pin state is immediately broadcast to all connected clients (for instance, if you switch a pin to `"input"`, every dashboard and script receives an update reflecting that change). Those use the same format as the `pin_update` message.

Additionally, if a pin is configured as an input and you push a physical button plugged into the Pi, the hub automatically catches the hardware edge-event in real-time and broadcasts the new state (e.g. `"valueHigh": true`) to all connections using the same `pin_update` format.

> **Hardware Constraints:** 
> Physical header pins 3 and 5 (BCM GPIO 2 and 3) have hardwired 1.8k pull-up resistors on the Raspberry Pi PCB for I2C. Any `pin_update` patch attempting to set them to `pull: "down"` or `pull: "none"` will be safely ignored by the backend hub.
