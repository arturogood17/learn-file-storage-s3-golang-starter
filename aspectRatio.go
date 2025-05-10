package main

import (
	"bytes"
	"encoding/json"
	"os/exec"
)

func getVideoAspectRatio(filePath string) (string, error) {
	cmd := exec.Command("ffprobe", "-v", "error", "-print_format", "json", "-show_streams", filePath)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Run()
	var params FFprobe
	if err := json.Unmarshal(out.Bytes(), &params); err != nil {
		return "", err
	}
	var aspectRatio = float64(params.Streams[0].Width) / float64(params.Streams[0].Height)
	if aspectRatio > 1.7 && aspectRatio < 1.85 {
		return "16:9", nil
	} else if aspectRatio > 0.0 && aspectRatio < 0.60 {
		return "9:16", nil
	} else {
		return "other", nil
	}
}
