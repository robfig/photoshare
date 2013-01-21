package controllers

import (
	"archive/zip"
	"bytes"
	"fmt"
	"github.com/robfig/goamz/aws"
	"github.com/robfig/goamz/s3"
	"github.com/robfig/photoshare/app/models"
	"github.com/robfig/revel"
	"github.com/robfig/revel/modules/db/app"
	"github.com/rwcarlsen/goexif/exif"
	"html/template"
	"image"
	_ "image/jpeg"
	"io"
	"io/ioutil"
	"math"
	"net/http"
	"path"
	"reflect"
	"time"
)

var PHOTO_BUCKET *s3.Bucket

// Initialize the AWS connection configuration.
func init() {
	auth, err := aws.EnvAuth()
	if err != nil {
		rev.ERROR.Fatalln(`AWS Authorization Required.
Please set AWS_ACCESS_KEY_ID and AWS_SECRET_ACCESS_KEY environment variables.`)
	}
	PHOTO_BUCKET = s3.New(auth, aws.USEast).Bucket("photoboard")
}

type Events struct {
	GorpController
	Event *models.Event
}

const (
	VIEW     = "Events/View.html"
	DOWNLOAD = "Events/Download.html"
)

type Grouping string

const (
	BY_USER Grouping = "Username"
	BY_DATE          = "TakenStr"
)

func (c *Events) GetEvent() rev.Result {
	val := c.Params.Bind("event", reflect.TypeOf(0))
	if !val.IsValid() {
		return c.NotFound("Event %s not found", c.Params.Get("event"))
	}

	eventId := val.Interface().(int)
	event, err := c.Txn.Get(models.Event{}, eventId)
	if event == nil {
		if err != nil {
			rev.ERROR.Println("Failed to get event:", err)
		}
		return c.NotFound("Event %d not found", eventId)
	}

	c.Event = event.(*models.Event)
	c.RenderArgs["event"] = event
	return nil
}

func (c Events) View(page int) rev.Result {
	return c.gallery(VIEW, page)
}

func (c Events) Download(page int) rev.Result {
	return c.gallery(DOWNLOAD, page)
}

const PHOTOS_PER_PAGE = 100

func (c Events) gallery(template string, page int) rev.Result {
	// Collect the photo gallery.
	if page == 0 {
		page = 1
	}
	start := (page - 1) * PHOTOS_PER_PAGE
	end := start + PHOTOS_PER_PAGE
	gallery, err := c.getGallery(start, end)
	if err != nil {
		return c.RenderError(err)
	}
	c.RenderArgs["gallery"] = gallery

	// Prepare the pagination control.
	url := c.Request.URL
	if gallery.Total < end {
		end = gallery.Total
	}
	c.RenderArgs["pagination"] = Pagination{
		CurrentPage: page,
		NumPages:    gallery.Total/PHOTOS_PER_PAGE + 1,
		BaseUrl:     fmt.Sprintf("http://%s/%s", url.Host, url.Path),
		Start:       start + 1,
		End:         end,
		Total:       gallery.Total,
	}
	return c.RenderTemplate(template)
}

type Gallery struct {
	Photos map[string][]*models.Photo
	Total  int
}

func (c Events) getGallery(start, num int) (*Gallery, error) {
	photos, err := c.Txn.Select(models.Photo{},
		"select * from Photo where EventId = ? order by Username, TakenStr limit ?, ?",
		c.Event.EventId, start, num)
	if err != nil {
		return nil, err
	}

	groupedPhotos := map[string][]*models.Photo{}
	for _, photoInterface := range photos {
		photo := photoInterface.(*models.Photo)
		if _, ok := groupedPhotos[photo.Username]; !ok {
			groupedPhotos[photo.Username] = []*models.Photo{}
		}
		groupedPhotos[photo.Username] = append(groupedPhotos[photo.Username], photo)
	}

	// TODO: Switch to Hood or resolve Gorp issue
	var total int
	row := db.Db.QueryRow("select count(*) from Photo where EventId = ?", c.Event.EventId)
	row.Scan(&total)

	return &Gallery{groupedPhotos, total}, nil
}

func (c Events) Upload() rev.Result {
	return c.Render()
}

var ORIENTATION_ANGLES = map[int]float64{
	1: 0.0,
	3: math.Pi,
	6: math.Pi * 3 / 2,
	8: math.Pi / 2,
}

