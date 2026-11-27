// Package media manages local raw audio/image attachments, cross-platform audio playback
// via mygo/audio (Android AudioTrack, Desktop, Web), and Android/Desktop microphone capture.
package media

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/egoist/mygo/audio"
	"github.com/pengstem/neromancer/internal/voicerec"
)

// ErrNoMicBackend is returned on desktop when neither ffmpeg nor sox/rec is present on PATH.
var ErrNoMicBackend = errors.New("当前电脑没装录音工具（ffmpeg 或 sox），可以先导入录音文件，或在手机上直接录音")

// Manager owns managed local copies of raw audio & image evidence files.
type Manager struct {
	audioDir string
	imageDir string

	mu        sync.Mutex
	recCmd    *exec.Cmd
	recOut    string
	recAt     time.Time
	nativeRec bool

	player   *audio.Player
	playKey  string
	playPath string
	onEnded  func()
}

// NewManager creates a media manager rooted at audioDir and imageDir.
func NewManager(audioDir, imageDir string) *Manager {
	return &Manager{
		audioDir: audioDir,
		imageDir: imageDir,
	}
}

// ImportFile copies an external audio or image file into managed storage and returns its new path.
func (m *Manager) ImportFile(srcPath string, isAudio bool) (string, error) {
	srcPath = strings.TrimSpace(srcPath)
	if srcPath == "" {
		return "", errors.New("文件路径为空")
	}
	targetDir := m.imageDir
	if isAudio {
		targetDir = m.audioDir
	}
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return "", err
	}

	in, err := os.Open(srcPath)
	if err != nil {
		return "", fmt.Errorf("无法读取文件 %s: %w", srcPath, err)
	}
	defer in.Close()

	ext := strings.ToLower(filepath.Ext(srcPath))
	if ext == "" {
		if isAudio {
			ext = ".wav"
		} else {
			ext = ".png"
		}
	}
	dst := filepath.Join(targetDir, fmt.Sprintf("raw_%d%s", time.Now().UnixNano(), ext))
	out, err := os.Create(dst)
	if err != nil {
		return "", err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return "", err
	}
	return dst, nil
}

// CanRecord reports whether microphone capture is available on the current platform.
func (m *Manager) CanRecord() bool {
	if runtime.GOOS == "android" {
		return true
	}
	return detectRecorderCommand("") != nil
}

// StartRecording starts capturing 16kHz 16-bit mono PCM WAV compatible with DeepSeek /v1/tools/asr.
func (m *Manager) StartRecording() (string, error) {
	if runtime.GOOS == "android" {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if err := voicerec.Start(ctx); err != nil {
			return "", err
		}
		m.mu.Lock()
		m.nativeRec = true
		m.recAt = time.Now()
		m.mu.Unlock()
		return "native_mic_active", nil
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if m.recCmd != nil {
		return m.recOut, nil
	}
	if err := os.MkdirAll(m.audioDir, 0o755); err != nil {
		return "", err
	}
	outPath := filepath.Join(m.audioDir, fmt.Sprintf("voice_%d.wav", time.Now().UnixNano()))
	cmd := detectRecorderCommand(outPath)
	if cmd == nil {
		return "", ErrNoMicBackend
	}
	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("启动录音失败: %w", err)
	}
	m.recCmd = cmd
	m.recOut = outPath
	m.recAt = time.Now()
	return outPath, nil
}

// StopRecording terminates an active recording and returns its managed WAV path and duration in milliseconds.
func (m *Manager) StopRecording() (string, int, error) {
	m.mu.Lock()
	isNative := m.nativeRec
	cmd := m.recCmd
	out := m.recOut
	started := m.recAt
	m.nativeRec = false
	m.recCmd = nil
	m.recOut = ""
	m.mu.Unlock()

	if isNative {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		tmpPath, durMs, err := voicerec.Stop(ctx)
		if err != nil {
			return "", 0, err
		}
		managed, err := m.ImportFile(tmpPath, true)
		if err != nil {
			return tmpPath, durMs, nil
		}
		return managed, durMs, nil
	}

	if cmd == nil {
		return "", 0, errors.New("当前没有进行中的录音")
	}
	if cmd.Process != nil {
		_ = cmd.Process.Signal(os.Interrupt)
		done := make(chan error, 1)
		go func() { done <- cmd.Wait() }()
		select {
		case <-done:
		case <-time.After(900 * time.Millisecond):
			_ = cmd.Process.Kill()
			<-done
		}
	}
	durMs := int(time.Since(started).Milliseconds())
	if durMs < 200 {
		durMs = 200
	}
	return out, durMs, nil
}

// IsRecording reports whether microphone capture is active.
func (m *Manager) IsRecording() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.nativeRec || m.recCmd != nil
}

// SetPlaybackEnded registers fn, called after the current recording plays to
// its end. A manual StopAudio does not call it. fn may run off the UI thread.
func (m *Manager) SetPlaybackEnded(fn func()) {
	m.mu.Lock()
	m.onEnded = fn
	m.mu.Unlock()
}

