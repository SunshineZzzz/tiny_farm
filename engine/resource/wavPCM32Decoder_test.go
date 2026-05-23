package resource

import (
	"math"
	"os"
	"path/filepath"
	"testing"
)

// 验证真实资源里的 32-bit PCM wav 能被 fallback 解成 beep streamer
//
// 默认测试只检查格式和采样数据，不依赖声卡设备
func TestDecodePCM32WAVStreamsPlantHarvest(t *testing.T) {
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

	if format.SampleRate != 44100 {
		t.Fatalf("expected sample rate 44100, got %d", format.SampleRate)
	}
	if format.NumChannels != 1 {
		t.Fatalf("expected mono wav, got %d channels", format.NumChannels)
	}
	if format.Precision != 4 {
		t.Fatalf("expected 32-bit precision, got %d", format.Precision)
	}
	if streamer.Len() <= 0 {
		t.Fatalf("expected positive streamer length, got %d", streamer.Len())
	}

	samples := make([][2]float64, 128)
	n, ok := streamer.Stream(samples)
	if n == 0 {
		t.Fatal("expected streamed samples")
	}
	if !ok {
		t.Fatal("expected streamer to have more data after first read")
	}
	if streamer.Position() != n {
		t.Fatalf("expected position %d, got %d", n, streamer.Position())
	}
	if samples[0][0] == 0 {
		t.Fatal("expected non-zero first sample")
	}
	if samples[0][0] != samples[0][1] {
		t.Fatalf("expected mono sample mirrored to both channels, got %v", samples[0])
	}
}

// 验证 fallback streamer 的 Seek 行为
//
// 后续播放系统可能需要重播或复用 buffer，Seek 必须能回到指定采样帧
func TestPCM32WAVStreamerSeek(t *testing.T) {
	file, err := os.Open(filepath.Join("..", "..", "assets", "audio", "plant_harvest.wav"))
	if err != nil {
		t.Fatalf("open test wav failed: %v", err)
	}
	defer file.Close()

	streamer, _, err := decodePCM32WAV(file)
	if err != nil {
		t.Fatalf("decode 32-bit pcm wav failed: %v", err)
	}
	defer streamer.Close()

	firstRead := make([][2]float64, 1)
	if n, _ := streamer.Stream(firstRead); n != 1 {
		t.Fatalf("expected one sample, got %d", n)
	}
	if err := streamer.Seek(0); err != nil {
		t.Fatalf("seek to start failed: %v", err)
	}

	secondRead := make([][2]float64, 1)
	if n, _ := streamer.Stream(secondRead); n != 1 {
		t.Fatalf("expected one sample after seek, got %d", n)
	}
	if firstRead[0] != secondRead[0] {
		t.Fatalf("expected seek to replay first sample, got %v then %v", firstRead[0], secondRead[0])
	}
}

// 验证 signed little-endian int32 采样到 beep 浮点采样范围的映射
func TestDecodePCM32SampleRange(t *testing.T) {
	cases := []struct {
		name string
		data []byte
		want float64
	}{
		{name: "zero", data: []byte{0x00, 0x00, 0x00, 0x00}, want: 0},
		{name: "max positive", data: []byte{0xff, 0xff, 0xff, 0x7f}, want: 1},
		{name: "max negative", data: []byte{0x00, 0x00, 0x00, 0x80}, want: -1},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := decodePCM32Sample(tc.data)
			if math.Abs(got-tc.want) > 0.000001 {
				t.Fatalf("expected %f, got %f", tc.want, got)
			}
		})
	}
}
