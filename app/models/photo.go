package models

import (
	"fmt"
	"github.com/coopernurse/gorp"
	"time"
)

type Photo struct {
	PhotoId  int    // Auto-incrementing key.
	Name     string // From the filename
	Format   string // Returned by image.Decode
	Username string // Name of the user that uploaded this.

	Width, Height int // Pixels

	Taken    time.Time // Initially set from EXIF data, but may be subsequently updated.
	Uploaded time.Time // Time of the upload (on record creation).

	PhotoUrl string // Amazon S3 URLs to the photo and thumbnails.
	ThumbUrl string

	TakenStr, UploadedStr string // Temporary fields used to store time.Time as a string.
}

const (
	DATE_FORMAT         = "Jan _2, 2006"
	SQL_DATETIME_FORMAT = "2006-01-02 15:04:05"
)

func (p *Photo) PreInsert(_ gorp.SqlExecutor) error {
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
