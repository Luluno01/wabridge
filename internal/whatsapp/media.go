package whatsapp

import (
	"context"
	"encoding/binary"
	"fmt"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"strings"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"google.golang.org/protobuf/proto"

	appstore "wabridge/internal/store"
)

// DownloadMedia downloads media for a stored message to the given output directory.
// It builds a downloadable proto message from the stored metadata and uses whatsmeow's Download.
// Returns the absolute path to the downloaded file.
func (c *Client) DownloadMedia(ctx context.Context, msg *appstore.Message, outputDir string) (string, error) {
	if msg.MediaType == nil || *msg.MediaType == "" {
		return "", fmt.Errorf("not a media message")
	}
	if msg.URL == nil || *msg.URL == "" {
		return "", fmt.Errorf("no media URL available")
	}
	if len(msg.MediaKey) == 0 || len(msg.FileSHA256) == 0 || len(msg.FileEncSHA256) == 0 {
		return "", fmt.Errorf("incomplete media metadata for download")
	}

	// Determine filename
	filename := "media"
	if msg.Filename != nil && *msg.Filename != "" {
		filename = *msg.Filename
	}
	localPath := filepath.Join(outputDir, filename)

	// Check if already downloaded
	absPath, err := filepath.Abs(localPath)
	if err != nil {
		return "", fmt.Errorf("failed to resolve path: %w", err)
	}
	if _, err := os.Stat(absPath); err == nil {
		return absPath, nil
	}

	// Ensure output directory exists
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create output directory: %w", err)
	}

	// Extract direct path from the stored URL
	directPath := extractDirectPathFromURL(*msg.URL)

	// Determine file length
	var fileLength uint64
	if msg.FileLength != nil {
		fileLength = uint64(*msg.FileLength)
	}

	// Build the appropriate protobuf message type for whatsmeow's Download.
	// The Download method uses proto reflection to determine the MediaType from
	// the concrete type name, so we must use the correct proto message struct.
	downloadable, err := buildDownloadable(*msg.MediaType, *msg.URL, directPath, msg.MediaKey, msg.FileSHA256, msg.FileEncSHA256, fileLength)
	if err != nil {
		return "", err
	}

	data, err := c.WAClient.Download(ctx, downloadable)
	if err != nil {
		return "", fmt.Errorf("failed to download media: %w", err)
	}

	if err := os.WriteFile(absPath, data, 0644); err != nil {
		return "", fmt.Errorf("failed to save media file: %w", err)
	}

	return absPath, nil
}

// buildDownloadable constructs the appropriate waE2E protobuf message for
// whatsmeow's Download method. The Download method uses proto reflection to
// look up the media type from the struct name (e.g. "ImageMessage" -> MediaImage),
// so we must use the real proto types rather than a custom struct.
func buildDownloadable(mediaType, url, directPath string, mediaKey, fileSHA256, fileEncSHA256 []byte, fileLength uint64) (whatsmeow.DownloadableMessage, error) {
	switch mediaType {
	case "image":
		return &waE2E.ImageMessage{
			URL:           proto.String(url),
			DirectPath:    proto.String(directPath),
			MediaKey:      mediaKey,
			FileSHA256:    fileSHA256,
			FileEncSHA256: fileEncSHA256,
			FileLength:    proto.Uint64(fileLength),
		}, nil
	case "video":
		return &waE2E.VideoMessage{
			URL:           proto.String(url),
			DirectPath:    proto.String(directPath),
			MediaKey:      mediaKey,
			FileSHA256:    fileSHA256,
			FileEncSHA256: fileEncSHA256,
			FileLength:    proto.Uint64(fileLength),
		}, nil
	case "audio":
		return &waE2E.AudioMessage{
			URL:           proto.String(url),
			DirectPath:    proto.String(directPath),
			MediaKey:      mediaKey,
			FileSHA256:    fileSHA256,
			FileEncSHA256: fileEncSHA256,
			FileLength:    proto.Uint64(fileLength),
		}, nil
	case "document":
		return &waE2E.DocumentMessage{
			URL:           proto.String(url),
			DirectPath:    proto.String(directPath),
			MediaKey:      mediaKey,
			FileSHA256:    fileSHA256,
			FileEncSHA256: fileEncSHA256,
			FileLength:    proto.Uint64(fileLength),
		}, nil
	case "sticker":
		return &waE2E.StickerMessage{
			URL:           proto.String(url),
			DirectPath:    proto.String(directPath),
			MediaKey:      mediaKey,
			FileSHA256:    fileSHA256,
			FileEncSHA256: fileEncSHA256,
			FileLength:    proto.Uint64(fileLength),
		}, nil
	default:
		return nil, fmt.Errorf("unsupported media type: %s", mediaType)
	}
}

// extractDirectPathFromURL extracts the direct path component from a WhatsApp
// media URL. The direct path is the path after the domain, without query params.
// Example: "https://mmg.whatsapp.net/v/t62.7118-24/file.enc?ccb=..." -> "/v/t62.7118-24/file.enc"
func extractDirectPathFromURL(url string) string {
	parts := strings.SplitN(url, ".net/", 2)
	if len(parts) < 2 {
		return url
	}
	pathPart := strings.SplitN(parts[1], "?", 2)[0]
	return "/" + pathPart
}

