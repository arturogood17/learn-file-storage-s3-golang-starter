package main

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
)

func processVideoForFastStart(filePath string) (string, error) {
	outputFile := filePath + ".processing"
	cmd := exec.Command("ffmpeg", "-i", filePath, "-c", "copy", "-movflags", "faststart", "-f", "mp4", outputFile)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", err
	}

	fileInfo, err := os.Stat(outputFile)
	if err != nil {
		return "", errors.New("error getting info of the file")
	}

	if fileInfo.Size() == 0 {
		return "", errors.New("size of file is 0")
	}
	return outputFile, nil
}