func (c Events) PostUpload(name string) rev.Result {
	c.Validation.Required(name)

	if c.Validation.HasErrors() {
		c.FlashParams()
		c.Validation.Keep()
		return c.Redirect(Events.Upload)
	}

	photos := c.Params.Files["photos[]"]
	for _, photoFileHeader := range photos {
		// Open the photo.
		input, err := photoFileHeader.Open()
		if err != nil {
			c.FlashParams()
			c.Flash.Error("Error opening photo: %s", err)
			return c.Redirect(Events.Upload)
		}

		photoBytes, err := ioutil.ReadAll(input)
		input.Close()
		if err != nil || len(photoBytes) == 0 {
			rev.ERROR.Println("Failed to read image:", err)
			continue
		}

		// Decode the photo.
		photoImage, format, err := image.Decode(bytes.NewReader(photoBytes))
		if err != nil {
			rev.ERROR.Println("Failed to decode image:", err)
			continue
		}

		// Decode the EXIF data
		x, err := exif.Decode(bytes.NewReader(photoBytes))
		if err != nil {
			rev.ERROR.Println("Failed to decode image exif:", err)
			continue
		}

		// Look for the taken time in the EXIF.
		var taken time.Time
		if takenTag, err := x.Get("DateTimeOriginal"); err == nil {
			taken, err = time.Parse("2006:01:02 15:04:05", takenTag.StringVal())
			if err != nil {
				rev.ERROR.Println("Failed to parse time:", takenTag.StringVal(), ":", err)
			}
		}

		// Save a record of the photo to our database.
		rect := photoImage.Bounds()
		photo := models.Photo{
			EventId:  c.Event.EventId,
			Username: name,
			Format:   format,
			Filename: path.Base(photoFileHeader.Filename),
			Width:    int32(rect.Max.X - rect.Min.X),
			Height:   int32(rect.Max.Y - rect.Min.Y),
			Uploaded: time.Now(),
			Taken:    taken,
		}
		c.Txn.Insert(&photo)

		// Save the photo to S3.
		err = PHOTO_BUCKET.PutReader(photo.S3Path(),
			bytes.NewReader(photoBytes),
			int64(len(photoBytes)),
			fmt.Sprintf("image/%s", format),
			s3.PublicRead)
		if err != nil {
			c.FlashParams()
			c.Flash.Error("Error writing photo: %s", err)
			return c.Redirect(Events.Upload)
		}

		go SaveThumbnail(photo.PhotoId, photoImage, x, 250, 250)
		go SaveThumbnail(photo.PhotoId, photoImage, x, 940, 705)
	}

	c.Flash.Success("%d photos uploaded.", len(photos))
	return c.Redirect("/events/%d/view", c.Event.EventId)
}

func (c Events) PostDownload(photoIds []int) rev.Result {
	if len(photoIds) == 0 {
		return c.RenderError(fmt.Errorf("Nothing to download"))
	}

	c.Response.Out.Header().Set("Content-Disposition", "attachment")
	c.Response.WriteHeader(200, "application/zip")

	wr := zip.NewWriter(c.Response.Out)
	defer wr.Close()

	for _, photoId := range photoIds {
		url := PHOTO_BUCKET.URL(models.Photo{PhotoId: int32(photoId)}.S3Path())
		resp, err := http.Get(url)
		if err != nil {
			rev.ERROR.Println("Failed to get photo from S3:", err)
			continue
		}

		photoWr, err := wr.Create(fmt.Sprintf("%d.jpg", photoId))
		if err != nil {
			rev.ERROR.Println("Failed to create photo in zip:", err)
			resp.Body.Close()
			continue
		}

		_, err = io.Copy(photoWr, resp.Body)
		resp.Body.Close()
		if err != nil {
			rev.ERROR.Println("Error writing photo:", err)
			return nil
		}
	}

	return nil
}

type Pagination struct {
	CurrentPage int
	NumPages    int
	BaseUrl     string

	Start, End, Total int
}

func (p Pagination) Pages() []Page {
	pages := make([]Page, p.NumPages+2, p.NumPages+2)
	pages[0] = Page{
		Label:    "Prev",
		Disabled: p.CurrentPage == 1,
		Url:      p.PageUrl(p.CurrentPage - 1),
	}
	for i := 1; i <= p.NumPages; i++ {
		pages[i] = Page{
			Label:  fmt.Sprintf("%d", i),
			Active: i == p.CurrentPage,
			Url:    p.PageUrl(i),
		}
	}
	pages[p.NumPages+1] = Page{
		Label:    "Next",
		Disabled: p.CurrentPage == p.NumPages,
		Url:      p.PageUrl(p.CurrentPage + 1),
	}
	return pages
}

func (p Pagination) PageUrl(page int) template.HTML {
	return template.HTML(fmt.Sprintf("%s?page=%d", p.BaseUrl, page))
}

type Page struct {
	Label    string
	Active   bool
	Disabled bool
	Url      template.HTML
}
