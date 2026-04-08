package parser

import (
	"log"
	"testing"

	"github.com/hankmor/vdd/core/test"
)

func TestParse(t *testing.T) {
	url := "https://www.youtube.com/watch?v=yDc0_8emz7M"
	parser := New(test.YtdlpPath)
	info, err := parser.ParseVideo(url)
	if err != nil {
		t.Errorf("Error parsing video: %v", err)
	}
	log.Printf("Video info: %+v", info)
}
