package audio

import (
	"errors"
	"fmt"

	"tiny_farm/engine/abstract"
	"tiny_farm/engine/utils/defs"
	emath "tiny_farm/engine/utils/math"

	"github.com/go-gl/mathgl/mgl32"
	"github.com/gopxl/beep/v2"
)

// 提供音频播放、音乐控制和音量参数的统一入口
type AudioPlayer struct {
	// 提供音效和音乐 buffer 的资源入口
	resourceManager abstract.IResourceManager
	// 负责把 streamer 提交给实际播放设备
	backend *speakerBackend
	// 音效通道音量，范围 0..1
	soundVolume float64
	// 背景音乐通道音量，范围 0..1
	musicVolume float64
	// 2D 空间声距离衰减范围
	// 2D 音效从音源位置开始，距离越远越小声，
	// 到 spatialFalloffDistance 个世界坐标单位时衰减到静音
	// attenuation = clamp(1 - distance/falloffDistance, 0.0, 1.0)
	spatialFalloffDistance float64
	// 2D 空间声左右声像映射范围
	// 2D 空间音效的“左右声像映射范围”，也就是声音偏左耳还是右耳的距离尺度
	// pan = clamp(delta.X / spatialPanRange, -1.0, 1.0)
	// 其中 delta.X = 音源X - 听者X
	spatialPanRange float64
	// 当前仍被播放器跟踪的短音效实例
	activeSounds []*playbackInstance
	// 当前背景音乐实例
	music *playbackInstance
	// 当前背景音乐资源 key
	currentMusicKey defs.ResourceKey
	// 标记播放器是否已经关闭
	closed bool
}

// 确保 AudioPlayer 实现 IAudioPlayer 接口
var _ abstract.IAudioPlayer = (*AudioPlayer)(nil)

// 保存单个播放实例的可变控制节点
type playbackInstance struct {
	// 暂停或停止时操作的控制节点
	ctrl *beep.Ctrl
	// 当前实例使用的线性音量节点
	volume *linearVolume
	// 播放完成后由回调标记
	finished bool
}

// 对 streamer 输出做线性增益调整
//
// beep/effects.Volume 使用指数音量语义，这里保留游戏配置中更直观的 0..1 线性音量
type linearVolume struct {
	// 被包装的音频流
	Streamer beep.Streamer
	// 线性增益，0.0 表示静音，1.0 表示原始音量
	Gain float64
}

// 确保 linearVolume 实现 beep.Streamer 接口
var _ beep.Streamer = (*linearVolume)(nil)

// 输出按线性增益调整后的采样
func (v *linearVolume) Stream(samples [][2]float64) (int, bool) {
	if v == nil || v.Streamer == nil {
		return 0, false
	}
	// 缓冲区大小来源于后端初始化(speaker.Init)；实际写入大小以 n 为准
	n, ok := v.Streamer.Stream(samples)
	gain := v.Gain
	if gain < 0.0 {
		gain = 0.0
	}
	// 本质上就是把每个采样点的左右声道振幅按比例缩放
	for i := range samples[:n] {
		samples[i][0] *= gain
		samples[i][1] *= gain
	}
	return n, ok
}

// 返回底层 streamer 的播放错误
func (v *linearVolume) Err() error {
	if v == nil || v.Streamer == nil {
		return nil
	}
	return v.Streamer.Err()
}

// 对 streamer 输出做简化左右声像调整
type panStreamer struct {
	// 被包装的音频流
	Streamer beep.Streamer
	// 声像位置，-1.0 表示最左，1.0 表示最右
	Pan float64
}

// 确保 panStreamer 实现 beep.Streamer 接口
var _ beep.Streamer = (*panStreamer)(nil)

// 输出按左右声像调整后的采样
func (p *panStreamer) Stream(samples [][2]float64) (int, bool) {
	if p == nil || p.Streamer == nil {
		return 0, false
	}
	n, ok := p.Streamer.Stream(samples)
	leftGain, rightGain := panGains(p.Pan)
	for i := range samples[:n] {
		samples[i][0] *= leftGain
		samples[i][1] *= rightGain
	}
	return n, ok
}

