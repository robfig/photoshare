package controllers

import (
	"fmt"
	"github.com/robfig/photoshare/app/models"
	"github.com/robfig/revel"
	"path"
)

type Photos struct {
	GorpController
}

func (c Photos) Delete(id int) rev.Result {
	photoResult, err := c.Txn.Get(models.Photo{}, id)
	if err != nil {
		return c.RenderError(err)
	}

	if photoResult == nil {
		return c.NotFound("No photo found.")
	}

	photo := photoResult.(*models.Photo)

	// TODO: Need a better way to manage S3 paths.
	eventIdStr := fmt.Sprintf("%d", photo.EventId)
	photoPath := path.Join(eventIdStr, "original", photo.Username, photo.Name)
	thumbPath := path.Join(eventIdStr, "250x250", photo.Username, photo.Name)
	_ = PHOTO_BUCKET.Del(photoPath)
	_ = PHOTO_BUCKET.Del(thumbPath)
	c.Txn.Delete(photo)
	c.Flash.Success("Photo deleted")

	return c.Redirect("/events/%d/view", photo.EventId)
}
