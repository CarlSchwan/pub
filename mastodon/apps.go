package mastodon

import (
	"errors"
	"net/http"

	"github.com/davecheney/pub/internal/httpx"
	"github.com/davecheney/pub/internal/mime"
	"github.com/davecheney/pub/internal/models"
	"github.com/davecheney/pub/internal/snowflake"
	"github.com/davecheney/pub/internal/to"
	"github.com/go-json-experiment/json"
	"github.com/google/uuid"
)

func AppsCreate(env *Env, w http.ResponseWriter, r *http.Request) error {
	var params struct {
		ClientName   string  `json:"client_name"`
		Website      *string `json:"website"`
		RedirectURIs string  `json:"redirect_uris"`
		Scopes       string  `json:"scopes"`
	}
	switch mt := mime.MediaType(r); mt {
	case "application/x-www-form-urlencoded", "multipart/form-data":
		params.ClientName = r.PostFormValue("client_name")
		params.Website = ptr(r.PostFormValue("website"))
		params.RedirectURIs = r.PostFormValue("redirect_uris")
		params.Scopes = r.PostFormValue("scopes")
	case "application/json":
		if err := json.UnmarshalFull(r.Body, &params); err != nil {
			return httpx.Error(http.StatusBadRequest, err)
		}
	default:
		return httpx.Error(http.StatusUnsupportedMediaType, errors.New("unsupported media type: "+mt))
	}

	var instance models.Instance
	if err := env.DB.Take(&instance, "domain = ?", r.Host).Error; err != nil {
		return httpx.Error(http.StatusNotFound, err)
	}

	app := &models.Application{
		ID:           snowflake.Now(),
		InstanceID:   instance.ID,
		Name:         params.ClientName,
		Website:      params.Website,
		ClientID:     uuid.New().String(),
		ClientSecret: uuid.New().String(),
		RedirectURI:  params.RedirectURIs,
		VapidKey:     "BCk-QqERU0q-CfYZjcuB6lnyyOYfJ2AifKqfeGIm7Z-HiTU5T9eTG5GxVA0_OH5mMlI4UkkDTpaZwozy0TzdZ2M=",
	}
	if err := env.DB.Create(app).Error; err != nil {
		return err
	}

	return to.JSON(w, serialiseApplication(app))
}

func ptr[T any](v T) *T {
	return &v
}
