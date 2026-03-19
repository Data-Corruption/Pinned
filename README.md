# 📌 Pinned

`Pinned` is a GPIO pin control and webcam streaming interface for Raspberry Pi 4/5.

No auth or built in security, just a simple server you run on your Pi, then access from a browser on the same **local network**.

## Features
- **Instant GPIO Control:** View and toggle GPIO directions (Input/Output), pull resistors, and logic states directly from the web interface. 
- **MJPEG Camera Streaming:** Plop a USB webcam or Pi Camera into `/dev/video0` and it natively streams right over HTTP into the dashboard.
- **Multiplayer:** Instant synchronous UI updates via WebSockets. If someone else toggles a pin or a hardware button triggers an edge event, the dashboard updates in real-time.
- **Zero Config:** Auto-detects your system. Run it on your laptop and you get a safe "simulated" GPIO mock mode. Run it on your Pi and it grabs control of the actual hardware headers seamlessly!

## Hardware Setup
If you plan to use the Camera feed on a Linux machine or Raspberry Pi, your user account needs permission to read the hardware video device. Add your user to the `video` group:
```bash
sudo usermod -a -G video $USER
```
> *You must log out and log back in, or restart the Pi, for this group permission to take effect!*

## Quick Start

**Prerequisites**
- Raspberry Pi / any arm64 or x86_64 machine (for testing)
- Linux (any systemd based, Trixie is tested)
- A USB webcam or Pi Camera (optional)

**Install**
```sh
curl -fsSL https://cd.pinned.shdata.net/install.sh | sh
```
Then open `http://<Pi Local IP>:7727` in your browser.

**Uninstall**
```sh
pinned uninstall
```
