package main

import (
	"encoding/base64"
	"fmt"
	"io"
	"net/http"

	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/database"
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

	// TODO: implement the upload here
	const maxMemory = 10 << 20 //10MB limit
	err = r.ParseMultipartForm(maxMemory)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "unable to parse form entry", err)
		return
	}

	file, header, err := r.FormFile("thumbnail")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Unable to parse form file", err)
	}

	defer file.Close()

	fileType := header.Header.Get("Content-Type")
	var fileRead []byte
	fileRead, err = io.ReadAll(file)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Unable to read file", err)
	}
	video, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Unable to get video", err)
	}
	if video.UserID != userID {
		respondWithError(w, http.StatusUnauthorized, "UserID mismatch", err)
	}

	encodedThumbnail := base64.StdEncoding.EncodeToString(fileRead)

	dataURL := "data:" + fileType + ";base64," + encodedThumbnail
	newVideo := database.Video{
		ID:                video.ID,
		CreatedAt:         video.CreatedAt,
		UpdatedAt:         video.UpdatedAt,
		ThumbnailURL:      &dataURL,
		VideoURL:          video.VideoURL,
		CreateVideoParams: video.CreateVideoParams,
	}
	err = cfg.db.UpdateVideo(newVideo)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Unable to update video", err)
	}
	returnVideo, err := cfg.db.GetVideo(video.ID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Unable to get updated video", err)
	}
	respondWithJSON(w, http.StatusOK, returnVideo)
}
