// Package voicerec exposes the native Android microphone recorder plugin (16kHz PCM WAV)
// compiled automatically into Android builds via mygo-plugin.json.
package voicerec

import (
	"context"

	"github.com/egoist/mygo"
)

const pluginID = "voicerec"

// Start requests RECORD_AUDIO permission if needed and begins 16kHz PCM microphone capture.
func Start(ctx context.Context) error {
	return mygo.Native(pluginID).Call(ctx, "start", nil, nil)
}

// Stop stops active microphone capture and returns the temporary WAV file path and duration.
func Stop(ctx context.Context) (path string, durationMs int, err error) {
	var res struct {
		Path       string `json:"path"`
		DurationMs int    `json:"duration_ms"`
	}
	err = mygo.Native(pluginID).Call(ctx, "stop", nil, &res)
	return res.Path, res.DurationMs, err
}