// 返回底层 streamer 的播放错误
func (p *panStreamer) Err() error {
	if p == nil || p.Streamer == nil {
		return nil
	}
	return p.Streamer.Err()
}

// 使用默认配置路径创建音频播放器
func NewAudioPlayer(resourceManager abstract.IResourceManager, configPath string) (*AudioPlayer, error) {
	if resourceManager == nil {
		return nil, errors.New("audio resource manager is nil")
	}
	if configPath == "" {
		configPath = DefaultConfigPath
	}
	config, err := LoadConfig(configPath)
	if err != nil {
		return nil, err
	}
	return newAudioPlayer(resourceManager, config, newSpeakerBackend())
}

// 按配置和播放后端创建音频播放器
func newAudioPlayer(resourceManager abstract.IResourceManager, config Config, backend *speakerBackend) (*AudioPlayer, error) {
	if resourceManager == nil {
		return nil, errors.New("audio resource manager is nil")
	}
	if backend == nil {
		return nil, errors.New("audio playback backend is nil")
	}
	return &AudioPlayer{
		resourceManager:        resourceManager,
		backend:                backend,
		soundVolume:            emath.Clamp(config.SoundVolume, 0.0, 1.0),
		musicVolume:            emath.Clamp(config.MusicVolume, 0.0, 1.0),
		spatialFalloffDistance: emath.Clamp(config.Spatial.FalloffDistance, 0.0, config.Spatial.FalloffDistance),
		spatialPanRange:        emath.Clamp(config.Spatial.PanRange, 0.0, config.Spatial.PanRange),
	}, nil
}

// 释放播放器持有的播放实例和底层播放设备
func (p *AudioPlayer) Close() {
	if p == nil || p.closed {
		return
	}

	p.backend.Lock()
	for _, sound := range p.activeSounds {
		stopInstance(sound)
	}
	stopInstance(p.music)
	p.activeSounds = nil
	p.music = nil
	p.currentMusicKey = ""
	p.backend.Unlock()

	p.backend.Clear()
	p.backend.Close()
	p.closed = true
}

// 播放一次短音效
//
// 当前每次播放都会从缓存 buffer 创建独立 streamer，允许同一音效多实例重叠播放
func (p *AudioPlayer) PlaySound(key defs.ResourceKey, paths ...string) error {
	if p == nil {
		return errors.New("audio player is nil")
	}
	if p.closed {
		return errors.New("audio player is closed")
	}
	handle, err := p.resourceManager.LoadSound(key, paths...)
	if err != nil {
		return err
	}
	streamer, format, err := p.streamerFromHandle(handle)
	if err != nil {
		return err
	}
	if err := p.ensureBackend(format.SampleRate); err != nil {
		return err
	}

	p.backend.Lock()
	p.cleanupFinishedSounds()
	p.backend.Unlock()

	instance := p.play(streamer, format, p.soundVolume)
	p.activeSounds = append(p.activeSounds, instance)
	return nil
}

// 播放带 2D 空间参数的短音效
func (p *AudioPlayer) PlaySound2D(key defs.ResourceKey, source mgl32.Vec2, listener mgl32.Vec2, paths ...string) error {
	if p == nil {
		return errors.New("audio player is nil")
	}
	if p.closed {
		return errors.New("audio player is closed")
	}
	handle, err := p.resourceManager.LoadSound(key, paths...)
	if err != nil {
		return err
	}
	streamer, format, err := p.streamerFromHandle(handle)
	if err != nil {
		return err
	}
	attenuation, pan := p.spatialParams(source, listener)
	playStream := beep.Streamer(streamer)
	if p.spatialPanRange > 0 {
		playStream = &panStreamer{
			Streamer: streamer,
			Pan:      pan,
		}
	}
	if err := p.ensureBackend(format.SampleRate); err != nil {
		return err
	}

	p.backend.Lock()
	p.cleanupFinishedSounds()
	p.backend.Unlock()

	instance := p.play(playStream, format, p.soundVolume*attenuation)
	p.activeSounds = append(p.activeSounds, instance)
	return nil
}