// IsPlayingPath reports whether audioPath is the recording currently playing.
// An empty path matches the demo chime used for seeded voice notes.
func (m *Manager) IsPlayingPath(audioPath string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.player != nil && m.player.Playing() && m.playKey == strings.TrimSpace(audioPath)
}

// PlayAudio starts one playback of audioPath. Calling it again while that
// same recording is already playing does not start a second voice.
// If audioPath is empty, it plays a short demo chime.
func (m *Manager) PlayAudio(audioPath string) error {
	key := strings.TrimSpace(audioPath)
	target, err := m.resolvePlayPath(key)
	if err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.player != nil && m.player.Playing() && m.playKey == key {
		return nil
	}
	m.stopLocked()
	data, err := os.ReadFile(target)
	if err != nil {
		return fmt.Errorf("读取原始录音失败: %w", err)
	}
	snd, err := audio.DecodeSound(data)
	if err != nil {
		return fmt.Errorf("解码录音失败: %w", err)
	}
	p := snd.Play()
	p.OnEnd(func() {
		m.mu.Lock()
		current := m.player == p
		var cb func()
		if current {
			m.player = nil
			m.playKey = ""
			m.playPath = ""
			cb = m.onEnded
		}
		m.mu.Unlock()
		if cb != nil {
			cb()
		}
	})
	m.player = p
	m.playKey = key
	m.playPath = target
	return nil
}

// StopAudio stops the current recording playback. It is safe when nothing is playing.
func (m *Manager) StopAudio() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.stopLocked()
}

func (m *Manager) stopLocked() {
	if m.player != nil {
		m.player.Stop()
		m.player = nil
	}
	m.playKey = ""
	m.playPath = ""
}

func (m *Manager) resolvePlayPath(key string) (string, error) {
	if key != "" {
		return key, nil
	}
	return m.ensureDemoChime()
}

func (m *Manager) ensureDemoChime() (string, error) {
	if err := os.MkdirAll(m.audioDir, 0o755); err != nil {
		return "", err
	}
	path := filepath.Join(m.audioDir, "demo_chime.wav")
	if _, err := os.Stat(path); err == nil {
		return path, nil
	}
	return path, writeSoftChimeWAV(path)
}

func writeSoftChimeWAV(path string) error {
	const sampleRate = 16000
	const durationSec = 0.45
	numSamples := int(sampleRate * durationSec)
	pcm := make([]byte, numSamples*2)

	for i := 0; i < numSamples; i++ {
		t := float64(i) / sampleRate
		env := math.Exp(-5.0 * t)
		val := 0.25*math.Sin(2*math.Pi*523.25*t) + 0.18*math.Sin(2*math.Pi*659.25*t)
		sample := int16(val * env * 32767)
		binary.LittleEndian.PutUint16(pcm[i*2:], uint16(sample))
	}

	var hdr [44]byte
	copy(hdr[0:4], "RIFF")
	binary.LittleEndian.PutUint32(hdr[4:8], uint32(36+len(pcm)))
	copy(hdr[8:12], "WAVE")
	copy(hdr[12:16], "fmt ")
	binary.LittleEndian.PutUint32(hdr[16:20], 16)
	binary.LittleEndian.PutUint16(hdr[20:22], 1)
	binary.LittleEndian.PutUint16(hdr[22:24], 1)
	binary.LittleEndian.PutUint32(hdr[24:28], sampleRate)
	binary.LittleEndian.PutUint32(hdr[28:32], sampleRate*2)
	binary.LittleEndian.PutUint16(hdr[32:34], 2)
	binary.LittleEndian.PutUint16(hdr[34:36], 16)
	copy(hdr[36:40], "data")
	binary.LittleEndian.PutUint32(hdr[40:44], uint32(len(pcm)))

	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := f.Write(hdr[:]); err != nil {
		return err
	}
	_, err = f.Write(pcm)
	return err
}

func detectRecorderCommand(outPath string) *exec.Cmd {
	if _, err := exec.LookPath("rec"); err == nil {
		if outPath == "" {
			return &exec.Cmd{}
		}
		return exec.Command("rec", "-q", "-r", "16000", "-c", "1", "-b", "16", outPath)
	}
	if _, err := exec.LookPath("ffmpeg"); err == nil {
		if outPath == "" {
			return &exec.Cmd{}
		}
		switch runtime.GOOS {
		case "darwin":
			return exec.Command("ffmpeg", "-y", "-f", "avfoundation", "-i", ":0", "-ar", "16000", "-ac", "1", "-t", "120", outPath)
		case "linux":
			return exec.Command("ffmpeg", "-y", "-f", "alsa", "-i", "default", "-ar", "16000", "-ac", "1", "-t", "120", outPath)
		}
	}
	return nil
}
