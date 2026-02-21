package main

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/database"
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

	fmt.Println("uploading file for video", videoID, "by user", userID)
	const maxMemory = 1 << 30 //1GB limit

	r.Body = http.MaxBytesReader(w, r.Body, maxMemory)

	err = r.ParseMultipartForm(maxMemory)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "unable to parse form entry", err)
		return
	}

	dbVideo, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Unable to get video", err)
	}
	if dbVideo.UserID != userID {
		respondWithError(w, http.StatusUnauthorized, "UserID mismatch", err)
		return
	}
	file, header, err := r.FormFile("video")
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

	tempFile, err := os.CreateTemp("", "tubely-upload-*.mp4")
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Unable to save video", err)
		return
	}

	_, err = io.Copy(tempFile, file)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Unable to save video", err)
		return
	}
	tempFile.Sync()
	_, err = tempFile.Seek(0, io.SeekStart)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Unable to read new file", err)
		return
	}
	randomKey, err := randomHex(32)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Unable to generate random file key for upload:", err)
		return
	}
	defer tempFile.Close()

	aspectRatio, err := getVideoAspectRatio(tempFile.Name())
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Unable to calculate aspect ratio:", err)
		return
	}

	var fileName string
	switch aspectRatio { //Split file into different containers in s3 depending on aspect ratio
	case "other":
		fileName = "/other/" + randomKey + ".mp4"
	case "16:9":
		fileName = "/landscape/" + randomKey + ".mp4"
	case "9:16":
		fileName = "/portrait/" + randomKey + ".mp4"
	default:
		fileName = "/error/" + randomKey + ".mp4"
	}

	s3Options := &s3.PutObjectInput{
		Body:        tempFile,
		Bucket:      &cfg.s3Bucket,
		Key:         &fileName,
		ContentType: &fileType,
	}
	_, err = cfg.s3Client.PutObject(r.Context(), s3Options)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Unable to save to s3:", err)
		return
	}
	defer os.Remove(tempFile.Name())

	videoURL := "https://" + cfg.s3Bucket + ".s3." + cfg.s3Region + ".amazonaws.com/" + fileName

	fmt.Println("Uploaded video to URL: ", videoURL)
	cfg.db.UpdateVideo(database.Video{
		ID:                dbVideo.ID,
		CreatedAt:         dbVideo.CreatedAt,
		UpdatedAt:         time.Now().UTC(),
		ThumbnailURL:      dbVideo.ThumbnailURL,
		VideoURL:          &videoURL,
		CreateVideoParams: dbVideo.CreateVideoParams,
	})
	respondWithJSON(w, http.StatusOK, "")
}

func randomHex(n int) (string, error) {
	bytes := make([]byte, n)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}
