package main

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
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

	const maxMemory = 10 << 20

	r.ParseMultipartForm(maxMemory)

	file, header, err := r.FormFile("thumbnail")
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error getting file and headers", err)
		return
	}

	defer file.Close()

	fileMediaType, _, err := mime.ParseMediaType(header.Header.Get("Content-Type"))
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error getting media type", err)
		return
	}

	if fileMediaType != "image/jpeg" && fileMediaType != "image/png" {
		respondWithError(w, http.StatusInternalServerError, "Wrong media type", err)
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

	ext, err := getExtension(fileMediaType)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error getting extension", err)
		return
	}

	key := make([]byte, 32) //tienes que hacer esto primero porque, si no, solo estás creando el url.
	rand.Read(key)          //Recordar que el nombre del archivo es el pathfile hacia ese archivo
	urlKey := base64.RawURLEncoding.EncodeToString(key)

	newPath := NewPath(urlKey, cfg.assetsRoot)

	tempFile, err := os.Create(newPath + ext)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error creating temp file", err)
		return
	}

	defer tempFile.Close()

	if _, err = io.Copy(tempFile, file); err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error copying content to temp. file", err)
		return
	}
	//si haces rand.Read aquí y solo agregas el URL al video, no estás creando el path al video, solo el URL.
	//El path hacia el file seguirá siendo otro y la imagen/video no se verá en la página.
	tURL := fmt.Sprintf("http://localhost:%v/assets/%s%s", cfg.port, urlKey, ext)

	video.ThumbnailURL = &tURL

	if err = cfg.db.UpdateVideo(video); err != nil {
		respondWithError(w, http.StatusUnauthorized, "Error updating video in database", err)
		return
	}

	respondWithJSON(w, http.StatusOK, video)
}

func getExtension(ct string) (string, error) {
	fields := strings.SplitAfter(ct, "/")
	if len(fields) != 2 {
		return "", errors.New("malformed Content-Type")
	}
	return "." + fields[1], nil
}

func NewPath(videoID, assetsPath string) string {
	return filepath.Join(assetsPath, videoID)
}
