package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"os/exec"
)

func getVideoAspectRatio(filepath string) (string, error) {
	var p struct {
		Streams []struct {
			Width  int `json:"width,omitempty"`
			Height int `json:"height,omitempty"`
		} `json:"streams"`
	}

	cmd := exec.Command("ffprobe", "-v", "error", "-print_format", "json", "-show_streams", filepath)
	var b bytes.Buffer
	cmd.Stdout = &b
	if err := cmd.Run(); err != nil {
		return "", err
	}

	if err := json.Unmarshal(b.Bytes(), &p); err != nil {
		return "", err
	}

	if len(p.Streams) == 0 {
		return "", errors.New("no video streams found")
	}

	if p.Streams[0].Height == 0 {
		return "", errors.New("height is and zero division is not permitted")
	}

	aspectRatio := float64(p.Streams[0].Width) / float64(p.Streams[0].Height)

	if aspectRatio > 1.6 && aspectRatio < 1.8 {
		return "16:9", nil
	}
	if aspectRatio < 0.6 {
		return "9:16", nil
	}
	return "other", nil
}
