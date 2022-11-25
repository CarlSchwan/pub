package main

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/go-json-experiment/json"

	"github.com/davecheney/m/activitypub"
	"github.com/davecheney/m/mastodon"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

type IndexCmd struct {
	DSN string `help:"data source name"`
}

func (i *IndexCmd) Run(ctx *Context) error {
	dsn := i.DSN + "?charset=utf8mb4&parseTime=True&loc=Local"
	db, err := gorm.Open(mysql.Open(dsn), &ctx.Config)
	if err != nil {
		return err
	}

	var activities []activitypub.Activity
	if err := db.Where("processed_at IS NULL").Find(&activities).Error; err != nil {
		return err
	}

	ip := &inboxProcessor{
		db:     db,
		actors: activitypub.NewActors(db),
	}

	for _, activity := range activities {
		if err := ip.Process(&activity); err != nil {
			fmt.Println(err)
			continue
		}
		t := time.Now()
		activity.ProcessedAt = &t
		if err := db.Save(&activity).Error; err != nil {
			return err
		}

	}

	return nil
}

type inboxProcessor struct {
	db     *gorm.DB
	actors *activitypub.Actors
}

func (ip *inboxProcessor) Process(activity *activitypub.Activity) error {
	var act map[string]any
	r := strings.NewReader(activity.Activity)
	if err := json.UnmarshalFull(r, &act); err != nil {
		return err
	}
	id, _ := act["id"].(string)
	typ, _ := act["type"].(string)
	actorID, _ := act["actor"].(string)
	fmt.Println("process: id:", id, "type:", typ, "actor:", actorID)
	actor, err := ip.actors.FindOrCreateActor(actorID)
	if err != nil {
		return err
	}
	account, err := ip.findOrCreateAccount(actor)
	if err != nil {
		return err
	}
	switch typ {
	case "Create":
		var create map[string]any
		r := strings.NewReader(activity.Activity)
		if err := json.UnmarshalFull(r, &create); err != nil {
			return err
		}
		return ip.processCreate(account, create["object"].(map[string]any))
	default:
		return nil
	}
}

func (ip *inboxProcessor) processCreate(account *mastodon.Account, obj map[string]any) error {
	// json.MarshalOptions{}.MarshalFull(json.EncodeOptions{Indent: "  "}, os.Stdout, obj)
	// fmt.Println()
	typ, _ := obj["type"].(string)
	switch typ {
	case "Note":
		return ip.processCreateNote(account, obj)
	default:
		return nil
	}
}

func (ip *inboxProcessor) processCreateNote(account *mastodon.Account, obj map[string]any) error {
	published, err := timeFromAny(obj["published"])
	if err != nil {
		return err
	}

	status := mastodon.Status{
		Model: gorm.Model{
			CreatedAt: published,
		},
		AccountID: account.ID,
		// InReplyToID        *uint
		// InReplyToAccountID *uint
		Sensitive:   boolFromAny(obj["sensitive"]),
		SpoilerText: stringFromAny(obj["summary"]),
		Visibility:  "public",
		Language:    "en",
		Content:     stringFromAny(obj["content"]),
	}

	if err := ip.db.Create(&status).Error; err != nil {
		return err
	}
	return nil
}

func boolFromAny(v any) bool {
	b, _ := v.(bool)
	return b
}

func stringFromAny(v any) string {
	s, _ := v.(string)
	return s
}

func timeFromAny(v any) (time.Time, error) {
	switch v := v.(type) {
	case string:
		return time.Parse(time.RFC3339, v)
	case time.Time:
		return v, nil
	default:
		return time.Time{}, errors.New("timeFromAny: invalid type")
	}
}

func (ip *inboxProcessor) findOrCreateAccount(actor *activitypub.Actor) (*mastodon.Account, error) {
	var account mastodon.Account
	err := ip.db.First(&account, "username = ? AND domain = ?", actor.Username(), actor.Domain()).Error
	if err == nil {
		// found cached key
		return &account, nil
	}

	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	var obj map[string]any
	if err := json.UnmarshalFull(bytes.NewReader(actor.Object), &obj); err != nil {
		return nil, err
	}
	account = mastodon.Account{
		Username:    actor.Username(),
		Domain:      actor.Domain(),
		DisplayName: obj["name"].(string),
		Locked:      false,
		Bot:         false,
		Note:        obj["summary"].(string),
		URL:         obj["url"].(string),
	}

	if err := ip.db.Create(&account).Error; err != nil {
		return nil, err
	}
	return &account, nil
}
