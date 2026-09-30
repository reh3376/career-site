package handlers

import (
	"context"
	"errors"
	"log/slog"
	"strconv"
	"strings"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	v1 "github.com/reh3376/career-site/services/api/gen/career/v1"
	"github.com/reh3376/career-site/services/api/internal/users"
)

// The Q&A bank's admin surface.
//
// Everything behind the bank worked before this existed and there was
// no way to put an entry into it, which made the bank a fast path in
// principle and nothing at all in practice. These RPCs are what make it
// real.
//
// An entry is not a cached answer. It is the owner's own words, served
// verbatim, never paraphrased by a model, and that property is the
// reason the bank can answer the topics the assistant otherwise refuses
// (FR-CHAT-06). The handlers below protect it in the only two places
// they can: nothing here sends an entry through generation, and
// enabling is treated as the approval rather than as a display toggle.

// qaEntryLimits are the server-side bounds. The proto validates them
// too; these exist because a validator can be bypassed by a caller that
// speaks the wire format directly, and because truncating quietly is
// worse than refusing.
const (
	qaQuestionMax = 500
	qaAnswerMax   = 8000
	qaSourcesMax  = 8
	qaTagsMax     = 12
	qaTagMax      = 40
)

// ListQaEntries returns the bank for the admin surface.
func (a *Admin) ListQaEntries(
	ctx context.Context, req *connect.Request[v1.ListQaEntriesRequest],
) (*connect.Response[v1.ListQaEntriesResponse], error) {
	if _, err := requireAdmin(a, ctx, req); err != nil {
		return nil, err
	}
	entries, err := a.users.ListQAEntries(ctx, req.Msg.GetIncludeDisabled())
	if err != nil {
		a.log.Error("ListQaEntries failed", slog.String("error", err.Error()))
		return nil, connect.NewError(connect.CodeInternal, errors.New("could not read the Q&A bank"))
	}
	out := &v1.ListQaEntriesResponse{Entries: make([]*v1.QaEntry, 0, len(entries))}
	for i := range entries {
		e := qaEntryToProto(&entries[i])
		// Counted across the whole bank rather than per entry, because
		// the question this answers is "can the bank match yet", and one
		// unembedded phrasing on one entry is a different situation from
		// a bank that has never been embedded at all.
		for _, p := range e.GetPhrasings() {
			if !p.GetEmbedded() {
				out.AwaitingEmbedding++
			}
		}
		out.Entries = append(out.Entries, e)
	}
	return connect.NewResponse(out), nil
}

// CreateQaEntry writes an entry and its canonical phrasing.
func (a *Admin) CreateQaEntry(
	ctx context.Context, req *connect.Request[v1.CreateQaEntryRequest],
) (*connect.Response[v1.CreateQaEntryResponse], error) {
	me, err := requireAdmin(a, ctx, req)
	if err != nil {
		return nil, err
	}
	entry, err := qaEntryFromRequest(
		req.Msg.GetQuestion(), req.Msg.GetAnswer(), req.Msg.GetSources(),
		req.Msg.GetTags(), req.Msg.GetCoversRestricted(), req.Msg.GetEnabled())
	if err != nil {
		return nil, err
	}
	for _, p := range req.Msg.GetPhrasings() {
		if t := strings.TrimSpace(p); t != "" {
			entry.Phrasings = append(entry.Phrasings, users.QAPhrasing{Text: capRunes(t, qaQuestionMax)})
		}
	}

	id, err := a.users.CreateQAEntry(ctx, entry)
	if err != nil {
		a.log.Error("CreateQaEntry failed", slog.String("error", err.Error()))
		return nil, connect.NewError(connect.CodeInternal, errors.New("could not save the entry"))
	}
	// Recorded because enabling a restricted-topic entry is the single
	// most consequential thing that happens on this screen: it is the
	// owner deciding the assistant may speak about compensation, or
	// references, or something personal.
	a.events.Emit(ctx, requestEvent(req, "admin.qa_entry_created", me.ID, map[string]any{
		"entry_id":          id,
		"enabled":           entry.Enabled,
		"covers_restricted": entry.CoversRestricted,
	}))
	return connect.NewResponse(&v1.CreateQaEntryResponse{Id: strconv.FormatInt(id, 10)}), nil
}

