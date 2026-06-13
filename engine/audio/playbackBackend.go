package audio

import (
	"time"

	"github.com/gopxl/beep/v2"
	"github.com/gopxl/beep/v2/speaker"
)

// 使用 beep/speaker 连接真实音频设备
type speakerBackend struct {
	// 当前播放设备采样率
	sampleRate beep.SampleRate
	// 标记 speaker 是否已经初始化
	initialized bool
}

// 创建真实播放后端
func newSpeakerBackend() *speakerBackend {
	return &speakerBackend{}
}

// 初始化真实播放设备
//
// 当前同一个后端只初始化一次，后续不同采样率资源由 AudioPlayer 重采样到该采样率
func (b *speakerBackend) Init(sampleRate beep.SampleRate) error {
	if b.initialized {
		return nil
	}
	// sampleRate，播放采样率，大于实际采样率时，变快、音调变高；小于实际采样率时，变慢、音调变低
	// bufferSize，播放缓冲区大小，大小影响播放延迟和内存占用
	// sampleRate.N，输入一段持续时间，算出在这段时间内，扬声器一共需要播放多少个声音样本
	if err := speaker.Init(sampleRate, sampleRate.N(time.Second/10)); err != nil {
		return err
	}
	b.sampleRate = sampleRate
	b.initialized = true
	return nil
}

// 返回真实播放设备采样率
func (b *speakerBackend) SampleRate() beep.SampleRate {
	if b == nil {
		return 0
	}
	return b.sampleRate
}

// 返回真实播放设备是否已经初始化
func (b *speakerBackend) Initialized() bool {
	return b != nil && b.initialized
}

// 提交音频流到真实播放设备
func (b *speakerBackend) Play(streamers ...beep.Streamer) {
	// asynchronous call
	speaker.Play(streamers...)
}

// 锁定真实播放设备状态
// 至少存在两个线程，音频消费线程(声卡驱动)，逻辑控制线程
// Always lock speaker for as little time as possible, to avoid playback glitches.
func (b *speakerBackend) Lock() {
	speaker.Lock()
}

// 释放真实播放设备状态锁
func (b *speakerBackend) Unlock() {
	speaker.Unlock()
}

// 清空真实播放设备队列
func (b *speakerBackend) Clear() {
	if b != nil && b.initialized {
		speaker.Clear()
	}
}

// 关闭真实播放设备
func (b *speakerBackend) Close() {
	if b == nil || !b.initialized {
		return
	}
	speaker.Close()
	b.initialized = false
	b.sampleRate = 0
}
