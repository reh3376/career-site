package handlers

import (
	"context"
	"errors"
	"log/slog"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	v1 "github.com/reh3376/career-site/services/api/gen/career/v1"
	"github.com/reh3376/career-site/services/api/gen/career/v1/careerv1connect"
	"github.com/reh3376/career-site/services/api/internal/users"
)

// Member implements careerv1connect.MemberServiceHandler. Only GetMe +
// GetHistory are live today; the rest fall through to the embedded
// UNIMPLEMENTED stubs.
type Member struct {
	careerv1connect.UnimplementedMemberServiceHandler

	auth  *Auth
	users *users.Repo
	// log exists so a welcome-back lookup that fails is visible. The
	// panel is deliberately non-fatal to the history call, and a
	// non-fatal failure with nowhere to report it is how a surface
	// quietly stops working.
	log *slog.Logger
}

func NewMember(log *slog.Logger, auth *Auth, repo *users.Repo) *Member {
	return &Member{log: log, auth: auth, users: repo}
}

// GetMe resolves the caller's session cookie and returns their Me. No
// session or an invalid one returns Unauthenticated so the client can
// route to /login.
func (m *Member) GetMe(
	ctx context.Context,
	req *connect.Request[v1.GetMeRequest],
) (*connect.Response[v1.GetMeResponse], error) {
	u, err := RequireCaller(ctx)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&v1.GetMeResponse{Me: m.auth.buildMe(u)}), nil
}

// GetHistory returns the caller's recent activity, newest first. When
// `kind` is set on the request, filters to that kind server-side.
// Powers the personalised /home "recent activity" panel.
func (m *Member) GetHistory(
	ctx context.Context,
	req *connect.Request[v1.GetHistoryRequest],
) (*connect.Response[v1.GetHistoryResponse], error) {
	u, err := RequireCaller(ctx)
	if err != nil {
		return nil, err
	}
	// The proto's PageRequest carries page_size; treat missing / <=0
	// as the default 50 (matches admin surface).
	limit := 50
	if p := req.Msg.Page; p != nil && p.PageSize > 0 {
		limit = int(p.PageSize)
	}
	events, err := m.users.RecentActivity(ctx, u.ID, limit)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.New("history lookup failed"))
	}
	// Optional kind filter — cheap client-side pass; the volume per
	// user is small enough that a dedicated query isn't worth it.
	wantKind := activityKindProtoToRepo(req.Msg.Kind)
	out := make([]*v1.ActivityEvent, 0, len(events))
	for i := range events {
		if wantKind != "" && events[i].Kind != wantKind {
			continue
		}
		out = append(out, activityEventRepoToProto(&events[i]))
	}
	// The welcome-back panel (FR-HIST-02). A failure here is not worth
	// failing the whole history call over: the panel is a greeting, the
	// activity list is the content, and a member who loses the greeting
	// should still get the page.
	wb, wbErr := m.users.WelcomeBackFor(ctx, u.ID)
	if wbErr != nil {
		m.log.Warn("welcome-back lookup failed",
			slog.Int64("user_id", u.ID), slog.String("error", wbErr.Error()))
	}

	return connect.NewResponse(&v1.GetHistoryResponse{
		Events:      out,
		Page:        &v1.PageResponse{TotalCount: int32(len(out))},
		WelcomeBack: welcomeBackToProto(wb),
	}), nil
}

// welcomeBackToProto fills only the fields that have data behind them.
//
// recent_topics, new_since_last_visit and resume_conversation_id stay
// unset: the first needs instrumentation on the chat send path that does
// not exist, the second needs per-item update timestamps the content
// model does not carry (articles date to a bare year), and the third is
// a separate feature. Each is left empty rather than filled with a
// guess, and the proto says so at the field.
func welcomeBackToProto(w users.WelcomeBack) *v1.WelcomeBack {
	out := &v1.WelcomeBack{PreviousVisits: w.PreviousVisits}
	if w.LastVisitAt != nil {
		out.LastVisitAt = timestamppb.New(w.LastVisitAt.UTC())
	}
	for _, id := range w.RecentlyViewed {
		// Only the id. The web app owns content metadata and resolves
		// the title and the link from its own files; duplicating that
		// knowledge in the API would give the two a way to disagree.
		out.RecentlyViewed = append(out.RecentlyViewed, &v1.ContentSummary{Id: id})
	}
	return out
}
