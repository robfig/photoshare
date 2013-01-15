package controllers

import (
	"fmt"
	"github.com/robfig/photoshare/app/models"
	"github.com/robfig/revel"
	"math/rand"
)

type Application struct {
	GorpController
}

func (c Application) Welcome() rev.Result {
	return c.Render()
}

func (c Application) CreateEvent(email, event string) rev.Result {
	c.Validation.Email(email).Message("Your email address is invalid")
	c.Validation.Required(event).Message("Please enter an event name")
	if c.Validation.HasErrors() {
		c.FlashParams()
		c.Validation.Keep()
		return c.Redirect(Application.Welcome)
	}

	e := models.Event{
		EventId: rand.Int(),
		Name:    event,
		Admin:   email,
	}

	err := c.Txn.Insert(&e)
	if err != nil {
		return c.RenderError(fmt.Errorf("Failed to create event: %s", err))
	}

	c.Flash.Success("Event created")
	return c.Redirect("/events/%d", e.EventId)
}
