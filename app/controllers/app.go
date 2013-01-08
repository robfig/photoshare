package controllers

import (
	"archive/zip"
	"bytes"
	"code.google.com/p/graphics-go/graphics"
	"fmt"
	"github.com/robfig/photoshare/app/models"
	"github.com/robfig/revel"
	"github.com/rwcarlsen/goexif/exif"
	"image"
	_ "image/gif"
	"image/jpeg"
	_ "image/png"
	"io"
	"io/ioutil"
	"math"
	"os"
	"path"
	"time"
)

var PHOTO_DIRECTORY string

type Application struct {
	GorpController
}

const (
	VIEW     = "Application/View.html"
	DOWNLOAD = "Application/Download.html"
)

type Grouping string

const (
	BY_USER Grouping = "Username"
	BY_DATE          = "TakenStr"
)

func (c Application) View() rev.Result {
	return c.gallery(VIEW)
}

func (c Application) Download() rev.Result {
	return c.gallery(DOWNLOAD)
}

func (c Application) gallery(template string) rev.Result {
	gallery, err := c.getGallery(0, 100)
	if err != nil {
		return c.RenderError(err)
	}
	c.RenderArgs["gallery"] = gallery
	return c.RenderTemplate(template)
}

func (c Application) ViewPhoto(username, filename string) rev.Result {
	photos, err := c.Txn.Select(models.Photo{},
		"select * from Photo where Username = ? and Name = ?",
		username, filename)
	if err != nil {
		return c.RenderError(err)
	}

	if len(photos) == 0 {
		return c.NotFound("No photo found.")
	}

	photo := photos[0]
	return c.Render(photo)
}

// Return an array of user names to photo paths.
// TODO: How to get the map to be ordered.
func (c Application) getGallery(start, num int) (map[string][]*models.Photo, error) {
	photos, err := c.Txn.Select(models.Photo{},
		"select * from Photo order by Username, TakenStr limit ?, ?",
		start, num)
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
	return groupedPhotos, nil
}

func (c Application) Upload() rev.Result {
	return c.Render()
}

var ORIENTATION_ANGLES = map[int]float64{
	1: 0.0,
	3: math.Pi,
	6: math.Pi * 3 / 2,
	8: math.Pi / 2,
}

func (c Application) PostUpload(name string) rev.Result {
	c.Validation.Required(name)

	if c.Validation.HasErrors() {
		c.FlashParams()
		c.Validation.Keep()
		return c.Redirect(Application.Upload)
	}

	photoDir := path.Join(PHOTO_DIRECTORY, name)
	thumbDir := path.Join(PHOTO_DIRECTORY, "thumbs", name)
	err := os.MkdirAll(photoDir, 0777)
	if err != nil {
		c.FlashParams()
		c.Flash.Error("Error making directory:", err)
		return c.Redirect(Application.Upload)
	}
	err = os.MkdirAll(thumbDir, 0777)
	if err != nil {
		c.FlashParams()
		c.Flash.Error("Error making directory:", err)
		return c.Redirect(Application.Upload)
	}

	photos := c.Params.Files["photos[]"]
	for _, photoFileHeader := range photos {
		// Open the photo.
		input, err := photoFileHeader.Open()
		if err != nil {
			c.FlashParams()
			c.Flash.Error("Error opening photo:", err)
			return c.Redirect(Application.Upload)
		}

		photoBytes, err := ioutil.ReadAll(input)
		if err != nil || len(photoBytes) == 0 {
			rev.ERROR.Println("Failed to read image:", err)
			continue
		}
		input.Close()

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

		var orientation int = 1
		if orientationTag, err := x.Get(exif.Orientation); err == nil {
			orientation = int(orientationTag.Int(0))
		}

		photoName := path.Base(photoFileHeader.Filename)

		// Create a thumbnail
		thumbnail := image.NewRGBA(image.Rect(0, 0, 256, 256))
		err = graphics.Thumbnail(thumbnail, photoImage)
		if err != nil {
			rev.ERROR.Println("Failed to create thumbnail:", err)
			continue
		}

		// If the EXIF said to, rotate the thumbnail.
		// TODO: maintain the EXIF in the thumb instead.
		if orientation != 1 {
			if angleRadians, ok := ORIENTATION_ANGLES[orientation]; ok {
				rotatedThumbnail := image.NewRGBA(image.Rect(0, 0, 256, 256))
				err = graphics.Rotate(rotatedThumbnail, thumbnail, &graphics.RotateOptions{Angle: angleRadians})
				if err != nil {
					rev.ERROR.Println("Failed to rotate:", err)
				} else {
					thumbnail = rotatedThumbnail
				}
			}
		}

		thumbnailFile, err := os.Create(path.Join(thumbDir, photoName))
		if err != nil {
			c.FlashParams()
			c.Flash.Error("Error creating file:", err)
			return c.Redirect(Application.Upload)
		}

		err = jpeg.Encode(thumbnailFile, thumbnail, nil)
		if err != nil {
			c.FlashParams()
			c.Flash.Error("Failed to save thumbnail:", err)
			return c.Redirect(Application.Upload)
		}

		// Save the photo
		output, err := os.Create(path.Join(photoDir, photoName))
		if err != nil {
			c.FlashParams()
			c.Flash.Error("Error creating file:", err)
			return c.Redirect(Application.Upload)
		}

		_, err = io.Copy(output, bytes.NewReader(photoBytes))
		output.Close()
		if err != nil {
			c.FlashParams()
			c.Flash.Error("Error writing photo:", err)
			return c.Redirect(Application.Upload)
		}

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
			Username: name,
			Format:   format,
			Name:     photoName,
			Width:    rect.Max.X - rect.Min.X,
			Height:   rect.Max.Y - rect.Min.Y,
			Uploaded: time.Now(),
			Taken:    taken,
		}

		c.Txn.Insert(&photo)
	}

	c.Flash.Success("%d photos uploaded.", len(photos))
	return c.Redirect(Application.View)
}

func (c Application) PostDownload(paths []string) rev.Result {
	if len(paths) == 0 {
		return c.RenderError(fmt.Errorf("Nothing to download"))
	}

	c.Response.Out.Header().Set("Content-Disposition", "attachment")
	c.Response.WriteHeader(200, "application/zip")

	wr := zip.NewWriter(c.Response.Out)
	defer wr.Close()

	for _, photoPath := range paths {
		file, err := os.Open(path.Join(PHOTO_DIRECTORY, photoPath))
		if err != nil {
			rev.ERROR.Println("Failed to create photo in zip:", err)
			continue
		}

		photoWr, err := wr.Create(photoPath)
		if err != nil {
			rev.ERROR.Println("Failed to create photo in zip:", err)
			continue
		}

		_, err = io.Copy(photoWr, file)
		if err != nil {
			rev.ERROR.Println("Error writing photo:", err)
			return nil
		}
	}

	return nil
}

type PhotoServerPlugin struct {
	rev.EmptyPlugin
}

func (t PhotoServerPlugin) OnRoutesLoaded(router *rev.Router) {
	router.Routes = append([]*rev.Route{
		rev.NewRoute("GET", "/photos/", "staticDir:"+PHOTO_DIRECTORY),
	}, router.Routes...)
}

func init() {
	rev.InitHooks = append(rev.InitHooks, func() {
		var ok bool
		PHOTO_DIRECTORY, ok = rev.Config.String("datadir")
		if !ok {
			rev.ERROR.Fatalln("Must define datadir in app.conf")
		}
	})
	rev.RegisterPlugin(PhotoServerPlugin{})
}
