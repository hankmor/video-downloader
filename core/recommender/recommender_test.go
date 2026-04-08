package recommender

import (
	"testing"

	"github.com/hankmor/vdd/core/parser"
)

func TestRecommend(t *testing.T) {
	r := New()

	input := []parser.Format{
		{FormatID: "137", Resolution: "1920x1080", Height: 1080, HasVideo: true, HasAudio: false, Extension: "mp4", VCodec: "avc1"}, // 1080p Video Only
		{FormatID: "22", Resolution: "1280x720", Height: 720, HasVideo: true, HasAudio: true, Extension: "mp4", VCodec: "avc1"},     // 720p Combined
		{FormatID: "140", Resolution: "audio only", Height: 0, HasVideo: false, HasAudio: true, Extension: "m4a"},                   // Audio Only
		{FormatID: "313", Resolution: "3840x2160", Height: 2160, HasVideo: true, HasAudio: false, Extension: "webm", VCodec: "vp9"}, // 4K Video Only
	}

	result := r.Recommend(input, 0)

	if len(result) == 0 {
		t.Fatal("Expected results, got empty")
	}

	// 1. Should include video-only formats (logic change requirement)
	foundVideoOnly := false
	for _, f := range result {
		if f.HasVideo && !f.HasAudio {
			foundVideoOnly = true
			break
		}
	}
	if !foundVideoOnly {
		t.Error("Result should include video-only formats")
	}

	// 2. 4K Video Only (313) should be recommended (Rank 1) because it has highest quality,
	// assuming we implemented the scoring fix.
	// Current legacy logic might pick 22. New logic should pick 313.
	best := result[0]
	if best.FormatID != "313" {
		t.Errorf("Best format is %s (Height: %d), expected 313 (4K)", best.FormatID, best.Height)
	}
}
