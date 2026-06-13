package resource

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
	"tiny_farm/engine/abstract"

	"github.com/gopxl/beep/v2"
	"github.com/gopxl/beep/v2/mp3"
	"github.com/gopxl/beep/v2/vorbis"
	"github.com/gopxl/beep/v2/wav"
)

// 持有已解码音频数据的稳定句柄
//
// 当前只缓存 buffer，不负责播放、循环、音量或混音策略
type AudioBufferHandle struct {
	// 解码后的音频采样缓存
	buffer *beep.Buffer
	// 音频采样格式
	// format.SampleRate，音频采样率，每秒钟从连续声音信号中“取样”的次数，单位是 Hz（赫兹）
	// 简单理解，采样率 = 每秒采集多少个“声音快照”
	// format.NumChannels，音频通道数，1 是单声道，2 是立体声
	format beep.Format
	// 音频来源路径
	sourcePath string
}

// 确保AudioBufferHandle实现IAudioBufferHandle接口
var _ abstract.IAudioBufferHandle = (*AudioBufferHandle)(nil)

// 管理音效和音乐资源的解码缓存
//
// 当前只负责把文件解码进内存 buffer，具体播放链路留给后续 AudioPlayer
type audioManager struct {
	// 音效缓存
	sounds map[ResourceKey]*AudioBufferHandle
	// 背景音乐缓存
	music map[ResourceKey]*AudioBufferHandle
}

// 创建音频资源管理器
func newAudioManager() *audioManager {
	return &audioManager{
		sounds: make(map[ResourceKey]*AudioBufferHandle),
		music:  make(map[ResourceKey]*AudioBufferHandle),
	}
}

// 获取音效资源，未加载时尝试加载 key 或 paths[0]
func (m *audioManager) loadSound(key ResourceKey, paths ...string) (*AudioBufferHandle, error) {
	if m == nil {
		return nil, errors.New("audio manager is nil")
	}
	return m.loadAudio(m.sounds, "sound", key, paths...)
}

// 加载音乐并按 key 缓存
func (m *audioManager) loadMusic(key ResourceKey, paths ...string) (*AudioBufferHandle, error) {
	if m == nil {
		return nil, errors.New("audio manager is nil")
	}
	return m.loadAudio(m.music, "music", key, paths...)
}

// 卸载指定音效缓存
func (m *audioManager) unloadSound(key ResourceKey) {
	if m == nil {
		return
	}
	delete(m.sounds, key)
}

// 卸载指定音乐缓存
func (m *audioManager) unloadMusic(key ResourceKey) {
	if m == nil {
		return
	}
	delete(m.music, key)
}

// 清空全部音效缓存
func (m *audioManager) clearSounds() {
	if m == nil {
		return
	}
	m.sounds = make(map[ResourceKey]*AudioBufferHandle)
}

// 清空全部音乐缓存
func (m *audioManager) clearMusic() {
	if m == nil {
		return
	}
	m.music = make(map[ResourceKey]*AudioBufferHandle)
}

// 清空全部音频缓存
func (m *audioManager) clear() {
	if m == nil {
		return
	}
	m.clearSounds()
	m.clearMusic()
}

// 加载音频文件并写入指定缓存
func (m *audioManager) loadAudio(cache map[ResourceKey]*AudioBufferHandle, kind string, key ResourceKey, paths ...string) (*AudioBufferHandle, error) {
	if key == "" {
		return nil, fmt.Errorf("%s key is empty", kind)
	}
	if handle, ok := cache[key]; ok && handle.buffer != nil {
		return handle, nil
	}

	path := string(key)
	if len(paths) != 0 {
		path = paths[0]
	}
	if path == "" {
		return nil, fmt.Errorf("%s %q path is empty", kind, key)
	}

	handle, err := decodeAudioFile(path)
	if err != nil {
		return nil, fmt.Errorf("load %s %q from %q: %w", kind, key, path, err)
	}
	cache[key] = handle
	return handle, nil
}

// 查询指定缓存里的音频句柄
func getAudio(cache map[ResourceKey]AudioBufferHandle, key ResourceKey) (AudioBufferHandle, bool) {
	handle, ok := cache[key]
	if !ok || handle.buffer == nil {
		return AudioBufferHandle{}, false
	}
	return handle, true
}

