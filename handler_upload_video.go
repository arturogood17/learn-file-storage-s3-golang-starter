package main

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"strings"

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
		respondWithError(w, http.StatusBadRequest, "Invalid ID", err)
		return
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
		respondWithError(w, http.StatusInternalServerError, "Error getting video from database", err)
		return
	}

	if video.UserID != userID {
		respondWithError(w, http.StatusUnauthorized, "Error getting video from database", err)
		return
	}

	// const maxMemory = 10 << 20 No hizo esto. No sé por qué.
	// r.ParseMultipartForm(maxMemory)

	file, header, err := r.FormFile("video")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Error parsing file and header from formfile", err)
		return
	}
	defer file.Close()

	videoMimeType, _, err := mime.ParseMediaType(header.Header.Get("Content-Type"))
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Type not supported", err)
		return
	}
	if videoMimeType != "video/mp4" {
		respondWithError(w, http.StatusBadRequest, "Content-Type not supported", err)
		return
	}

	fExt := "." + strings.SplitAfter(videoMimeType, "/")[1]

	uploadedF, err := os.CreateTemp("", "tubely-upload.mp4")
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error creating temp file", err)
		return
	}

	defer os.Remove(uploadedF.Name())
	defer uploadedF.Close()

	_, err = io.Copy(uploadedF, file)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error copying file to temp file", err)
		return
	}

	_, err = uploadedF.Seek(0, io.SeekStart)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't reset file pointer", err)
		return
	}

	aspectRatio, err := getVideoAspectRatio(uploadedF.Name())
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error getting aspect ratio", err)
		return
	}

	key := make([]byte, 32)
	_, err = rand.Read(key)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error creating base id", err)
		return
	}
	var baseToString string
	if aspectRatio == "16:9" {
		baseToString = "landscape/" + base64.RawURLEncoding.EncodeToString(key) + fExt
	}
	if aspectRatio == "9:16" {
		baseToString = "portrait/" + base64.RawURLEncoding.EncodeToString(key) + fExt
	}
	if aspectRatio == "other" {
		baseToString = "other/" + base64.RawURLEncoding.EncodeToString(key) + fExt
	}

	fastStartFile, err := processVideoForFastStart(uploadedF.Name())
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error encoding video for fast start", err)
		return
	}

	_, err = cfg.s3Client.PutObject(r.Context(), &s3.PutObjectInput{
		Bucket:      aws.String(cfg.s3Bucket),
		Key:         aws.String(baseToString),
		Body:        uploadedF,
		ContentType: &videoMimeType})
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error uploading file to s3", err)
		return
	}

	videoURL := fmt.Sprintf("https://%s.s3.%s.amazonaws.com/%s", cfg.s3Bucket, cfg.s3Region, baseToString)
	video.VideoURL = &videoURL
	if err = cfg.db.UpdateVideo(video); err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error updating video on database side", err)
		return
	}
	respondWithJSON(w, http.StatusOK, video)
}
