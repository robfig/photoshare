package controllers

import (
	"github.com/robfig/revel"
	"math/rand"
	"time"
)

func init() {
	rand.Seed(time.Now().UnixNano())
	rev.RegisterPlugin(GorpPlugin{})
	rev.InterceptMethod((*GorpController).Begin, rev.BEFORE)
	rev.InterceptMethod((*GorpController).Commit, rev.AFTER)
	rev.InterceptMethod((*GorpController).Rollback, rev.FINALLY)
	rev.InterceptMethod((*Events).GetEvent, rev.BEFORE)
}
