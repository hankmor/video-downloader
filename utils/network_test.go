package utils

import (
	"fmt"
	"testing"
)

func TestDownloadImage(t *testing.T) {
	url := "https://pbs.twimg.com/amplify_video_thumb/2011167511970971648/img/JgBU06wYdr71paTr.jpg?name=orig"
	data, err := DownloadImage(url, "")
	if err != nil {
		t.Error(err)
	}
	fmt.Println(data)
}