// 播放背景音乐
//
// 当前只维护一个当前音乐实例，重复播放同一 key 时直接复用已有实例
func (p *AudioPlayer) PlayMusic(key defs.ResourceKey, loop bool, fadeInMS int, paths ...string) error {
	if p == nil {
		return errors.New("audio player is nil")
	}
	if p.closed {
		return errors.New("audio player is closed")
	}
	if p.music != nil && p.currentMusicKey == key {
		return nil
	}
	// 淡入淡出参数先保留在 API 上，当前阶段暂不实现 cross-fade
	_ = fadeInMS

	handle, err := p.resourceManager.LoadMusic(key, paths...)
	if err != nil {
		return err
	}
	streamer, format, err := p.streamerFromHandle(handle)
	if err != nil {
		return err
	}
	playStream := beep.Streamer(streamer)
	if loop {
		playStream, err = beep.Loop2(streamer)
		if err != nil {
			return err
		}
	}
	if err := p.ensureBackend(format.SampleRate); err != nil {
		return err
	}

	p.backend.Lock()
	stopInstance(p.music)
	p.backend.Unlock()

	instance := p.play(playStream, format, p.musicVolume)

	p.backend.Lock()
	p.music = instance
	p.currentMusicKey = key
	p.backend.Unlock()
	return nil
}

// 停止当前背景音乐
//
// 当前阶段保留淡出参数但不做 cross-fade，后续音乐控制阶段再补完整淡出语义
func (p *AudioPlayer) StopMusic(fadeOutMS int) {
	if p == nil || p.music == nil {
		return
	}
	_ = fadeOutMS
	p.backend.Lock()
	stopInstance(p.music)
	p.music = nil
	p.currentMusicKey = ""
	p.backend.Unlock()
}

// 暂停当前背景音乐
func (p *AudioPlayer) PauseMusic() {
	if p == nil || p.music == nil || p.music.ctrl == nil {
		return
	}
	p.backend.Lock()
	p.music.ctrl.Paused = true
	p.backend.Unlock()
}

// 恢复当前背景音乐
func (p *AudioPlayer) ResumeMusic() {
	if p == nil || p.music == nil || p.music.ctrl == nil {
		return
	}
	p.backend.Lock()
	p.music.ctrl.Paused = false
	p.backend.Unlock()
}

// 设置音效通道音量
func (p *AudioPlayer) SetSoundVolume(volume float64) {
	if p == nil {
		return
	}

	if len(p.activeSounds) == 0 {
		return
	}

	p.backend.Lock()
	p.soundVolume = emath.Clamp(volume, 0.0, 1.0)
	p.cleanupFinishedSounds()
	for _, sound := range p.activeSounds {
		if sound != nil && sound.volume != nil {
			sound.volume.Gain = p.soundVolume
		}
	}
	p.backend.Unlock()
}

// 设置背景音乐通道音量
func (p *AudioPlayer) SetMusicVolume(volume float64) {
	if p == nil {
		return
	}

	if p.music == nil {
		return
	}

	p.backend.Lock()
	p.musicVolume = emath.Clamp(volume, 0.0, 1.0)
	if p.music != nil && p.music.volume != nil {
		p.music.volume.Gain = p.musicVolume
	}
	p.backend.Unlock()
}

// 返回当前音效通道音量
func (p *AudioPlayer) SoundVolume() float64 {
	if p == nil {
		return 0
	}
	return p.soundVolume
}

// 返回当前背景音乐通道音量
func (p *AudioPlayer) MusicVolume() float64 {
	if p == nil {
		return 0
	}
	return p.musicVolume
}

// 返回当前 2D 空间声距离衰减范围
func (p *AudioPlayer) SpatialFalloffDistance() float64 {
	if p == nil {
		return 0
	}
	return p.spatialFalloffDistance
}

