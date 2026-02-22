package main

import (
	"bytes"
	"encoding/json"
	"os/exec"
)

type probeResult struct {
	Streams []struct {
		CodecType          string `json:"codec_type"`
		Width              int    `json:"width"`
		Height             int    `json:"height"`
		DisplayAspectRatio string `json:"display_aspect_ratio"`
	} `json:"streams"`
}

func getVideoAspectRatio(filepath string) (string, error) {
	cmd := exec.Command("ffprobe", "-v", "error", "-print_format", "json", "-show_streams", filepath)
	var outBuf bytes.Buffer
	cmd.Stdout = &outBuf
	if err := cmd.Run(); err != nil {
		return "", err
	}

	var res probeResult
	if err := json.Unmarshal(outBuf.Bytes(), &res); err != nil {
		return "", err
	}

	var aspectRatio string

	for i := range res.Streams {
		if res.Streams[i].CodecType == "video" {
			aspectRatio = res.Streams[i].DisplayAspectRatio
			break
		}
	}
	if aspectRatio != "16:9" && aspectRatio != "9:16" {
		aspectRatio = "other"
	}

	return aspectRatio, nil
}

func processVideoForFastStart(filePath string) (string, error) {
	outputFilePath := filePath + ".processing"
	cmd := exec.Command("ffmpeg", "-i", filePath, "-c", "copy", "-movflags", "faststart", "-f", "mp4", outputFilePath)
	if err := cmd.Run(); err != nil {
		return "", err
	}
	return outputFilePath, nil
}
