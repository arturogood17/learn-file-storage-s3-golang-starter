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

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/google/uuid"
)

func (cfg *apiConfig) handlerUploadVideo(w http.ResponseWriter, r *http.Request) {
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
	const uploadLimit = 1 << 30

	r.Body = http.MaxBytesReader(w, r.Body, uploadLimit)

	video, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error getting video from database", err)
		return
	}

	if video.UserID != userID {
		respondWithError(w, http.StatusUnauthorized, "Error getting video from database", err)
		return
	}

	const maxMemory = 10 << 20

	r.ParseMultipartForm(maxMemory)

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
	uploadedF, err := os.CreateTemp("", "tubely-upload"+fExt)
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
	uploadedF.Seek(0, io.SeekStart)

	base := make([]byte, 32)
	_, err = rand.Read(base)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error creating base id", err)
		return
	}

	baseToString := base64.RawURLEncoding.EncodeToString(base) + fExt
	_, err = cfg.s3Client.PutObject(r.Context(), &s3.PutObjectInput{Bucket: &cfg.s3Bucket,
		Key: &baseToString, Body: uploadedF,
		ContentType: &videoMimeType})
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error put object output", err)
		return
	}
	videoURL := fmt.Sprintf("https://%s.s3.%s.amazonaws.com/%s", cfg.s3Bucket, cfg.s3Region, baseToString)
	video.VideoURL = &videoURL
	if err = cfg.db.UpdateVideo(video); err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error updating video on database side", err)
		return
	}
}