// 返回当前 2D 空间声左右声像映射范围
func (p *AudioPlayer) SpatialPanRange() float64 {
	if p == nil {
		return 0
	}
	return p.spatialPanRange
}

// 从音频缓存句柄创建可播放流
func (p *AudioPlayer) streamerFromHandle(handle abstract.IAudioBufferHandle) (beep.StreamSeeker, beep.Format, error) {
	if handle == nil {
		return nil, beep.Format{}, errors.New("audio buffer is nil")
	}
	streamer, ok := handle.Streamer()
	if !ok {
		return nil, beep.Format{}, errors.New("audio buffer is empty")
	}
	return streamer, handle.Format(), nil
}

// 根据音源和监听者位置计算空间音效参数
func (p *AudioPlayer) spatialParams(source mgl32.Vec2, listener mgl32.Vec2) (float64, float64) {
	if p == nil {
		return 1, 0
	}
	delta := source.Sub(listener)
	attenuation := 1.0
	if p.spatialFalloffDistance > 0 {
		attenuation = emath.Clamp((1 - float64(delta.Len())/p.spatialFalloffDistance), 0.0, 1.0)
	}
	pan := 0.0
	if p.spatialPanRange > 0 {
		pan = emath.Clamp(float64(delta.X())/p.spatialPanRange, -1.0, 1.0)
	}
	return attenuation, pan
}

// 确保底层播放设备已经按指定采样率初始化
func (p *AudioPlayer) ensureBackend(sampleRate beep.SampleRate) error {
	if sampleRate <= 0 {
		return errors.New("audio sample rate is invalid")
	}
	if p.backend.Initialized() {
		return nil
	}
	if err := p.backend.Init(sampleRate); err != nil {
		return fmt.Errorf("init audio playback backend: %w", err)
	}
	return nil
}

// 提交单个播放实例到后端
func (p *AudioPlayer) play(streamer beep.Streamer, format beep.Format, volume float64) *playbackInstance {
	// 如果音频文件的采样率和当前播放设备采样率不同，重采样
	if p.backend.SampleRate() != 0 && format.SampleRate != p.backend.SampleRate() {
		streamer = beep.Resample(4, format.SampleRate, p.backend.SampleRate(), streamer)
	}
	// 创建可控制节点，用于暂停、恢复、停止
	ctrl := &beep.Ctrl{Streamer: streamer}
	// 包一层线性音量节点，用于调整音量
	volumeNode := &linearVolume{
		Streamer: ctrl,
		Gain:     emath.Clamp(volume, 0.0, 1.0),
	}
	// 创建播放实例句柄
	instance := &playbackInstance{
		ctrl:   ctrl,
		volume: volumeNode,
	}
	// 提交播放实例到后端
	p.backend.Play(beep.Seq(volumeNode, beep.Callback(func() {
		instance.finished = true
	})))
	return instance
}

// 移除已经播放完成的短音效实例
func (p *AudioPlayer) cleanupFinishedSounds() {
	if len(p.activeSounds) == 0 {
		return
	}
	active := p.activeSounds[:0]
	for _, sound := range p.activeSounds {
		if sound != nil && !sound.finished {
			active = append(active, sound)
		}
	}
	p.activeSounds = active
}

// 立即停止单个播放实例
func stopInstance(instance *playbackInstance) {
	if instance == nil || instance.ctrl == nil {
		return
	}
	instance.ctrl.Streamer = nil
	instance.finished = true
}

// 根据 pan 计算左右声道增益
//
// 当前使用线性削弱模型，保持中间位置不改变原始左右声道
func panGains(pan float64) (float64, float64) {
	pan = emath.Clamp(pan, -1.0, 1.0)
	if pan < 0.0 {
		return 1.0, 1.0 + pan
	}
	if pan > 0.0 {
		return 1.0 - pan, 1.0
	}
	return 1.0, 1.0
}
