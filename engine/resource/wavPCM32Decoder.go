package resource

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"

	"github.com/gopxl/beep/v2"
)

// 解码 beep wav 当前未覆盖的 32-bit PCM wav
//
// 当前资源中存在标准 RIFF/WAVE PCM 文件，但采样精度是 32-bit。
// beep 的 wav 解码器优先负责通用路径，这里只作为窄范围 fallback：
// 只接受 audioFormat=1 的 PCM、只接受 32-bit、只输出 beep.StreamSeekCloser
func decodePCM32WAV(reader io.Reader) (beep.StreamSeekCloser, beep.Format, error) {
	var riff [4]byte
	if _, err := io.ReadFull(reader, riff[:]); err != nil {
		return nil, beep.Format{}, err
	}
	if string(riff[:]) != "RIFF" {
		return nil, beep.Format{}, errors.New("wav: missing RIFF marker")
	}
	if _, err := io.CopyN(io.Discard, reader, 4); err != nil {
		return nil, beep.Format{}, err
	}
	var wave [4]byte
	if _, err := io.ReadFull(reader, wave[:]); err != nil {
		return nil, beep.Format{}, err
	}
	if string(wave[:]) != "WAVE" {
		return nil, beep.Format{}, errors.New("wav: missing WAVE marker")
	}

	var format beep.Format
	var data []byte
	for {
		chunkID, chunkData, ok, err := readWAVChunk(reader)
		if err != nil {
			return nil, beep.Format{}, err
		}
		if !ok {
			break
		}

		switch chunkID {
		case "fmt ":
			parsedFormat, err := parsePCM32WAVFormat(chunkData)
			if err != nil {
				return nil, beep.Format{}, err
			}
			format = parsedFormat
		case "data":
			data = chunkData
		}
	}

	if format.SampleRate == 0 {
		return nil, beep.Format{}, errors.New("wav: missing fmt chunk")
	}
	if len(data) == 0 {
		return nil, beep.Format{}, errors.New("wav: missing data chunk")
	}
	return &pcm32WAVStreamer{format: format, data: data}, format, nil
}

// 读取一个 WAV chunk
//
// 返回 ok=false 表示已经到达文件尾部；WAV chunk 使用 2 字节对齐，
// 所以奇数字节长度的 chunk 后面会有一个 padding 字节需要跳过
func readWAVChunk(reader io.Reader) (string, []byte, bool, error) {
	var chunkID [4]byte
	if _, err := io.ReadFull(reader, chunkID[:]); err != nil {
		if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
			return "", nil, false, nil
		}
		return "", nil, false, err
	}

	var chunkSize uint32
	if err := binary.Read(reader, binary.LittleEndian, &chunkSize); err != nil {
		return "", nil, false, err
	}
	chunkData := make([]byte, chunkSize)
	if _, err := io.ReadFull(reader, chunkData); err != nil {
		return "", nil, false, err
	}
	if chunkSize%2 == 1 {
		if _, err := io.CopyN(io.Discard, reader, 1); err != nil {
			return "", nil, false, err
		}
	}

	return string(chunkID[:]), chunkData, true, nil
}

// 解析 fmt chunk 并确认它正好是当前 fallback 支持的格式
func parsePCM32WAVFormat(chunkData []byte) (beep.Format, error) {
	if len(chunkData) < 16 {
		return beep.Format{}, errors.New("wav: fmt chunk too short")
	}
	audioFormat := binary.LittleEndian.Uint16(chunkData[0:2])
	numChannels := binary.LittleEndian.Uint16(chunkData[2:4])
	sampleRate := binary.LittleEndian.Uint32(chunkData[4:8])
	blockAlign := binary.LittleEndian.Uint16(chunkData[12:14])
	bitsPerSample := binary.LittleEndian.Uint16(chunkData[14:16])
	if audioFormat != 1 {
		return beep.Format{}, fmt.Errorf("wav: unsupported fallback format type %d", audioFormat)
	}
	if numChannels == 0 {
		return beep.Format{}, errors.New("wav: invalid channel count")
	}
	if bitsPerSample != 32 {
		return beep.Format{}, fmt.Errorf("wav: fallback expects 32-bit PCM, got %d-bit", bitsPerSample)
	}
	if blockAlign != numChannels*4 {
		return beep.Format{}, fmt.Errorf("wav: invalid block align %d", blockAlign)
	}
	return beep.Format{
		SampleRate:  beep.SampleRate(sampleRate),
		NumChannels: int(numChannels),
		Precision:   4,
	}, nil
}

// 将 32-bit PCM wav 数据按 beep.StreamSeekCloser 接口提供给上层
//
// WAV 数据本身已经完整读入内存；这个 streamer 只负责按帧推进位置，
// 并把 signed little-endian int32 采样转换成 beep 使用的 [-1, 1] float64
type pcm32WAVStreamer struct {
	format beep.Format
	data   []byte
	pos    int
}

func (s *pcm32WAVStreamer) Stream(samples [][2]float64) (int, bool) {
	frameSize := s.format.Width()
	if frameSize <= 0 || s.pos >= len(s.data) {
		return 0, false
	}

	n := 0
	for n < len(samples) && s.pos+frameSize <= len(s.data) {
		frame := s.data[s.pos : s.pos+frameSize]
		left := decodePCM32Sample(frame[0:4])
		right := left
		if s.format.NumChannels >= 2 {
			right = decodePCM32Sample(frame[4:8])
		}
		samples[n] = [2]float64{left, right}
		s.pos += frameSize
		n++
	}
	return n, s.pos < len(s.data)
}

// 确保 pcm32WAVStreamer 实现 beep.StreamSeekCloser 接口
var _ beep.StreamSeekCloser = (*pcm32WAVStreamer)(nil)

// 返回流式读取过程中记录的错误
//
// 当前 streamer 的数据已经完整在内存中，读取和转换不会产生延迟错误
func (s *pcm32WAVStreamer) Err() error {
	return nil
}

// 释放 streamer 持有的资源
//
// 当前没有外部文件或设备句柄需要关闭，保留方法是为了满足 beep.StreamSeekCloser
func (s *pcm32WAVStreamer) Close() error {
	return nil
}

// 返回可读取的采样帧数量
func (s *pcm32WAVStreamer) Len() int {
	return len(s.data) / s.format.Width()
}

// 返回当前读取位置，单位为采样帧
func (s *pcm32WAVStreamer) Position() int {
	return s.pos / s.format.Width()
}

// 移动当前读取位置，单位为采样帧
func (s *pcm32WAVStreamer) Seek(position int) error {
	if position < 0 || position > s.Len() {
		return fmt.Errorf("wav: seek position %d out of range", position)
	}
	s.pos = position * s.format.Width()
	return nil
}

// 将 signed little-endian int32 PCM 采样映射到 beep 使用的 [-1, 1] 范围
func decodePCM32Sample(data []byte) float64 {
	value := int32(binary.LittleEndian.Uint32(data))
	if value < 0 {
		return float64(value) / 2147483648.0
	}
	return float64(value) / 2147483647.0
}
