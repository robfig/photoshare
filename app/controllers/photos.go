package controllers

import (
	"github.com/robfig/photoshare/app/models"
	"github.com/robfig/revel"
)

type Photos struct {
	GorpController
}

func (c Photos) View(id int) rev.Result {
	photoResult, err := c.Txn.Get(models.Photo{}, id)
	if err != nil {
		return c.RenderError(err)
	}

	photo := photoResult.(*models.Photo)
	event, _ := c.Txn.Get(models.Event{}, photo.EventId)

	// Get the following photo
	photos, err := c.Txn.Select(models.Photo{}, `
select * from Photo
 where EventId = ? and (TakenStr > ? or Username > ?)
 order by Username, TakenStr
 limit 1`,
		photo.EventId, photo.TakenStr, photo.Username)
	if err != nil {
		return c.RenderError(err)
	}

	var next *models.Photo
	if len(photos) != 0 {
		next = photos[0].(*models.Photo)
	}

	// Get the previous photo
	photos, err = c.Txn.Select(models.Photo{}, `
select * from Photo
 where EventId = ? and (TakenStr < ? or Username < ?)
 order by Username desc, TakenStr desc
 limit 1`,
		photo.EventId, photo.TakenStr, photo.Username)
	if err != nil {
		return c.RenderError(err)
	}

	var prev *models.Photo
	if len(photos) != 0 {
		prev = photos[0].(*models.Photo)
	}

	return c.Render(photo, next, prev, event)
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
	_ = PHOTO_BUCKET.Del(photo.S3Path())

	thumbResults, err := c.Txn.Select(models.Thumbnail{},
		"select * from Thumbnail where PhotoId = ?",
		id)
	if err != nil {
		return c.RenderError(err)
	}

	for _, thumb := range thumbResults {
		_ = PHOTO_BUCKET.Del(thumb.(*models.Thumbnail).S3Path())
	}

	c.Txn.Delete(photo)
	c.Flash.Success("Photo deleted")

	return c.Redirect("/events/%d/view", photo.EventId)
}
