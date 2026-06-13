package audio

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"

	emath "tiny_farm/engine/utils/math"
)

const (
	// 启动时默认读取的音频配置路径
	DefaultConfigPath = "config/audio.json"

	// 背景音乐音量默认值，范围 0..1
	defaultMusicVolume = 0.2
	// 音效音量默认值，范围 0..1
	defaultSoundVolume = 0.5
	// 2D 空间声距离衰减范围默认值，单位使用世界坐标
	defaultSpatialFalloffDistance = 320.0
	// 2D 空间声左右声像映射范围默认值，单位使用世界坐标
	defaultSpatialPanRange = 160.0
)

// 保存音频播放器运行参数
//
// 当前只覆盖 copy_source 第 16 节使用的音量和 2D 空间声参数
type Config struct {
	// 背景音乐音量，范围 0..1
	MusicVolume float64
	// 音效音量，范围 0..1
	SoundVolume float64
	// 2D 空间声参数
	Spatial SpatialConfig
}

// 保存简化 2D 空间声参数
type SpatialConfig struct {
	// 声音距离衰减范围，单位使用世界坐标
	FalloffDistance float64
	// 左右声像映射范围，单位使用世界坐标
	PanRange float64
}

// 配置
// 用指针区分字段缺失和显式写入 0
type rawConfig struct {
	// 背景音乐音量覆盖值
	MusicVolume *float64 `json:"music_volume"`
	// 音效音量覆盖值
	SoundVolume *float64 `json:"sound_volume"`
	// 2D 空间声参数覆盖值
	Spatial *rawSpatialConfig `json:"spatial"`
}

// 保存 JSON 中可选的 2D 空间声参数
type rawSpatialConfig struct {
	// 声音距离衰减范围覆盖值
	FalloffDistance *float64 `json:"falloff_distance"`
	// 左右声像映射范围覆盖值
	PanRange *float64 `json:"pan_range"`
}

// 返回播放器默认配置
func DefaultConfig() Config {
	return Config{
		MusicVolume: defaultMusicVolume,
		SoundVolume: defaultSoundVolume,
		Spatial: SpatialConfig{
			FalloffDistance: defaultSpatialFalloffDistance,
			PanRange:        defaultSpatialPanRange,
		},
	}
}

// 从 JSON 文件读取音频配置
//
// 当前缺失文件返回默认值，避免开发阶段少一个配置文件就中断启动
// JSON 格式错误或字段类型错误仍返回明确错误，方便尽早发现配置写错
func LoadConfig(path string) (Config, error) {
	config := DefaultConfig()
	if path == "" {
		return config, errors.New("audio config path is empty")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return config, nil
		}
		return config, fmt.Errorf("load audio config %q: %w", path, err)
	}
	if err := config.ApplyJSON(data, path); err != nil {
		return config, err
	}
	return config, nil
}

// 把 JSON 配置覆盖到当前配置
//
// 当前同时兼容根对象直接写音频字段，以及 copy_source 一样包一层 audio
func (c *Config) ApplyJSON(data []byte, source string) error {
	if c == nil {
		return errors.New("audio config is nil")
	}
	var root map[string]json.RawMessage
	if err := json.Unmarshal(data, &root); err != nil {
		return fmt.Errorf("parse audio config %q: %w", source, err)
	}
	if root == nil {
		return fmt.Errorf("parse audio config %q: root must be object", source)
	}

	rawData := data
	if wrapped, ok := root["audio"]; ok {
		rawData = wrapped
	}

	var raw rawConfig
	if err := json.Unmarshal(rawData, &raw); err != nil {
		return fmt.Errorf("parse audio config %q: %w", source, err)
	}
	c.applyRaw(raw)
	return nil
}

// 应用已经解析出来的可选覆盖项
//
// 当前字段缺失时保留原配置，适合先载入默认值再按 JSON 覆盖
func (c *Config) applyRaw(raw rawConfig) {
	if raw.MusicVolume != nil {
		c.MusicVolume = emath.Clamp(*raw.MusicVolume, 0.0, 1.0)
	}
	if raw.SoundVolume != nil {
		c.SoundVolume = emath.Clamp(*raw.SoundVolume, 0.0, 1.0)
	}
	if raw.Spatial != nil {
		if raw.Spatial.FalloffDistance != nil {
			c.Spatial.FalloffDistance = emath.Clamp(*raw.Spatial.FalloffDistance, 0.0, *raw.Spatial.FalloffDistance)
		}
		if raw.Spatial.PanRange != nil {
			c.Spatial.PanRange = emath.Clamp(*raw.Spatial.PanRange, 0.0, *raw.Spatial.PanRange)
		}
	}
}