// 解码音频文件到内存 buffer
func decodeAudioFile(path string) (*AudioBufferHandle, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	streamer, format, err := decodeAudioStream(file, path)
	if err != nil {
		return nil, err
	}
	// 关闭音频流，确保解码器内部资源及时释放
	defer streamer.Close()

	// 音频数据的内存容器，专门用来预加载整个音频文件到内存中
	// 类似于 bytes.Buffer，但存储的是音频样本，而不是字节
	buffer := beep.NewBuffer(format)
	buffer.Append(streamer)
	if err := streamer.Err(); err != nil {
		return nil, err
	}

	return &AudioBufferHandle{
		buffer:     buffer,
		format:     format,
		sourcePath: path,
	}, nil
}

// 按扩展名选择解码器
func decodeAudioStream(file *os.File, path string) (beep.StreamSeekCloser, beep.Format, error) {
	// xxx.Decode(f)
	// It simply returns a streamer that does the reading and decoding on-line (when needed).
	// The main consequence/gotcha is that you can't close the file f before you finish streaming it.

	switch strings.ToLower(filepath.Ext(path)) {
	case ".wav":
		streamer, format, err := wav.Decode(file)
		if err == nil {
			return streamer, format, nil
		}
		// 上面失败以后已经关闭了文件，需要重新打开文件
		fallbackFile, openErr := os.Open(path)
		if openErr != nil {
			return nil, beep.Format{}, fmt.Errorf("%w; open wav fallback: %v", err, openErr)
		}
		fallbackStreamer, fallbackFormat, fallbackErr := decodePCM32WAV(fallbackFile)
		fallbackFile.Close()
		if fallbackErr != nil {
			return nil, beep.Format{}, fmt.Errorf("%w; pcm32 wav fallback: %v", err, fallbackErr)
		}
		return fallbackStreamer, fallbackFormat, nil
	case ".mp3":
		return mp3.Decode(file)
	case ".ogg":
		return vorbis.Decode(file)
	default:
		return nil, beep.Format{}, fmt.Errorf("unsupported audio format %q", filepath.Ext(path))
	}
}

// 返回音频时长
func (h *AudioBufferHandle) Duration() time.Duration {
	if h.buffer == nil {
		return 0
	}
	// 时间(秒) = 样本数 / 采样率
	return h.format.SampleRate.D(h.buffer.Len())
}

// 返回已缓存采样数量
//
// 当前这里返回的是采样帧数量，不是单声道样本总数
func (h *AudioBufferHandle) SampleCount() int {
	if h.buffer == nil {
		return 0
	}
	return h.buffer.Len()
}

// 返回音频采样格式
func (h *AudioBufferHandle) Format() beep.Format {
	return h.format
}

// 返回一个从头播放到结尾的新音频流
//
// 每次调用都会创建独立 streamer，确保同一缓存可以并发播放多次
func (h *AudioBufferHandle) Streamer() (beep.StreamSeeker, bool) {
	if h == nil || h.buffer == nil {
		return nil, false
	}
	return h.buffer.Streamer(0, h.buffer.Len()), true
}

// 返回音频来源路径
func (h *AudioBufferHandle) SourcePath() string {
	return h.sourcePath
}

// 返回按 key 排序的音效调试信息
func (m *audioManager) soundDebugInfo() []AudioDebugInfo {
	if m == nil {
		return nil
	}
	return audioDebugInfo(m.sounds)
}

// 返回按 key 排序的音乐调试信息
func (m *audioManager) musicDebugInfo() []AudioDebugInfo {
	if m == nil {
		return nil
	}
	return audioDebugInfo(m.music)
}

// 返回按 key 排序的音频调试信息
func audioDebugInfo(cache map[ResourceKey]*AudioBufferHandle) []AudioDebugInfo {
	if len(cache) == 0 {
		return nil
	}

	keys := make([]ResourceKey, 0, len(cache))
	for key, handle := range cache {
		if handle.buffer == nil {
			continue
		}
		keys = append(keys, key)
	}
	slices.Sort(keys)

	info := make([]AudioDebugInfo, 0, len(keys))
	for _, key := range keys {
		handle := cache[key]
		format := handle.format
		sampleCount := handle.SampleCount()
		info = append(info, AudioDebugInfo{
			Key:         key,
			SourcePath:  handle.sourcePath,
			SampleRate:  int(format.SampleRate),
			NumChannels: format.NumChannels,
			Precision:   format.Precision,
			SampleCount: sampleCount,
			Duration:    handle.Duration(),
			MemoryBytes: estimateAudioMemoryBytes(format, sampleCount),
		})
	}
	return info
}

// 按 buffer 格式估算音频缓存内存占用
func estimateAudioMemoryBytes(format beep.Format, sampleCount int) int {
	if sampleCount <= 0 {
		return 0
	}
	width := format.Width()
	if width <= 0 {
		return 0
	}
	return sampleCount * width
}
