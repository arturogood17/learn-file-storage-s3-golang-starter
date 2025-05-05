package main

import (
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"

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

	const maxMemory = 10 << 20 //10 mb

	r.ParseMultipartForm(maxMemory)

	file, header, err := r.FormFile("thumbnail")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Couldn't parse form file", err)
		return
	}
	mimetype, _, err := mime.ParseMediaType(header.Header.Get("Content-Type"))
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid content type", err)
		return
	}
	if (mimetype != "image/jpeg") && (mimetype != "image/png") {
		respondWithError(w, http.StatusBadRequest, "Type not supported for this operation", nil)
		return
	}

	defer file.Close()
	metadata, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't get video in database", err)
		return
	}
	if metadata.UserID != userID {
		respondWithError(w, http.StatusUnauthorized, "Can't change the video as you are not its owner", nil)
		return
	}
	if err = cfg.ensureAssetsDir(); err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error creating assets dir", err)
		return
	}

	fileExt := "." + strings.SplitAfter(mimetype, "/")[1]
	filepath := filepath.Join(cfg.assetsRoot, videoID.String()+fileExt)
	newFile, err := os.Create(filepath)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error creating file", err)
		return
	}
	if _, err := io.Copy(newFile, file); err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error copying filedata to file in system", err)
		return
	}
	defer newFile.Close()
	URLData := fmt.Sprintf("http://localhost:%s/assets/%s%s", cfg.port, videoID.String(), fileExt)
	metadata.ThumbnailURL = &URLData
	if err := cfg.db.UpdateVideo(metadata); err != nil {
		respondWithError(w, http.StatusInternalServerError, "error updating thumbnail in database", err)
		return
	}
	fmt.Println(fileExt)
	respondWithJSON(w, http.StatusOK, metadata)
}