// UploadMedia uploads a file to WhatsApp CDN and returns the upload response.
func (c *Client) UploadMedia(ctx context.Context, filePath string, mediaType whatsmeow.MediaType) (*whatsmeow.UploadResponse, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	resp, err := c.WAClient.Upload(ctx, data, mediaType)
	if err != nil {
		return nil, fmt.Errorf("failed to upload media: %w", err)
	}

	return &resp, nil
}

// AnalyzeOggOpus reads an Ogg Opus file and returns its duration in seconds.
// Used for voice messages (PTT) which need duration metadata.
// Parses Ogg page headers to find the last granule position and divides by
// 48000 (Opus sample rate) for duration.
func AnalyzeOggOpus(data []byte) (uint32, error) {
	if len(data) < 4 || string(data[0:4]) != "OggS" {
		return 0, fmt.Errorf("not a valid Ogg file (missing OggS signature)")
	}

	var lastGranule uint64
	var sampleRate uint32 = 48000 // Default Opus sample rate
	var preSkip uint16
	var foundOpusHead bool

	for i := 0; i < len(data); {
		// Need at least 27 bytes for the Ogg page header
		if i+27 > len(data) {
			break
		}

		// Verify Ogg page signature
		if string(data[i:i+4]) != "OggS" {
			i++
			continue
		}

		// Read granule position (bytes 6-13)
		granulePos := binary.LittleEndian.Uint64(data[i+6 : i+14])

		// Read page sequence number (bytes 18-21)
		pageSeqNum := binary.LittleEndian.Uint32(data[i+18 : i+22])

		// Number of segments (byte 26)
		numSegments := int(data[i+26])

		// Need segment table
		if i+27+numSegments > len(data) {
			break
		}
		segmentTable := data[i+27 : i+27+numSegments]

		// Calculate total page size (header + segment table + segment data)
		pageDataSize := 0
		for _, segLen := range segmentTable {
			pageDataSize += int(segLen)
		}
		pageSize := 27 + numSegments + pageDataSize

		// Bounds check for the full page
		if i+pageSize > len(data) {
			break
		}

		// Look for OpusHead packet in the first pages
		if !foundOpusHead && pageSeqNum <= 1 {
			pageData := data[i : i+pageSize]
			headIdx := findBytes(pageData, []byte("OpusHead"))
			if headIdx >= 0 && headIdx+16 <= len(pageData) {
				// OpusHead layout after the 8-byte magic:
				//   Version(1) + Channels(1) + PreSkip(2) + SampleRate(4) + ...
				base := headIdx + 8
				if base+8 <= len(pageData) {
					preSkip = binary.LittleEndian.Uint16(pageData[base+2 : base+4])
					sampleRate = binary.LittleEndian.Uint32(pageData[base+4 : base+8])
					foundOpusHead = true
				}
			}
		}

		// Track the last non-zero granule position
		if granulePos != 0 {
			lastGranule = granulePos
		}

		i += pageSize
	}

	// Calculate duration
	var duration uint32
	if lastGranule > 0 {
		durationSeconds := float64(lastGranule-uint64(preSkip)) / float64(sampleRate)
		duration = uint32(math.Ceil(durationSeconds))
	} else {
		// Fallback: rough estimation from file size
		durationEstimate := float64(len(data)) / 2000.0
		duration = uint32(durationEstimate)
	}

	// Clamp to reasonable range
	if duration < 1 {
		duration = 1
	} else if duration > 300 {
		duration = 300
	}

	return duration, nil
}

// findBytes returns the index of the first occurrence of needle in haystack,
// or -1 if not found.
func findBytes(haystack, needle []byte) int {
	for i := 0; i <= len(haystack)-len(needle); i++ {
		match := true
		for j := range needle {
			if haystack[i+j] != needle[j] {
				match = false
				break
			}
		}
		if match {
			return i
		}
	}
	return -1
}

// PlaceholderWaveform generates a synthetic 64-byte waveform for voice messages.
// Uses deterministic random based on duration for reproducibility.
// Thread-safe: uses rand.New() not global rand.Seed().
func PlaceholderWaveform(duration uint32) []byte {
	const waveformLength = 64
	waveform := make([]byte, waveformLength)

	rng := rand.New(rand.NewSource(int64(duration)))

	baseAmplitude := 35.0
	frequencyFactor := float64(min(int(duration), 120)) / 30.0

	for i := range waveform {
		pos := float64(i) / float64(waveformLength)

		// Multiple sine waves for a natural-looking pattern
		val := baseAmplitude * math.Sin(pos*math.Pi*frequencyFactor*8)
		val += (baseAmplitude / 2) * math.Sin(pos*math.Pi*frequencyFactor*16)

		// Add randomness
		val += (rng.Float64() - 0.5) * 15

		// Fade-in/fade-out envelope
		fadeInOut := math.Sin(pos * math.Pi)
		val = val * (0.7 + 0.3*fadeInOut)

		// Center around a typical voice baseline
		val += 50

		// Clamp to [0, 100]
		if val < 0 {
			val = 0
		} else if val > 100 {
			val = 100
		}

		waveform[i] = byte(val)
	}

	return waveform
}
