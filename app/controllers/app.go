package controllers

import (
	"fmt"
	"github.com/robfig/revel"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"os"
	"path"
)

const PHOTO_DIRECTORY = "/Users/robfig/whartonphotos"

type Application struct {
	*rev.Controller
}

func (c Application) Index() rev.Result {
	dir, err := os.Open(PHOTO_DIRECTORY)
	if err != nil {
		return c.RenderError(fmt.Errorf("Failed to open photo directory: %s", err))
	}

	fileInfos, err := dir.Readdir(-1)
	if err != nil {
		return c.RenderError(fmt.Errorf("Failed to read photo directory: %s", err))
	}

	userPhotos := map[string][]string{}
	for _, fileInfo := range fileInfos {
		if fileInfo.IsDir() {
			userPhotos[fileInfo.Name()] = []string{}
		}
	}

	for username, _ := range userPhotos {
		userDir, err := os.Open(path.Join(PHOTO_DIRECTORY, username))
		if err != nil {
			return c.RenderError(fmt.Errorf("Failed to open user's directory: %s", err))
		}

		names, err := userDir.Readdirnames(-1)
		if err != nil {
			return c.RenderError(fmt.Errorf("Failed to read user's directory: %s", err))
		}

		userPhotos[username] = names
	}

	return c.Render(userPhotos)
}

func (c Application) Upload() rev.Result {
	return c.Render()
}

// TODO: Should be able to accept photos []*multipart.FileHeader
// TODO: Create thumbnails.  Use a native go library or imagemagick/graphicsmagick
// TODO: Read EXIF data and allow reset by time zone?
func (c Application) PostUpload(name string) rev.Result {
	c.Validation.Required(name)

	if c.Validation.HasErrors() {
		c.FlashParams()
		c.Validation.Keep()
		return c.Redirect(Application.Upload)
	}

	userDir := path.Join(PHOTO_DIRECTORY, name)
	err := os.MkdirAll(userDir, 0777)
	if err != nil {
		c.FlashParams()
		c.Flash.Error("Error making directory:", err)
		return c.Redirect(Application.Upload)
	}

	photos := c.Params.Files["photos[]"]
	for _, photo := range photos {
		output, err := os.Create(path.Join(userDir, path.Base(photo.Filename)))
		if err != nil {
			c.FlashParams()
			c.Flash.Error("Error creating file:", err)
			return c.Redirect(Application.Upload)
		}

		input, err := photo.Open()
		if err != nil {
			c.FlashParams()
			c.Flash.Error("Error opening photo:", err)
			return c.Redirect(Application.Upload)
		}

		_, err = io.Copy(output, input)
		if err != nil {
			c.FlashParams()
			c.Flash.Error("Error writing photo:", err)
			return c.Redirect(Application.Upload)
		}
		input.Close()
		output.Close()
	}

	c.Flash.Success("%d photos uploaded.", len(photos))
	return c.Redirect(Application.Index)
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
	rev.TemplateFuncs["each"] = func(a, b int) bool { return a%b == 0 }
	rev.RegisterPlugin(PhotoServerPlugin{})
}
