package mastodon

import (
	"fmt"
	"net/http"

	"github.com/davecheney/pub/internal/algorithms"
	"github.com/davecheney/pub/internal/httpx"
	"github.com/davecheney/pub/internal/models"
	"github.com/davecheney/pub/internal/to"
	"github.com/go-chi/chi/v5"
	"gorm.io/gorm"
)

func TimelinesHome(env *Env, w http.ResponseWriter, r *http.Request) error {
	user, err := env.authenticate(r)
	if err != nil {
		return err
	}

	var followingIDs []int64
	if err := env.DB.Model(&models.Relationship{ActorID: user.Actor.ID}).Where("following = true").Pluck("target_id", &followingIDs).Error; err != nil {
		return httpx.Error(http.StatusInternalServerError, err)
	}
	followingIDs = append(followingIDs, int64(user.ID))

	var statuses []*models.Status
	// TODO stop copying and pasting this query
	scope := env.DB.Scopes(models.PaginateStatuses(r)).Where("(actor_id IN (?) AND in_reply_to_actor_id is null) or (actor_id in (?) and in_reply_to_actor_id IN (?))", followingIDs, followingIDs, followingIDs)
	query := scope.Joins("Actor")                                    // author, one join and one join only
	query = query.Preload("Reblog").Preload("Reblog.Actor")          // boosts
	query = query.Preload("Attachments")                             // media
	query = query.Preload("Reaction", "actor_id = ?", user.Actor.ID) // reactions
	query = query.Preload("Mentions").Preload("Mentions.Actor")      // mentions
	query = query.Preload("Tags").Preload("Tags.Tag")                // tags
	if err := query.Find(&statuses).Error; err != nil {
		return httpx.Error(http.StatusInternalServerError, err)
	}

	if len(statuses) > 0 {
		w.Header().Set("Link", fmt.Sprintf("<https://%s/api/v1/timelines/home?max_id=%d>; rel=\"next\", <https://%s/api/v1/timelines/home?min_id=%d>; rel=\"prev\"", r.Host, statuses[len(statuses)-1].ID, r.Host, statuses[0].ID))
	}
	return to.JSON(w, algorithms.Map(statuses, serialiseStatus))
}

func TimelinesPublic(env *Env, w http.ResponseWriter, r *http.Request) error {
	user, err := env.authenticate(r)
	authenticated := err == nil

	var statuses []*models.Status
	scope := env.DB.Scopes(models.PaginateStatuses(r)).Where("visibility = ? and reblog_id is null and in_reply_to_id is null", "public")
	switch r.URL.Query().Get("local") {
	case "true":
		scope = scope.Joins("Actor").Where("Actor.domain = ?", r.Host)
	default:
		scope = scope.Joins("Actor")
	}
	query := scope.Preload("Reblog").Preload("Reblog.Actor") // boosts
	query = query.Preload("Attachments")                     // media
	if authenticated {
		query = query.Preload("Reaction", "actor_id = ?", user.Actor.ID) // reactions
	}
	query = query.Preload("Mentions").Preload("Mentions.Actor") // mentions
	query = query.Preload("Tags").Preload("Tags.Tag")           // tags
	if err := query.Find(&statuses).Error; err != nil {
		return httpx.Error(http.StatusInternalServerError, err)
	}

	if len(statuses) > 0 {
		w.Header().Set("Link", fmt.Sprintf("<https://%s/api/v1/timelines/public?max_id=%d>; rel=\"next\", <https://%s/api/v1/timelines/public?min_id=%d>; rel=\"prev\"", r.Host, statuses[len(statuses)-1].ID, r.Host, statuses[0].ID))
	}
	return to.JSON(w, algorithms.Map(statuses, serialiseStatus))
}

func TimelinesListShow(env *Env, w http.ResponseWriter, r *http.Request) error {
	user, err := env.authenticate(r)
	if err != nil {
		return err
	}

	var listMembers []int64
	if err := env.DB.Model(&models.AccountListMember{}).Where("account_list_id = ? ", chi.URLParam(r, "id")).Pluck("member_id", &listMembers).Error; err != nil {
		return httpx.Error(http.StatusInternalServerError, err)
	}

	var statuses []*models.Status
	scope := env.DB.Scopes(models.PaginateStatuses(r)).Where("(actor_id IN (?) AND in_reply_to_actor_id is null) or (actor_id in (?) and in_reply_to_actor_id IN (?))", listMembers, listMembers, listMembers)
	query := scope.Joins("Actor")                                    // author, one join and one join only
	query = query.Preload("Reblog").Preload("Reblog.Actor")          // boosts
	query = query.Preload("Attachments")                             // media
	query = query.Preload("Reaction", "actor_id = ?", user.Actor.ID) // reactions
	query = query.Preload("Mentions").Preload("Mentions.Actor")      // mentions
	query = query.Preload("Tags").Preload("Tags.Tag")                // tags
	if err := query.Find(&statuses).Error; err != nil {
		return httpx.Error(http.StatusInternalServerError, err)
	}

	// if len(statuses) > 0 {
	// 	w.Header().Set("Link", fmt.Sprintf("<https://%s/api/v1/timelines/home?max_id=%d>; rel=\"next\", <https://%s/api/v1/timelines/home?min_id=%d>; rel=\"prev\"", r.Host, statuses[len(statuses)-1].ID, r.Host, statuses[0].ID))
	// }
	return to.JSON(w, algorithms.Map(statuses, serialiseStatus))
}

func TimelinesTagShow(env *Env, w http.ResponseWriter, r *http.Request) error {
	user, err := env.authenticate(r)
	if err != nil {
		return err
	}

	var tag models.Tag
	if err := env.DB.Where("name = ?", chi.URLParam(r, "tag")).First(&tag).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			// TODO move this tag lookup in a join in the query below so an unknown tag returns an empty result set.
			return to.JSON(w, []any{})
		}
		return err
	}

	var statuses []*models.Status
	scope := env.DB.Scopes(models.PaginateStatuses(r))
	// use Joins("JOIN status_tags ...") as Joins("Tags") -- joining on an association -- causes a reflect panic in gorm.
	// no biggie, just write the JOIN manually.
	query := scope.Joins("JOIN status_tags ON status_tags.status_id = statuses.id").Where("status_tags.tag_id = ?", tag.ID)
	query = query.Preload("Actor")
	query = query.Preload("Reblog").Preload("Reblog.Actor")          // boosts
	query = query.Preload("Attachments")                             // media
	query = query.Preload("Reaction", "actor_id = ?", user.Actor.ID) // reactions
	query = query.Preload("Mentions").Preload("Mentions.Actor")      // mentions
	query = query.Preload("Tags").Preload("Tags.Tag")                // tags
	if err := query.Find(&statuses).Error; err != nil {
		return httpx.Error(http.StatusInternalServerError, err)
	}

	// if len(statuses) > 0 {
	// 	w.Header().Set("Link", fmt.Sprintf("<https://%s/api/v1/timelines/home?max_id=%d>; rel=\"next\", <https://%s/api/v1/timelines/home?min_id=%d>; rel=\"prev\"", r.Host, statuses[len(statuses)-1].ID, r.Host, statuses[0].ID))
	// }
	return to.JSON(w, algorithms.Map(statuses, serialiseStatus))
}
