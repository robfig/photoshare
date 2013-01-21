package models

import (
	"fmt"
	"github.com/robfig/revel"
)

type Thumbnail struct {
	PhotoId       int32 // The photo of which this thumbnail is a smaller version.
	Width, Height int32
}

func (t Thumbnail) S3Path() string {
	return fmt.Sprintf("%dx%d/%d", t.Width, t.Height, t.PhotoId)
}

func (t Thumbnail) S3Url() string {
	return fmt.Sprintf("%s/%s", S3, t.S3Path())
}

func init() {
	rev.TemplateFuncs["thumbUrl"] = func(photo *Photo, width, height int32) string {
		return Thumbnail{PhotoId: photo.PhotoId, Width: width, Height: height}.S3Url()
	}
}
