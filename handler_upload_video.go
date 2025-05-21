package main

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/google/uuid"
)

func (cfg *apiConfig) handlerUploadVideo(w http.ResponseWriter, r *http.Request) {
	const uploadLimit = 1 << 30

	r.Body = http.MaxBytesReader(w, r.Body, uploadLimit)

	videoIDString := r.PathValue("videoID")

	videoID, err := uuid.Parse(videoIDString)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "VideoID not valid", err)
	}

	token, err := auth.GetBearerToken(r.Header)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't find JWT", err)
		return
	}

	userID, err := auth.ValidateJWT(token, cfg.jwtSecret)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't validate JWT", err)
		return
	}

	video, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Couldn't get video from database", err)
		return
	}

	if video.UserID != userID {
		respondWithError(w, http.StatusUnauthorized, "You can't modify the video since you are not its owner", err)
		return
	}

	file, header, err := r.FormFile("video")
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't get data from formfile", err)
		return
	}

	defer file.Close()

	mimeType, _, err := mime.ParseMediaType(header.Header.Get("Content-Type"))
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't get mime type", err)
		return
	}

	if mimeType != "video/mp4" {
		respondWithError(w, http.StatusInternalServerError, "Mime type not supported for video", err)
		return
	}

	tempFile, err := os.CreateTemp("", "tubely-upload.mp4")
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't create temp file", err)
		return
	}

	defer tempFile.Close()
	defer os.Remove(tempFile.Name())

	_, err = io.Copy(tempFile, file)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error copying content to tempfile", err)
		return
	}

	_, err = tempFile.Seek(0, io.SeekStart)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't get back to beginning of tempfile", err)
		return
	}

	aspectRatio, err := getVideoAspectRatio(tempFile.Name())
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't get back to beginning of tempfile", err)
		return
	}

	processedFilePath, err := processVideoForFastStart(tempFile.Name())
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error moving moov atom", err)
		return
	}

	processedFile, err := os.Open(processedFilePath)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error opening processed file", err)
		return
	}

	defer processedFile.Close()
	defer os.Remove(processedFile.Name())

	key := make([]byte, 32)
	rand.Read(key)
	ext, err := getExtension(mimeType)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't get extension", err)
		return
	}

	var base64key string
	if aspectRatio == "16:9" {
		base64key = filepath.Join("landscape", base64.RawURLEncoding.EncodeToString(key)+ext)
	}
	if aspectRatio == "9:16" {
		base64key = filepath.Join("portrait", base64.RawURLEncoding.EncodeToString(key)+ext)
	}
	if aspectRatio == "other" {
		base64key = filepath.Join("other", base64.RawURLEncoding.EncodeToString(key)+ext)
	}

	_, err = cfg.s3Client.PutObject(r.Context(), &s3.PutObjectInput{Bucket: aws.String(cfg.s3Bucket),
		Key:         aws.String(base64key),
		Body:        processedFile,
		ContentType: aws.String(mimeType)})
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't upload objet to s3", err)
		return
	}

	newURLforVideo := fmt.Sprintf("%s,%s", cfg.s3Bucket, base64key)

	video.VideoURL = &newURLforVideo

	if err = cfg.db.UpdateVideo(video); err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't update video in database", err)
		return
	}

	signedVideo, err := cfg.dbVideoToSignedVideo(video)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't update signed video", err)
		return
	}

	respondWithJSON(w, http.StatusOK, signedVideo)
}