// UpdateQaEntry replaces an entry's editable fields.
func (a *Admin) UpdateQaEntry(
	ctx context.Context, req *connect.Request[v1.UpdateQaEntryRequest],
) (*connect.Response[v1.UpdateQaEntryResponse], error) {
	me, err := requireAdmin(a, ctx, req)
	if err != nil {
		return nil, err
	}
	id, err := parseID(req.Msg.GetId())
	if err != nil {
		return nil, err
	}
	entry, err := qaEntryFromRequest(
		req.Msg.GetQuestion(), req.Msg.GetAnswer(), req.Msg.GetSources(),
		req.Msg.GetTags(), req.Msg.GetCoversRestricted(), req.Msg.GetEnabled())
	if err != nil {
		return nil, err
	}
	entry.ID = id

	if err := a.users.UpdateQAEntry(ctx, entry); err != nil {
		if errors.Is(err, users.ErrQAEntryNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, errors.New("no such entry"))
		}
		a.log.Error("UpdateQaEntry failed", slog.String("error", err.Error()))
		return nil, connect.NewError(connect.CodeInternal, errors.New("could not save the entry"))
	}
	a.events.Emit(ctx, requestEvent(req, "admin.qa_entry_updated", me.ID, map[string]any{
		"entry_id":          id,
		"enabled":           entry.Enabled,
		"covers_restricted": entry.CoversRestricted,
	}))
	return connect.NewResponse(&v1.UpdateQaEntryResponse{}), nil
}

// SetQaEntryEnabled approves or withdraws an entry.
func (a *Admin) SetQaEntryEnabled(
	ctx context.Context, req *connect.Request[v1.SetQaEntryEnabledRequest],
) (*connect.Response[v1.SetQaEntryEnabledResponse], error) {
	me, err := requireAdmin(a, ctx, req)
	if err != nil {
		return nil, err
	}
	id, err := parseID(req.Msg.GetId())
	if err != nil {
		return nil, err
	}
	if err := a.users.SetQAEntryEnabled(ctx, id, req.Msg.GetEnabled()); err != nil {
		if errors.Is(err, users.ErrQAEntryNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, errors.New("no such entry"))
		}
		a.log.Error("SetQaEntryEnabled failed", slog.String("error", err.Error()))
		return nil, connect.NewError(connect.CodeInternal, errors.New("could not change the entry"))
	}
	a.events.Emit(ctx, requestEvent(req, "admin.qa_entry_enabled", me.ID, map[string]any{
		"entry_id": id, "enabled": req.Msg.GetEnabled(),
	}))
	return connect.NewResponse(&v1.SetQaEntryEnabledResponse{}), nil
}

// DeleteQaEntry removes an entry and its phrasings.
func (a *Admin) DeleteQaEntry(
	ctx context.Context, req *connect.Request[v1.DeleteQaEntryRequest],
) (*connect.Response[v1.DeleteQaEntryResponse], error) {
	me, err := requireAdmin(a, ctx, req)
	if err != nil {
		return nil, err
	}
	id, err := parseID(req.Msg.GetId())
	if err != nil {
		return nil, err
	}
	if err := a.users.DeleteQAEntry(ctx, id); err != nil {
		if errors.Is(err, users.ErrQAEntryNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, errors.New("no such entry"))
		}
		a.log.Error("DeleteQaEntry failed", slog.String("error", err.Error()))
		return nil, connect.NewError(connect.CodeInternal, errors.New("could not delete the entry"))
	}
	a.events.Emit(ctx, requestEvent(req, "admin.qa_entry_deleted", me.ID, map[string]any{"entry_id": id}))
	return connect.NewResponse(&v1.DeleteQaEntryResponse{}), nil
}

// AddQaPhrasing adds another way of asking an entry's question.
func (a *Admin) AddQaPhrasing(
	ctx context.Context, req *connect.Request[v1.AddQaPhrasingRequest],
) (*connect.Response[v1.AddQaPhrasingResponse], error) {
	if _, err := requireAdmin(a, ctx, req); err != nil {
		return nil, err
	}
	entryID, err := parseID(req.Msg.GetEntryId())
	if err != nil {
		return nil, err
	}
	text := strings.TrimSpace(req.Msg.GetText())
	if text == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("the phrasing is empty"))
	}
	id, err := a.users.AddQAPhrasing(ctx, entryID, capRunes(text, qaQuestionMax))
	if err != nil {
		if errors.Is(err, users.ErrQAEntryNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, errors.New("no such entry"))
		}
		a.log.Error("AddQaPhrasing failed", slog.String("error", err.Error()))
		return nil, connect.NewError(connect.CodeInternal, errors.New("could not add the phrasing"))
	}
	return connect.NewResponse(&v1.AddQaPhrasingResponse{Id: strconv.FormatInt(id, 10)}), nil
}

