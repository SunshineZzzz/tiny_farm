package resource

import (
	"encoding/json"
	"fmt"
	"os"
)

// 标识资源映射中的稳定语义 key
//
// 当前先用字符串保持调用和测试简单，后续如需数字 ID 可在内部补 hash
type ResourceKey string

// 保存 resource_mapping.json 中当前阶段需要识别的资源路径
//
// 当前只解析 texture、sound、music，UI preset 字段先保留给后续阶段使用
type resourceMapping struct {
	// sound key -> 文件路径
	Sound map[ResourceKey]string
	// music key -> 文件路径
	Music map[ResourceKey]string
	// texture key -> 文件路径
	Texture map[ResourceKey]string
}

// 从 JSON 文件加载资源映射
//
// 当前只做格式解析和校验，不触发任何真实资源加载
func loadResourceMapping(path string) (*resourceMapping, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("load resource mapping %q: %w", path, err)
	}
	return parseResourceMapping(data, path)
}

// 从 JSON 数据解析资源映射
//
// source 用于错误信息，通常传入文件路径
func parseResourceMapping(data []byte, source string) (*resourceMapping, error) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse resource mapping %q: %w", source, err)
	}
	if raw == nil {
		return nil, fmt.Errorf("parse resource mapping %q: root must be object", source)
	}

	mapping := &resourceMapping{
		Sound:   make(map[ResourceKey]string),
		Music:   make(map[ResourceKey]string),
		Texture: make(map[ResourceKey]string),
	}

	var err error
	if mapping.Sound, err = decodeResourceStringMap(raw, "sound", source); err != nil {
		return nil, err
	}
	if mapping.Music, err = decodeResourceStringMap(raw, "music", source); err != nil {
		return nil, err
	}
	if mapping.Texture, err = decodeResourceStringMap(raw, "texture", source); err != nil {
		return nil, err
	}

	return mapping, nil
}

// 解析 key 到路径的对象字段
func decodeResourceStringMap(raw map[string]json.RawMessage, field string, source string) (map[ResourceKey]string, error) {
	result := make(map[ResourceKey]string)
	value, ok := raw[field]
	if !ok {
		return result, nil
	}

	var section map[string]json.RawMessage
	if err := json.Unmarshal(value, &section); err != nil || section == nil {
		return nil, fmt.Errorf("parse resource mapping %q: field %q must be object", source, field)
	}

	for key, rawPath := range section {
		var path string
		if err := json.Unmarshal(rawPath, &path); err != nil {
			return nil, fmt.Errorf("parse resource mapping %q: field %q.%q must be string", source, field, key)
		}
		result[ResourceKey(key)] = path
	}

	return result, nil
}
