package main

import (
	"encoding/base64"
	"fmt"
	"io"
	"net/http"

	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/google/uuid"
)

func (cfg *apiConfig) handlerUploadThumbnail(w http.ResponseWriter, r *http.Request) {
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

	fmt.Println("uploading thumbnail for video", videoID, "by user", userID)

	const maxMemory = 10 << 20

	r.ParseMultipartForm(maxMemory)

	file, headers, err := r.FormFile("thumbnail")
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error getting file and headers", err)
		return
	}

	defer file.Close()

	fileMediaType := headers.Header.Get("Content-Type")

	fileData, err := io.ReadAll(file)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error reading file", err)
		return
	}
	video, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error getting video from database", err)
		return
	}
	if userID != video.UserID {
		respondWithError(w, http.StatusUnauthorized, "Can't change this video since you are not its creator", err)
		return
	}

	base64String := base64.StdEncoding.EncodeToString(fileData)

	dataURL := fmt.Sprintf("data:%v;base64,%v", fileMediaType, base64String)
	video.ThumbnailURL = &dataURL

	if err = cfg.db.UpdateVideo(video); err != nil {
		respondWithError(w, http.StatusUnauthorized, "Error updating video in database", err)
		return
	}

	respondWithJSON(w, http.StatusOK, video)
}