// DeleteQaPhrasing removes one variant.
func (a *Admin) DeleteQaPhrasing(
	ctx context.Context, req *connect.Request[v1.DeleteQaPhrasingRequest],
) (*connect.Response[v1.DeleteQaPhrasingResponse], error) {
	if _, err := requireAdmin(a, ctx, req); err != nil {
		return nil, err
	}
	entryID, err := parseID(req.Msg.GetEntryId())
	if err != nil {
		return nil, err
	}
	phrasingID, err := parseID(req.Msg.GetPhrasingId())
	if err != nil {
		return nil, err
	}
	if err := a.users.DeleteQAPhrasing(ctx, entryID, phrasingID); err != nil {
		if errors.Is(err, users.ErrQAEntryNotFound) {
			// Also the answer for the canonical phrasing, which the
			// store refuses to delete. Saying "no such phrasing" there
			// would be a lie; the surface does not offer the button, so
			// reaching this means something went round it.
			return nil, connect.NewError(connect.CodeNotFound,
				errors.New("no such phrasing, or it is the entry's own question"))
		}
		a.log.Error("DeleteQaPhrasing failed", slog.String("error", err.Error()))
		return nil, connect.NewError(connect.CodeInternal, errors.New("could not remove the phrasing"))
	}
	return connect.NewResponse(&v1.DeleteQaPhrasingResponse{}), nil
}

// qaEntryFromRequest validates and normalises the fields shared by
// create and update.
//
// Refuses rather than truncates. An answer silently cut at 8,000
// characters would be served verbatim in its cut form, and "verbatim"
// is the whole promise the bank makes.
func qaEntryFromRequest(
	question, answer string, sources []*v1.QaSource, tags []string,
	coversRestricted, enabled bool,
) (users.QAEntry, error) {
	q := strings.TrimSpace(question)
	ans := strings.TrimSpace(answer)
	if q == "" {
		return users.QAEntry{}, connect.NewError(connect.CodeInvalidArgument,
			errors.New("the question is empty"))
	}
	if ans == "" {
		return users.QAEntry{}, connect.NewError(connect.CodeInvalidArgument,
			errors.New("the answer is empty"))
	}
	if len([]rune(q)) > qaQuestionMax {
		return users.QAEntry{}, connect.NewError(connect.CodeInvalidArgument,
			errors.New("the question is too long; it is shown in a list and used as a suggestion"))
	}
	if len([]rune(ans)) > qaAnswerMax {
		return users.QAEntry{}, connect.NewError(connect.CodeInvalidArgument,
			errors.New("the answer is too long; it is served word for word and cannot be shortened later"))
	}
	if len(sources) > qaSourcesMax {
		return users.QAEntry{}, connect.NewError(connect.CodeInvalidArgument,
			errors.New("too many sources for one answer"))
	}

	out := users.QAEntry{
		Question:         q,
		Answer:           ans,
		CoversRestricted: coversRestricted,
		Enabled:          enabled,
	}
	for _, s := range sources {
		title := strings.TrimSpace(s.GetTitle())
		path := strings.TrimSpace(s.GetPath())
		if title == "" && path == "" {
			continue
		}
		// A source that links off the site is not a source for an answer
		// about the owner's own records, and an absolute URL in this
		// field would render as a site path and go somewhere else.
		//
		// Leading "/" is not enough on its own, which a test caught:
		// "//example.test/page" starts with a slash and a browser reads
		// it as protocol-relative, so it navigates off-site. "/\" is
		// treated the same way by some browsers. Both are refused, which
		// makes this "a path on this site" rather than "a string
		// beginning with a slash".
		if path != "" && (!strings.HasPrefix(path, "/") ||
			strings.HasPrefix(path, "//") || strings.HasPrefix(path, `/\`)) {
			return users.QAEntry{}, connect.NewError(connect.CodeInvalidArgument,
				errors.New("a source path must be a path on this site, starting with a single /"))
		}
		out.Sources = append(out.Sources, users.QASource{Title: title, Path: path})
	}
	for _, t := range tags {
		t = strings.ToLower(strings.TrimSpace(t))
		if t == "" {
			continue
		}
		if len(out.Tags) >= qaTagsMax {
			break
		}
		out.Tags = append(out.Tags, capRunes(t, qaTagMax))
	}
	return out, nil
}

func qaEntryToProto(e *users.QAEntry) *v1.QaEntry {
	out := &v1.QaEntry{
		Id:               strconv.FormatInt(e.ID, 10),
		Question:         e.Question,
		Answer:           e.Answer,
		Tags:             e.Tags,
		CoversRestricted: e.CoversRestricted,
		Enabled:          e.Enabled,
		CreatedAt:        timestamppb.New(e.CreatedAt),
		UpdatedAt:        timestamppb.New(e.UpdatedAt),
	}
	for _, s := range e.Sources {
		out.Sources = append(out.Sources, &v1.QaSource{Title: s.Title, Path: s.Path})
	}
	for _, p := range e.Phrasings {
		out.Phrasings = append(out.Phrasings, &v1.QaPhrasing{
			Id:        strconv.FormatInt(p.ID, 10),
			Text:      p.Text,
			Canonical: p.Canonical,
			Embedded:  p.HasVector,
		})
	}
	return out
}

func capRunes(s string, n int) string {
	r := []rune(s)
	if len(r) > n {
		return string(r[:n])
	}
	return s
}
