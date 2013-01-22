package controllers

import (
	"bytes"
	"github.com/disintegration/imaging"
	"github.com/robfig/goamz/s3"
	"github.com/robfig/photoshare/app/models"
	"github.com/robfig/revel"
	"github.com/rwcarlsen/goexif/exif"
	"image"
	"image/jpeg"
)

var REORIENTATION_FUNCS = map[int]func(image.Image) *image.NRGBA{
	1: nil,
	3: imaging.Rotate180,
	6: imaging.Rotate270,
	8: imaging.Rotate90,
}

// Generate a thumbnail from the given image, save it to S3, and save the record to the DB.
func SaveThumbnail(
	proc func(image.Image, int, int, imaging.ResampleFilter) *image.NRGBA,
	photoId int32,
	photoImage image.Image,
	ex *exif.Exif,
	width, height int32) {

	thumbnail := proc(photoImage, int(width), int(height), imaging.Lanczos)

	// If the EXIF says to, rotate the thumbnail.
	var orientation int = 1
	if ex != nil {
		if orientationTag, err := ex.Get(exif.Orientation); err == nil {
			orientation = int(orientationTag.Int(0))
		}
		if rotateFunc, ok := REORIENTATION_FUNCS[orientation]; ok && rotateFunc != nil {
			thumbnail = rotateFunc(thumbnail)
		}
	}

	var thumbnailBuffer bytes.Buffer
	err := jpeg.Encode(&thumbnailBuffer, thumbnail, nil)
	if err != nil {
		rev.ERROR.Println("Failed to create thumbnail:", err)
		return
	}

	thumbnailModel := &models.Thumbnail{
		PhotoId: photoId,
		Width:   width,
		Height:  height,
	}

	err = PHOTO_BUCKET.PutReader(thumbnailModel.S3Path(),
		&thumbnailBuffer,
		int64(thumbnailBuffer.Len()),
		"image/jpeg",
		s3.PublicRead)
	if err != nil {
		rev.ERROR.Println("Failed to create thumbnail:", err)
		return
	}

	dbm.Insert(thumbnailModel)
}

// To avoid getting swamped with thumbnail goroutines, limit the processing to a
// single routine.

type ThumbnailRequest struct {
	PhotoId    int32
	PhotoImage image.Image
	Exif       *exif.Exif
}

var THUMBNAIL_GENERATOR chan ThumbnailRequest

func init() {
	THUMBNAIL_GENERATOR = make(chan ThumbnailRequest, 1000)
	go RunThumbnailGenerator()
}

func RunThumbnailGenerator() {
	for {
		rev.INFO.Println("Thumbnailer: Waiting...")
		req := <-THUMBNAIL_GENERATOR
		rev.INFO.Println("Thumbnailer: Thumbnailing photo", req.PhotoId)
		SaveThumbnail(imaging.Thumbnail, req.PhotoId, req.PhotoImage, req.Exif, 250, 250)
		SaveThumbnail(imaging.Fit, req.PhotoId, req.PhotoImage, req.Exif, 740, 555)
	}
}
