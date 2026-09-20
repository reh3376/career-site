package handlers

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	v1 "github.com/reh3376/career-site/services/api/gen/career/v1"
	"github.com/reh3376/career-site/services/api/gen/career/v1/careerv1connect"
	"github.com/reh3376/career-site/services/api/internal/users"
)

// Activity implements careerv1connect.ActivityServiceHandler. The
// client sends batches of view / download / search / save events;
// the handler persists them (de-duped by client_event_id) and
// returns how many were new vs already stored.
type Activity struct {
	careerv1connect.UnimplementedActivityServiceHandler

	log   *slog.Logger
	users *users.Repo
	auth  *Auth
}

func NewActivity(log *slog.Logger, repo *users.Repo, auth *Auth) *Activity {
	return &Activity{log: log, users: repo, auth: auth}
}

// RecordEvents inserts every event in the batch that isn't a
// duplicate. Requires a member session — anonymous callers get
// Unauthenticated. Failures on a single event never fail the batch:
// the response counts what actually landed.
func (a *Activity) RecordEvents(
	ctx context.Context,
	req *connect.Request[v1.RecordEventsRequest],
) (*connect.Response[v1.RecordEventsResponse], error) {
	u, err := a.auth.LookupSessionUser(ctx, req)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.New("session lookup failed"))
	}
	if u == nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("not signed in"))
	}

	var accepted, dups int32
	now := time.Now().UTC()
	// Clamp per-event occurred_at into [now-24h, now]. The proto
	// comment already promises this — do it here so downstream
	// queries never see future or ancient timestamps.
	minAt := now.Add(-24 * time.Hour)

	for _, ev := range req.Msg.Events {
		if ev == nil {
			continue
		}
		kind := activityKindProtoToRepo(ev.Kind)
		if kind == "" {
			continue
		}
		at := now
		if ev.OccurredAt != nil {
			at = ev.OccurredAt.AsTime()
			if at.Before(minAt) {
				at = minAt
			}
			if at.After(now) {
				at = now
			}
		}
		inserted, err := a.users.RecordActivity(ctx, users.ActivityEvent{
			UserID:         u.ID,
			Kind:           kind,
			ContentID:      ev.ContentId,
			ConversationID: ev.ConversationId,
			Query:          ev.Query,
			Variant:        ev.Variant,
			DwellMs:        ev.DwellMs,
			ClientEventID:  ev.ClientEventId,
			OccurredAt:     at,
		})
		if err != nil {
			a.log.Warn("record activity failed",
				slog.Int64("user_id", u.ID),
				slog.String("kind", kind),
				slog.String("error", err.Error()),
			)
			continue
		}
		if inserted {
			accepted++
		} else {
			dups++
		}
	}
	return connect.NewResponse(&v1.RecordEventsResponse{
		Accepted:   accepted,
		Duplicates: dups,
	}), nil
}

// activityKindProtoToRepo maps proto Kind enums to the string slugs
// the DB uses. Empty return = skip this event.
func activityKindProtoToRepo(k v1.ActivityEvent_Kind) string {
	switch k {
	case v1.ActivityEvent_KIND_VIEW:
		return "view"
	case v1.ActivityEvent_KIND_DOWNLOAD:
		return "download"
	case v1.ActivityEvent_KIND_SEARCH:
		return "search"
	case v1.ActivityEvent_KIND_SAVE:
		return "save"
	case v1.ActivityEvent_KIND_CHAT:
		return "chat"
	case v1.ActivityEvent_KIND_ESCALATE:
		return "escalate"
	case v1.ActivityEvent_KIND_LOGIN:
		return "login"
	case v1.ActivityEvent_KIND_LOGOUT:
		return "logout"
	default:
		return ""
	}
}

func activityKindRepoToProto(kind string) v1.ActivityEvent_Kind {
	switch kind {
	case "view":
		return v1.ActivityEvent_KIND_VIEW
	case "download":
		return v1.ActivityEvent_KIND_DOWNLOAD
	case "search":
		return v1.ActivityEvent_KIND_SEARCH
	case "save":
		return v1.ActivityEvent_KIND_SAVE
	case "chat":
		return v1.ActivityEvent_KIND_CHAT
	case "escalate":
		return v1.ActivityEvent_KIND_ESCALATE
	case "login":
		return v1.ActivityEvent_KIND_LOGIN
	case "logout":
		return v1.ActivityEvent_KIND_LOGOUT
	default:
		return v1.ActivityEvent_KIND_UNSPECIFIED
	}
}

// activityEventRepoToProto converts a stored event for the admin
// view. The proto message carries dwell_ms / query / variant /
// content / conversation only for the kinds that populate them.
func activityEventRepoToProto(e *users.ActivityEvent) *v1.ActivityEvent {
	out := &v1.ActivityEvent{
		Kind:           activityKindRepoToProto(e.Kind),
		ContentId:      e.ContentID,
		ConversationId: e.ConversationID,
		Query:          e.Query,
		Variant:        e.Variant,
		DwellMs:        e.DwellMs,
		ClientEventId:  e.ClientEventID,
		OccurredAt:     timestamppb.New(e.OccurredAt),
	}
	return out
}
