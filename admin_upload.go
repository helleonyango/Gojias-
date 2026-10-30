package main

import (
	"bytes"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
)

// saveUploadedImages validates and stores every file in the given multipart
// field, returning the public URLs. The second return value is a friendly error
// message ("" when everything was accepted).
func (a *App) saveUploadedImages(r *http.Request, field, prefix string, max int) ([]string, string) {
	if r.MultipartForm == nil {
		return nil, ""
	}
	files := r.MultipartForm.File[field]
	if len(files) == 0 {
		return nil, ""
	}
	if len(files) > max {
		return nil, "Upload at most " + strconv.Itoa(max) + " photos at a time."
	}

	var saved []string
	for _, fh := range files {
		if fh.Size == 0 {
			continue
		}
		if fh.Size > maxUpload {
			a.removeUploads(saved)
			return nil, "Each photo must be 5 MB or smaller."
		}
		path, msg := a.storeImage(fh, prefix)
		if msg != "" {
			a.removeUploads(saved)
			return nil, msg
		}
		saved = append(saved, path)
	}
	return saved, ""
}

// storeImage writes one validated image and returns its public URL.
func (a *App) storeImage(fh *multipart.FileHeader, prefix string) (string, string) {
	src, err := fh.Open()
	if err != nil {
		return "", "We could not read that photo. Please try again."
	}
	defer src.Close()

	head := make([]byte, 512)
	n, _ := io.ReadFull(src, head)
	ct := http.DetectContentType(head[:n])
	exts := map[string]string{"image/jpeg": ".jpg", "image/png": ".png", "image/webp": ".webp"}
	ext, ok := exts[ct]
	if !ok {
		return "", "Photos must be JPG, PNG or WebP."
	}

	name, err := randomName()
	if err != nil {
		return "", "We could not save that photo. Please try again."
	}
	if err := os.MkdirAll(a.uploadDir, 0o755); err != nil {
		log.Printf("create upload dir: %v", err)
		return "", "We could not save that photo. Please try again."
	}
	path := filepath.Join(a.uploadDir, prefix+"-"+name+ext)
	dst, err := os.Create(path)
	if err != nil {
		log.Printf("create upload: %v", err)
		return "", "We could not save that photo. Please try again."
	}
	defer dst.Close()

	body := io.LimitReader(io.MultiReader(bytes.NewReader(head[:n]), src), maxUpload)
	if _, err := io.Copy(dst, body); err != nil {
		log.Printf("write upload: %v", err)
		_ = os.Remove(path)
		return "", "We could not save that photo. Please try again."
	}
	return "/uploads/" + filepath.Base(path), ""
}

// removeUpload deletes a saved upload by its public URL.
func (a *App) removeUpload(url string) {
	if url == "" {
		return
	}
	name := filepath.Base(url)
	if name == "." || name == "/" || name == "" {
		return
	}
	if err := os.Remove(filepath.Join(a.uploadDir, name)); err != nil && !os.IsNotExist(err) {
		log.Printf("remove upload %s: %v", name, err)
	}
}

func (a *App) removeUploads(urls []string) {
	for _, u := range urls {
		a.removeUpload(u)
	}
}
