//go:build audio_playback

package resource

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gopxl/beep/v2/speaker"
)

// 验证 32-bit PCM wav fallback 能接入 beep speaker 并真实播放
//
// 这个测试需要声卡设备和人工确认声音，只在显式传入 -tags audio_playback 时编译运行
func TestDecodePCM32WAVPlaysThroughBeepSpeaker(t *testing.T) {
	file, err := os.Open(filepath.Join("..", "..", "assets", "audio", "plant_harvest.wav"))
	if err != nil {
		t.Fatalf("open test wav failed: %v", err)
	}
	defer file.Close()

	streamer, format, err := decodePCM32WAV(file)
	if err != nil {
		t.Fatalf("decode 32-bit pcm wav failed: %v", err)
	}
	defer streamer.Close()

	if err := speaker.Init(format.SampleRate, 2048); err != nil {
		t.Fatalf("init beep speaker failed: %v", err)
	}
	defer speaker.Close()

	speaker.PlayAndWait(streamer)
}
