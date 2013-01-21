package models

import (
	"fmt"
	"github.com/coopernurse/gorp"
	"math/rand"
	"time"
)

const S3 = "http://s3.amazonaws.com/photoboard"

type Photo struct {
	PhotoId  int32  // Random integer key.
	EventId  int32  // Which event is it attached to?
	Filename string // The base filename on the uploader's computer.
	Format   string // Image format returned by image.Decode
	Username string // Name of the user that uploaded this.

	Width, Height int32 // Pixels

	Taken    time.Time // Initially set from EXIF data, but may be subsequently updated.
	Uploaded time.Time // Time of the upload (on record creation).

	TakenStr, UploadedStr string // Temporary fields used to store time.Time as a string.
}

const (
	DATE_FORMAT         = "Jan _2, 2006"
	SQL_DATETIME_FORMAT = "2006-01-02 15:04:05"
)

func (p *Photo) PreInsert(_ gorp.SqlExecutor) error {
	p.PhotoId = rand.Int31()
	p.TakenStr = p.Uploaded.Format(SQL_DATETIME_FORMAT)
	p.UploadedStr = p.Taken.Format(SQL_DATETIME_FORMAT)
	return nil
}

func (p *Photo) PostGet(_ gorp.SqlExecutor) error {
	var err error
	if p.Taken, err = time.Parse(SQL_DATETIME_FORMAT, p.TakenStr); err != nil {
		return fmt.Errorf("Error parsing taken date '%s':", p.TakenStr, err)
	}
	if p.Uploaded, err = time.Parse(SQL_DATETIME_FORMAT, p.UploadedStr); err != nil {
		return fmt.Errorf("Error parsing uploaded date '%s':", p.UploadedStr, err)
	}
	return nil
}

func (p Photo) S3Path() string {
	return fmt.Sprintf("%d", p.PhotoId)
}

func (p Photo) S3Url() string {
	return fmt.Sprintf("%s/%s", S3, p.S3Path())
}

func (p Photo) ViewUrl() string {
	return fmt.Sprintf("/photos/%d", p.PhotoId)
}
