package main

import "gorm.io/gorm"

type ImportCmd struct {
	URI string `short:"u" long:"uri" description:"URI to import"`
}

func (c *ImportCmd) Run(ctx *Context) error {
	db, err := gorm.Open(ctx.Dialector, &ctx.Config)
	if err != nil {
		return err
	}
	_ = db
	return nil
}
