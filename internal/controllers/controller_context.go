package controllers

import (
	"context"
	"time"

	domainentity "server-apeiron/internal/domain/entity"
	"server-apeiron/internal/domain/ids"
)

type ControllerContext struct {
	Context  context.Context
	Now      time.Time
	Delta    time.Duration
	Tick     uint64
	RegionID ids.RegionID
}

type EntityContext struct {
	ControllerContext
	Entity domainentity.Entity
}

func (c EntityContext) RuntimeID() ids.RuntimeEntityID {
	if c.Entity == nil {
		return ids.InvalidRuntimeEntityID
	}

	return c.Entity.RuntimeID()
}
