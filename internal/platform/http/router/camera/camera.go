package camera

import (
	"context"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"sprout/internal/app"
	"sync"

	"github.com/Data-Corruption/stdx/xhttp"
	"github.com/go-chi/chi/v5"
	"github.com/vladimirvivien/go4vl/device"
	"github.com/vladimirvivien/go4vl/v4l2"
)

var (
	viewerMu sync.Mutex
	viewers  map[chan []byte]struct{}
	cam      *device.Device
)

func Register(a *app.App, r chi.Router) {
	viewers = make(map[chan []byte]struct{})
	r.Get("/api/camera/stream", handleGetStream(a))
}

func handleGetStream(a *app.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		viewerMu.Lock()
		empty := len(viewers) == 0
		if empty && cam == nil {
			dev, err := device.Open("/dev/video0")
			if err != nil {
				viewerMu.Unlock()
				xhttp.Error(r.Context(), w, &xhttp.Err{Code: 500, Msg: "camera unavailable or not found at /dev/video0", Err: err})
				return
			}
			
			if err := dev.SetPixFormat(v4l2.PixFormat{
				PixelFormat: v4l2.PixelFmtMJPEG,
				Width:       640,
				Height:      480,
			}); err != nil {
				dev.Close()
				viewerMu.Unlock()
				xhttp.Error(r.Context(), w, &xhttp.Err{Code: 500, Msg: "failed to set pixel format", Err: err})
				return
			}
			
			out := dev.GetOutput()

			if err := dev.Start(context.Background()); err != nil {
				dev.Close()
				viewerMu.Unlock()
				xhttp.Error(r.Context(), w, &xhttp.Err{Code: 500, Msg: "failed to start stream", Err: err})
				return
			}
			cam = dev

			go func() {
				for frame := range out {
					viewerMu.Lock()
					for v := range viewers {
						select {
						case v <- frame:
						default:
						}
					}
					viewerMu.Unlock()
				}
			}()
		}

		ch := make(chan []byte, 2)
		viewers[ch] = struct{}{}
		viewerMu.Unlock()

		defer func() {
			viewerMu.Lock()
			delete(viewers, ch)
			if len(viewers) == 0 && cam != nil {
				cam.Stop()
				cam.Close()
				cam = nil
			}
			viewerMu.Unlock()
		}()

		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		w.Header().Set("Pragma", "no-cache")
		w.Header().Set("Expires", "0")
		w.Header().Set("Connection", "close")

		mimeWriter := multipart.NewWriter(w)
		w.Header().Set("Content-Type", fmt.Sprintf("multipart/x-mixed-replace; boundary=%s", mimeWriter.Boundary()))

		partHeader := make(textproto.MIMEHeader)
		partHeader.Add("Content-Type", "image/jpeg")

		ctx := r.Context()
		for {
			select {
			case <-ctx.Done():
				return
			case <-a.Context.Done():
				return
			case frame := <-ch:
				if len(frame) == 0 {
					continue
				}
				partWriter, err := mimeWriter.CreatePart(partHeader)
				if err != nil {
					return
				}
				if _, err := partWriter.Write(frame); err != nil {
					return
				}
				if flusher, ok := w.(http.Flusher); ok {
					flusher.Flush()
				}
			}
		}
	}
}
