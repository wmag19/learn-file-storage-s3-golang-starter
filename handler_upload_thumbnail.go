package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"

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

	const maxMemory = 10 << 20 //10MB limit
	err = r.ParseMultipartForm(maxMemory)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "unable to parse form entry", err)
		return
	}

	file, header, err := r.FormFile("thumbnail")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Unable to parse form file", err)
		return
	}
	defer file.Close()

	fileType := header.Header.Get("Content-Type")
	fileExtensionSlice := strings.Split(fileType, "/")
	fileExtension := fileExtensionSlice[1]
	isValidFileType := validFileType(fileExtension, cfg.allowedFileTypes)
	if isValidFileType != true {
		respondWithError(w, http.StatusNotAcceptable, "Invalid file extension", err)
	}

	video, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Unable to get video", err)
	}
	if video.UserID != userID {
		respondWithError(w, http.StatusUnauthorized, "UserID mismatch", err)
		return
	}

	filePath := filepath.Join("./", cfg.assetsRoot, videoIDString+"."+fileExtension)

	osFile, err := os.Create(filePath)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Unable to save video file", err)
		return
	}
	defer osFile.Close()

	_, err = io.Copy(osFile, file)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Unable to save video file", err)
		return
	}

	dataURL := "http://localhost:8091/assets/" + videoIDString + "." + fileExtension
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
		return
	}

	returnVideo, err := cfg.db.GetVideo(video.ID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Unable to get updated video", err)
		return
	}

	respondWithJSON(w, http.StatusOK, returnVideo)
}

func validFileType(fileType string, allowedList []string) bool {
	fmt.Println(allowedList)
	fmt.Println(fileType)
	if slices.Contains(allowedList, fileType) {
		return true
	}
	return false
}
