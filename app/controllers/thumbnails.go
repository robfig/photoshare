package controllers

import (
	"bytes"
	"code.google.com/p/graphics-go/graphics"
	"github.com/robfig/goamz/s3"
	"github.com/robfig/photoshare/app/models"
	"github.com/robfig/revel"
	"github.com/rwcarlsen/goexif/exif"
	"image"
	"image/jpeg"
)

// Generate a thumbnail from the given image, save it to S3, and save the record to the DB.
func SaveThumbnail(photoId int32, photoImage image.Image, ex *exif.Exif, width, height int32) {
	thumbnail := image.NewRGBA(image.Rect(0, 0, int(width), int(height)))
	err := graphics.Thumbnail(thumbnail, photoImage)
	if err != nil {
		rev.ERROR.Println("Failed to create thumbnail:", err)
		return
	}

	// If the EXIF says to, rotate the thumbnail.
	var orientation int = 1
	if orientationTag, err := ex.Get(exif.Orientation); err == nil {
		orientation = int(orientationTag.Int(0))
	}

	if orientation != 1 {
		if angleRadians, ok := ORIENTATION_ANGLES[orientation]; ok {
			rotatedThumbnail := image.NewRGBA(image.Rect(0, 0, 250, 250))
			err = graphics.Rotate(rotatedThumbnail, thumbnail, &graphics.RotateOptions{Angle: angleRadians})
			if err != nil {
				rev.ERROR.Println("Failed to rotate:", err)
			} else {
				thumbnail = rotatedThumbnail
			}
		}
	}

	var thumbnailBuffer bytes.Buffer
	err = jpeg.Encode(&thumbnailBuffer, thumbnail, nil)
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
}
