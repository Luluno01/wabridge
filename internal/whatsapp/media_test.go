package whatsapp

import (
	"encoding/binary"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// buildMinimalOggOpus constructs a minimal valid Ogg Opus file with the given
// duration (in seconds at 48000 Hz sample rate) for testing purposes.
func buildMinimalOggOpus(durationSeconds float64) []byte {
	var data []byte

	// --- Page 0: OpusHead ---
	data = append(data, buildOggPage(0, 0, 0, buildOpusHead(48000, 0))...)

	// --- Page 1: OpusTags (empty, required by spec) ---
	tags := []byte("OpusTags")
	tags = append(tags, 0, 0, 0, 0) // vendor string length = 0
	tags = append(tags, 0, 0, 0, 0) // comment count = 0
	data = append(data, buildOggPage(0, 1, 0, tags)...)

	// --- Page 2: Audio data with final granule ---
	granule := uint64(durationSeconds * 48000)
	audioData := make([]byte, 100) // dummy audio payload
	data = append(data, buildOggPage(0, 2, granule, audioData)...)

	return data
}

// buildOpusHead constructs an OpusHead packet.
func buildOpusHead(sampleRate uint32, preSkip uint16) []byte {
	head := []byte("OpusHead")
	head = append(head, 1)    // version
	head = append(head, 2)    // channels
	ps := make([]byte, 2)
	binary.LittleEndian.PutUint16(ps, preSkip)
	head = append(head, ps...)
	sr := make([]byte, 4)
	binary.LittleEndian.PutUint32(sr, sampleRate)
	head = append(head, sr...)
	head = append(head, 0, 0) // output gain
	head = append(head, 0)    // channel mapping family
	return head
}

// buildOggPage constructs a minimal Ogg page.
func buildOggPage(streamSerial, pageSeqNum uint32, granulePos uint64, payload []byte) []byte {
	// Ogg page header: 27 bytes + segment table + segments
	numSegments := (len(payload) / 255) + 1
	segmentTable := make([]byte, numSegments)
	remaining := len(payload)
	for i := 0; i < numSegments; i++ {
		if remaining >= 255 {
			segmentTable[i] = 255
			remaining -= 255
		} else {
			segmentTable[i] = byte(remaining)
			remaining = 0
		}
	}

	header := make([]byte, 27)
	copy(header[0:4], "OggS")
	header[4] = 0 // version
	header[5] = 0 // header type

	binary.LittleEndian.PutUint64(header[6:14], granulePos)
	binary.LittleEndian.PutUint32(header[14:18], streamSerial)
	binary.LittleEndian.PutUint32(header[18:22], pageSeqNum)
	// CRC (bytes 22-25) left as zero for testing
	header[26] = byte(numSegments)

	var page []byte
	page = append(page, header...)
	page = append(page, segmentTable...)
	page = append(page, payload...)
	return page
}

func TestAnalyzeOggOpus_ValidFile(t *testing.T) {
	data := buildMinimalOggOpus(5.0)
	duration, err := AnalyzeOggOpus(data)
	require.NoError(t, err)
	assert.Equal(t, uint32(5), duration)
}

func TestAnalyzeOggOpus_LongDuration(t *testing.T) {
	data := buildMinimalOggOpus(120.0)
	duration, err := AnalyzeOggOpus(data)
	require.NoError(t, err)
	assert.Equal(t, uint32(120), duration)
}

func TestAnalyzeOggOpus_ShortDuration(t *testing.T) {
	// Very short: 0.1 seconds -> should be clamped to 1
	data := buildMinimalOggOpus(0.1)
	duration, err := AnalyzeOggOpus(data)
	require.NoError(t, err)
	assert.Equal(t, uint32(1), duration, "should be clamped to minimum of 1")
}

func TestAnalyzeOggOpus_InvalidFile(t *testing.T) {
	_, err := AnalyzeOggOpus([]byte("not an ogg file"))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not a valid Ogg file")
}

func TestAnalyzeOggOpus_TooShort(t *testing.T) {
	_, err := AnalyzeOggOpus([]byte("Og"))
	assert.Error(t, err)
}

func TestAnalyzeOggOpus_EmptyData(t *testing.T) {
	_, err := AnalyzeOggOpus(nil)
	assert.Error(t, err)
}

func TestPlaceholderWaveform_Length(t *testing.T) {
	wf := PlaceholderWaveform(10)
	assert.Len(t, wf, 64)
}

func TestPlaceholderWaveform_Deterministic(t *testing.T) {
	wf1 := PlaceholderWaveform(30)
	wf2 := PlaceholderWaveform(30)
	assert.Equal(t, wf1, wf2, "same duration should produce identical waveforms")
}

func TestPlaceholderWaveform_DifferentDurations(t *testing.T) {
	wf1 := PlaceholderWaveform(10)
	wf2 := PlaceholderWaveform(60)
	assert.NotEqual(t, wf1, wf2, "different durations should produce different waveforms")
}

func TestPlaceholderWaveform_ValuesInRange(t *testing.T) {
	for _, duration := range []uint32{1, 10, 30, 60, 120, 300} {
		wf := PlaceholderWaveform(duration)
		for i, v := range wf {
			assert.LessOrEqual(t, v, byte(100), "value at index %d for duration %d should be <= 100", i, duration)
		}
	}
}

func TestPlaceholderWaveform_ThreadSafety(t *testing.T) {
	// Run multiple goroutines to verify no data races (run with -race)
	done := make(chan []byte, 10)
	for i := 0; i < 10; i++ {
		go func() {
			done <- PlaceholderWaveform(42)
		}()
	}
	var results [][]byte
	for i := 0; i < 10; i++ {
		results = append(results, <-done)
	}
	// All results should be identical
	for i := 1; i < len(results); i++ {
		assert.Equal(t, results[0], results[i])
	}
}

func TestExtractDirectPathFromURL(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		expected string
	}{
		{
			name:     "standard WhatsApp URL",
			url:      "https://mmg.whatsapp.net/v/t62.7118-24/file.enc?ccb=11-4&oh=abc",
			expected: "/v/t62.7118-24/file.enc",
		},
		{
			name:     "no query params",
			url:      "https://mmg.whatsapp.net/v/t62/file.enc",
			expected: "/v/t62/file.enc",
		},
		{
			name:     "non-whatsapp URL",
			url:      "https://example.com/file.jpg",
			expected: "https://example.com/file.jpg",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractDirectPathFromURL(tt.url)
			assert.Equal(t, tt.expected, result)
		})
	}
}

