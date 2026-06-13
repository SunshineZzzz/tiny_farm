package system

import (
	"errors"
	"log/slog"

	"tiny_farm/engine/abstract"
	"tiny_farm/engine/ecs/component"
	"tiny_farm/engine/utils/defs"
	"tiny_farm/engine/utils/dispatch"
	"tiny_farm/engine/utils/event"

	"github.com/go-gl/mathgl/mgl32"
	"github.com/yohamta/donburi"
)

// 监听音效事件并把事件转换成实际播放调用
//
// 当前对齐 copy_source 的简化模型：无实体或实体不可用于定位时播放全局音效，有 Transform 时播放 2D 音效
type AudioSystem struct {
	// 当前场景持有的 ECS World
	world donburi.World
	// 负责提交真实播放请求的播放器
	audioPlayer abstract.IAudioPlayer
	// 提供监听者位置的相机
	camera abstract.ICamera
	// PlaySoundEvent 的监听连接
	playSoundConnection dispatch.Connection
}

// 创建监听 PlaySoundEvent 的音频系统
func NewAudioSystem(world donburi.World, dispatcher *dispatch.Dispatcher, audioPlayer abstract.IAudioPlayer, camera abstract.ICamera) (*AudioSystem, error) {
	switch {
	case world == nil:
		return nil, errors.New("audio system world is nil")
	case dispatcher == nil:
		return nil, errors.New("audio system dispatcher is nil")
	case audioPlayer == nil:
		return nil, errors.New("audio system audioPlayer is nil")
	case camera == nil:
		return nil, errors.New("audio system camera is nil")
	}

	system := &AudioSystem{
		world:       world,
		audioPlayer: audioPlayer,
		camera:      camera,
	}
	system.playSoundConnection = dispatch.SinkOf[event.PlaySoundEvent](dispatcher).Connect(system.onPlaySoundEvent)
	return system, nil
}

// 释放系统持有的事件监听连接
func (s *AudioSystem) Close() {
	if s == nil {
		return
	}
	s.playSoundConnection.Release()
	s.world = nil
	s.audioPlayer = nil
	s.camera = nil
}

// 处理播放音效事件
func (s *AudioSystem) onPlaySoundEvent(e event.PlaySoundEvent) {
	if s == nil || s.audioPlayer == nil {
		return
	}
	if e.SoundKey == "" {
		slog.Debug("ignore empty play sound event")
		return
	}

	soundKey := s.resolveSoundKey(e)
	if e.Entity == 0 {
		s.playGlobal(soundKey)
		return
	}
	if s.world == nil || !s.world.Valid(e.Entity) {
		slog.Warn("play sound entity is invalid, fallback to global sound")
		s.playGlobal(soundKey)
		return
	}

	entry := s.world.Entry(e.Entity)
	if entry == nil || !entry.Valid() || !entry.HasComponent(component.Transform) {
		slog.Warn("play sound entity has no transform, fallback to global sound")
		s.playGlobal(soundKey)
		return
	}

	transform := component.Transform.Get(entry)
	listener := mgl32.Vec2{}
	if s.camera != nil {
		listener = s.camera.Position()
	}
	if err := s.audioPlayer.PlaySound2D(soundKey, transform.Position, listener); err != nil {
		slog.Warn("play 2d sound failed", slog.Any("err", err), slog.String("key", string(soundKey)))
	}
}

// 解析事件中的音效 key
//
// 如果实体有 AudioComponent 且配置了触发表，就把触发 key 转成真实资源 key
func (s *AudioSystem) resolveSoundKey(e event.PlaySoundEvent) defs.ResourceKey {
	if s == nil || s.world == nil || e.Entity == 0 || !s.world.Valid(e.Entity) {
		return e.SoundKey
	}
	entry := s.world.Entry(e.Entity)
	if entry == nil || !entry.Valid() || !entry.HasComponent(component.Audio) {
		return e.SoundKey
	}
	audio := component.Audio.Get(entry)
	if audio == nil || len(audio.Sounds) == 0 {
		return e.SoundKey
	}
	if resolved, ok := audio.Sounds[e.SoundKey]; ok && resolved != "" {
		return resolved
	}
	return e.SoundKey
}

// 播放全局音效并记录失败信息
func (s *AudioSystem) playGlobal(soundKey defs.ResourceKey) {
	if err := s.audioPlayer.PlaySound(soundKey); err != nil {
		slog.Warn("play sound failed", slog.Any("err", err), slog.String("key", string(soundKey)))
	}
}
