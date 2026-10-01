package handlers

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	careerv1 "github.com/reh3376/career-site/services/api/gen/career/v1"
	"github.com/reh3376/career-site/services/api/gen/career/v1/careerv1connect"
	"github.com/reh3376/career-site/services/api/internal/chat"
	"github.com/reh3376/career-site/services/api/internal/prompts"
	"github.com/reh3376/career-site/services/api/internal/users"
)

// Chat implements careerv1connect.ChatServiceHandler: Ask Roger.
//
// The contract in proto/career/v1/chat.proto predates every part of the
// implementation and is followed as written rather than trimmed to what
// exists today. Two RPCs are left unimplemented and say so plainly:
// Escalate (FR-CHAT-10, nothing behind it yet) and GetQuota
// (FR-CHAT-12, no quotas yet). A stub returning plausible zeroes would
// be worse than a refusal, because the panel would render an allowance
// the member does not actually have.
//
// Members-only, like the scheduler and the JD reviewer. The proto
// declares AUTH_LEVEL_MEMBER and enforcement lives here until the auth
// interceptor lands.
type Chat struct {
	careerv1connect.UnimplementedChatServiceHandler

	log    *slog.Logger
	users  *users.Repo
	auth   *Auth
	answer *chat.Service
}

// NewChat wires the handler. A nil answer service is not an error: the
// conversation surfaces still work and SendMessage refuses, which is
// the state before the sidecar is reachable.
func NewChat(log *slog.Logger, repo *users.Repo, auth *Auth, answer *chat.Service) *Chat {
	return &Chat{log: log, users: repo, auth: auth, answer: answer}
}

// requireMember resolves the caller's session or fails the RPC.
func (h *Chat) requireMember(ctx context.Context, req connect.AnyRequest) (*users.User, error) {
	if h.auth == nil {
		return nil, connect.NewError(connect.CodeInternal, errors.New("auth not wired"))
	}
	u, err := h.auth.LookupSessionUser(ctx, req)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.New("session lookup failed"))
	}
	if u == nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("sign in to ask a question"))
	}
	if u.Status != users.StatusActive {
		return nil, connect.NewError(connect.CodePermissionDenied, errors.New("account is not active"))
	}
	return u, nil
}

// DisclosureText is the first message of every conversation
// (FR-CHAT-02).
//
// Sent as a real assistant message rather than chrome above the thread,
// because the requirement is that the disclosure is in the conversation
// and chrome is what a reader scrolls past. It is not persisted: it is
// the same sentence every time, it would be the first row of every
// thread forever, and a stored copy would go stale the moment the
// wording changed.
const DisclosureText = "I am an AI assistant. I answer in Roger's voice, using only his own records, and I show you where each answer came from. I get things wrong sometimes, so check anything that matters. [How this works](/how-ask-roger-works)"

// CreateConversation opens a thread and returns the disclosure.
func (h *Chat) CreateConversation(
	ctx context.Context, req *connect.Request[careerv1.CreateConversationRequest],
) (*connect.Response[careerv1.CreateConversationResponse], error) {
	me, err := h.requireMember(ctx, req)
	if err != nil {
		return nil, err
	}
	persona := prompts.AskRogerPersona.Fingerprint()
	id, err := h.users.CreateConversation(ctx, users.Conversation{
		UserID:           me.ID,
		PersonaVersion:   persona,
		ContextContentID: strings.TrimSpace(req.Msg.GetContextContentId()),
	})
	if err != nil {
		h.log.Error("CreateConversation failed", slog.String("error", err.Error()))
		return nil, connect.NewError(connect.CodeInternal, errors.New("could not start a conversation"))
	}
	conv, _, err := h.users.GetConversation(ctx, me.ID, id)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.New("could not start a conversation"))
	}
	return connect.NewResponse(&careerv1.CreateConversationResponse{
		Conversation: conversationToProto(conv),
		Disclosure: &careerv1.Message{
			ConversationId: strconv.FormatInt(id, 10),
			Role:           careerv1.Message_ROLE_ASSISTANT,
			Text:           DisclosureText,
			PersonaVersion: persona,
			CreatedAt:      timestamppb.New(conv.StartedAt),
		},
	}), nil
}

// ListConversations returns the member's threads.
func (h *Chat) ListConversations(
	ctx context.Context, req *connect.Request[careerv1.ListConversationsRequest],
) (*connect.Response[careerv1.ListConversationsResponse], error) {
	me, err := h.requireMember(ctx, req)
	if err != nil {
		return nil, err
	}
	limit := 50
	if p := req.Msg.GetPage(); p != nil && p.GetPageSize() > 0 {
		limit = int(p.GetPageSize())
	}
	convs, err := h.users.ListConversations(ctx, me.ID, limit)
	if err != nil {
		h.log.Error("ListConversations failed", slog.String("error", err.Error()))
		return nil, connect.NewError(connect.CodeInternal, errors.New("could not list conversations"))
	}
	out := &careerv1.ListConversationsResponse{
		Conversations: make([]*careerv1.Conversation, 0, len(convs)),
	}
	for i := range convs {
		out.Conversations = append(out.Conversations, conversationToProto(convs[i]))
	}
	return connect.NewResponse(out), nil
}

// GetConversation returns one thread with its messages.
func (h *Chat) GetConversation(
	ctx context.Context, req *connect.Request[careerv1.GetConversationRequest],
) (*connect.Response[careerv1.GetConversationResponse], error) {
	me, err := h.requireMember(ctx, req)
	if err != nil {
		return nil, err
	}
	id, err := parseID(req.Msg.GetConversationId())
	if err != nil {
		return nil, err
	}
	conv, msgs, err := h.users.GetConversation(ctx, me.ID, id)
	if errors.Is(err, users.ErrConversationNotFound) {
		// Covers both "no such thread" and "not yours", deliberately:
		// telling a stranger which would let them map the id space.
		return nil, connect.NewError(connect.CodeNotFound, errors.New("no such conversation"))
	}
	if err != nil {
		h.log.Error("GetConversation failed", slog.String("error", err.Error()))
		return nil, connect.NewError(connect.CodeInternal, errors.New("could not load the conversation"))
	}
	out := &careerv1.GetConversationResponse{
		Conversation: conversationToProto(conv),
		Messages:     make([]*careerv1.Message, 0, len(msgs)),
	}
	for i := range msgs {
		out.Messages = append(out.Messages, messageToProto(&msgs[i]))
	}
	return connect.NewResponse(out), nil
}

// SendMessage stores the member's question and streams the reply.
//
// The stream is real even though the model is not streamed yet. `start`
// is sent the moment the question is stored, which is what lets the
// surface show an honest "reading his records" state against a wait
// that is 10 to 25 seconds on this hardware. The text then arrives as a
// single `delta`, which is a valid stream by the contract
// ("concatenate to build the reply") and becomes many deltas when the
// sidecar's streaming RPC lands, with no change here or in the client.
func (h *Chat) SendMessage(
	ctx context.Context,
	req *connect.Request[careerv1.SendMessageRequest],
	stream *connect.ServerStream[careerv1.SendMessageResponse],
) error {
	me, err := h.requireMember(ctx, req)
	if err != nil {
		return err
	}
	convID, err := parseID(req.Msg.GetConversationId())
	if err != nil {
		return err
	}
	question := strings.TrimSpace(req.Msg.GetText())
	if question == "" {
		return connect.NewError(connect.CodeInvalidArgument, errors.New("ask a question"))
	}

	// Ownership is established by loading the thread, which is also
	// where the history comes from. One query rather than a check
	// followed by a fetch.
	conv, history, err := h.users.GetConversation(ctx, me.ID, convID)
	if errors.Is(err, users.ErrConversationNotFound) {
		return connect.NewError(connect.CodeNotFound, errors.New("no such conversation"))
	}
	if err != nil {
		h.log.Error("SendMessage: load conversation failed", slog.String("error", err.Error()))
		return connect.NewError(connect.CodeInternal, errors.New("could not load the conversation"))
	}

	userMsgID, err := h.users.AppendMessage(ctx, users.ChatMessage{
		ConversationID: convID,
		Role:           "user",
		Text:           question,
	})
	if err != nil {
		h.log.Error("SendMessage: store question failed", slog.String("error", err.Error()))
		return connect.NewError(connect.CodeInternal, errors.New("could not store your question"))
	}

	// The first title is the first question, trimmed. Done here rather
	// than at creation because a thread is named by what it turned out
	// to be about, and at creation nobody knows.
	if conv.Title == "" {
		if err := h.users.SetConversationTitle(ctx, me.ID, convID, titleFrom(question)); err != nil {
			h.log.Warn("could not title conversation", slog.String("error", err.Error()))
		}
	}

	persona := prompts.AskRogerPersona.Fingerprint()
	if err := stream.Send(&careerv1.SendMessageResponse{
		Event: &careerv1.SendMessageResponse_Start_{Start: &careerv1.SendMessageResponse_Start{
			UserMessageId:  strconv.FormatInt(userMsgID, 10),
			PersonaVersion: persona,
		}},
	}); err != nil {
		return err
	}

	ans, err := h.answer.Answer(ctx, chat.Request{
		ConversationID: convID,
		UserID:         me.ID,
		Question:       question,
		History:        turnsFrom(history),
		// The private corpus informs the answer, and this was the
		// missing half of the feature.
		//
		// Retrieval defaults to public, and with it left there the
		// assistant could see 52 chunks across two published articles,
		// 50 of them from one, against 251 chunks of actual career
		// record sitting in corpus_only. Every answer therefore came
		// back drawn from one marketing article, including "does he
		// have capital project experience", which his own facts sheet
		// answers plainly.
		//
		// This is what FR-CHAT-20 ingests that material for. The
		// protections are already in place and tested, and none of them
		// is this flag: private passages reach the model unnumbered and
		// untitled so there is no marker it could cite, Citable() keeps
		// them out of the source list, and the persona is told they may
		// inform an answer and may never be quoted at length or named.
		// A document the owner does not want speaking for him at all is
		// excluded with chatbot_include, which is the control built for
		// that job.
		AllowPrivate: true,
	})
	if err != nil {
		h.log.Error("SendMessage: answer failed",
			slog.Int64("conversation", convID), slog.String("error", err.Error()))
		return connect.NewError(connect.CodeInternal, errors.New("could not answer just now"))
	}

	if err := stream.Send(&careerv1.SendMessageResponse{
		Event: &careerv1.SendMessageResponse_Delta_{
			Delta: &careerv1.SendMessageResponse_Delta{Text: ans.Text},
		},
	}); err != nil {
		return err
	}
	if len(ans.Citations) > 0 {
		cites := make([]*careerv1.Citation, 0, len(ans.Citations))
		for _, c := range ans.Citations {
			cites = append(cites, citationToProto(c))
		}
		if err := stream.Send(&careerv1.SendMessageResponse{
			Event: &careerv1.SendMessageResponse_Citations_{
				Citations: &careerv1.SendMessageResponse_Citations{Citations: cites},
			},
		}); err != nil {
			return err
		}
	}
	return stream.Send(&careerv1.SendMessageResponse{
		Event: &careerv1.SendMessageResponse_Done_{Done: &careerv1.SendMessageResponse_Done{
			Message: &careerv1.Message{
				Id:             strconv.FormatInt(ans.MessageID, 10),
				ConversationId: strconv.FormatInt(convID, 10),
				Role:           careerv1.Message_ROLE_ASSISTANT,
				Text:           ans.Text,
				Citations:      citationsToProto(ans.Citations),
				PersonaVersion: persona,
				ProposedAction: proposedActionToProto(ans.Intent),
				Flags: &careerv1.MessageFlags{
					OutOfScope: ans.OutOfScope,
					NoSupport:  ans.NoSupport,
					Degraded:   ans.Degraded,
					QaMatch:    ans.QAMatch,
				},
			},
		}},
	})
}

// DeleteConversation soft-deletes a thread.
func (h *Chat) DeleteConversation(
	ctx context.Context, req *connect.Request[careerv1.DeleteConversationRequest],
) (*connect.Response[careerv1.DeleteConversationResponse], error) {
	me, err := h.requireMember(ctx, req)
	if err != nil {
		return nil, err
	}
	id, err := parseID(req.Msg.GetConversationId())
	if err != nil {
		return nil, err
	}
	if err := h.users.DeleteConversation(ctx, me.ID, id); err != nil {
		if errors.Is(err, users.ErrConversationNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, errors.New("no such conversation"))
		}
		h.log.Error("DeleteConversation failed", slog.String("error", err.Error()))
		return nil, connect.NewError(connect.CodeInternal, errors.New("could not delete the conversation"))
	}
	return connect.NewResponse(&careerv1.DeleteConversationResponse{}), nil
}

// RateMessage records the member's rating (FR-CHAT-14).
//
// The rating is a queue signal for the owner's grading, not a grade
// itself. A member's thumbs-down says an answer disappointed someone,
// which is exactly the row worth opening first, and says nothing about
// whether it was grounded or in the right voice.
func (h *Chat) RateMessage(
	ctx context.Context, req *connect.Request[careerv1.RateMessageRequest],
) (*connect.Response[careerv1.RateMessageResponse], error) {
	me, err := h.requireMember(ctx, req)
	if err != nil {
		return nil, err
	}
	id, err := parseID(req.Msg.GetMessageId())
	if err != nil {
		return nil, err
	}
	rating := req.Msg.GetRating()
	if rating == careerv1.Rating_RATING_UNSPECIFIED {
		// Clearing a rating is not supported by the store yet: the
		// column distinguishes unrated from neutral and there is no
		// path back to unrated. Refusing beats silently recording a
		// thumbs-down.
		return nil, connect.NewError(connect.CodeUnimplemented,
			errors.New("a rating cannot be cleared once given"))
	}
	comment := strings.TrimSpace(req.Msg.GetComment())
	if err := h.users.RateMessage(ctx, me.ID, id, rating == careerv1.Rating_RATING_UP, comment); err != nil {
		if errors.Is(err, users.ErrConversationNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, errors.New("no such message"))
		}
		h.log.Error("RateMessage failed", slog.String("error", err.Error()))
		return nil, connect.NewError(connect.CodeInternal, errors.New("could not save the rating"))
	}
	return connect.NewResponse(&careerv1.RateMessageResponse{}), nil
}

// GetSuggestions returns starter questions (FR-CHAT-09).
//
// Drawn from the Q&A bank first, which is a deliberate connection
// rather than a convenience: a suggested question that is in the bank
// is answered instantly and in the owner's own words, so the first
// thing a visitor tries is the thing this hardware does best. Static
// fallbacks fill the list when the bank is thin, which today is always,
// because the bank is empty.
func (h *Chat) GetSuggestions(
	ctx context.Context, req *connect.Request[careerv1.GetSuggestionsRequest],
) (*connect.Response[careerv1.GetSuggestionsResponse], error) {
	if _, err := h.requireMember(ctx, req); err != nil {
		return nil, err
	}
	var qs []string
	entries, err := h.users.ListQAEntries(ctx, false)
	if err != nil {
		// Suggestions are a convenience; failing the call over them
		// would take the panel down for a garnish.
		h.log.Warn("suggestions: qa bank unavailable", slog.String("error", err.Error()))
	}
	for _, e := range entries {
		if len(qs) == 3 {
			break
		}
		qs = append(qs, e.Question)
	}
	for _, s := range fallbackSuggestions {
		if len(qs) >= 3 {
			break
		}
		qs = append(qs, s)
	}
	return connect.NewResponse(&careerv1.GetSuggestionsResponse{Questions: qs}), nil
}

// fallbackSuggestions are used until the bank has entries. Written as
// questions a hiring manager would actually ask, and answerable from
// the corpus as it stands.
var fallbackSuggestions = []string{
	"What kind of work has he done in industrial controls?",
	"Where has he applied AI to manufacturing problems?",
	"What does MDEMG do, and why did he build it?",
}

// proposedActionToProto carries a validated action to the surface.
// Nil stays nil: an answer that proposed nothing must not render a
// control.
func proposedActionToProto(in *chat.Intent) *careerv1.ProposedAction {
	if in == nil {
		return nil
	}
	return &careerv1.ProposedAction{Action: in.Action, Arg: in.Arg}
}

func conversationToProto(c users.Conversation) *careerv1.Conversation {
	return &careerv1.Conversation{
		Id:               strconv.FormatInt(c.ID, 10),
		Title:            c.Title,
		StartedAt:        timestamppb.New(c.StartedAt),
		LastMessageAt:    timestamppb.New(c.LastMessageAt),
		MessageCount:     int32(c.MessageCount),
		PersonaVersion:   c.PersonaVersion,
		ContextContentId: c.ContextContentID,
	}
}

func messageToProto(m *users.ChatMessage) *careerv1.Message {
	out := &careerv1.Message{
		Id:             strconv.FormatInt(m.ID, 10),
		ConversationId: strconv.FormatInt(m.ConversationID, 10),
		Role:           chatRoleToProto(m.Role),
		Text:           m.Text,
		CreatedAt:      timestamppb.New(m.CreatedAt),
		Citations:      citationsToProto(m.Citations),
		PersonaVersion: m.PersonaVersion,
		Flags: &careerv1.MessageFlags{
			OutOfScope: m.OutOfScope,
			NoSupport:  m.NoSupport,
			Degraded:   m.Degraded,
			QaMatch:    m.QAMatch,
		},
	}
	// Nil means unrated, which is not the same as neutral and has to
	// stay distinguishable all the way to the surface.
	if m.RatingUp != nil {
		if *m.RatingUp {
			out.Rating = careerv1.Rating_RATING_UP
		} else {
			out.Rating = careerv1.Rating_RATING_DOWN
		}
	}
	return out
}

func chatRoleToProto(role string) careerv1.Message_Role {
	switch role {
	case "user":
		return careerv1.Message_ROLE_USER
	case "assistant":
		return careerv1.Message_ROLE_ASSISTANT
	case "owner":
		return careerv1.Message_ROLE_OWNER
	default:
		return careerv1.Message_ROLE_UNSPECIFIED
	}
}

func citationsToProto(cs []users.ChatCitation) []*careerv1.Citation {
	if len(cs) == 0 {
		return nil
	}
	out := make([]*careerv1.Citation, 0, len(cs))
	for _, c := range cs {
		out = append(out, citationToProto(c))
	}
	return out
}

func citationToProto(c users.ChatCitation) *careerv1.Citation {
	return &careerv1.Citation{
		ChunkId:   strconv.FormatInt(c.ChunkID, 10),
		ContentId: c.ContentID,
		Title:     c.Title,
		Path:      c.Path,
		Heading:   c.Heading,
		Rank:      int32(c.Rank),
	}
}

// turnsFrom builds the history the prompt needs: complete exchanges,
// oldest first. A question with no answer yet (the one just stored) has
// no place in it, and neither does an answer with no question.
func turnsFrom(msgs []users.ChatMessage) []prompts.AskTurn {
	var turns []prompts.AskTurn
	var pending string
	for _, m := range msgs {
		switch m.Role {
		case "user":
			pending = m.Text
		case "assistant", "owner":
			if pending != "" {
				turns = append(turns, prompts.AskTurn{Question: pending, Answer: m.Text})
				pending = ""
			}
		}
	}
	return turns
}

// titleFrom derives a thread title from its first question. A title
// appears in a list and a question can be a paragraph.
func titleFrom(q string) string {
	q = strings.TrimSpace(strings.Join(strings.Fields(q), " "))
	const max = 70
	if len([]rune(q)) <= max {
		return q
	}
	r := []rune(q)[:max]
	// Cut at the last space so the title does not end mid-word.
	if i := strings.LastIndex(string(r), " "); i > 20 {
		return strings.TrimRight(string(r)[:i], " ,.;:") + "..."
	}
	return string(r) + "..."
}

func parseID(s string) (int64, error) {
	id, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
	if err != nil || id <= 0 {
		return 0, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("id must be numeric"))
	}
	return id, nil
}

// requireChatAdmin resolves an admin session or fails the RPC.
//
// Separate from requireMember because these two RPCs read the database
// and a member must not. The proto declares AUTH_LEVEL_ADMIN and
// enforcement lives here until the auth interceptor lands, same as
// everywhere else on this service.
func (h *Chat) requireChatAdmin(ctx context.Context, req connect.AnyRequest) (*users.User, error) {
	me, err := h.requireMember(ctx, req)
	if err != nil {
		return nil, err
	}
	if me.Role != users.RoleAdmin {
		return nil, connect.NewError(connect.CodePermissionDenied, errors.New("admin role required"))
	}
	return me, nil
}

// ListAdminQueries returns the dropdown contents.
func (h *Chat) ListAdminQueries(
	ctx context.Context, req *connect.Request[careerv1.ListAdminQueriesRequest],
) (*connect.Response[careerv1.ListAdminQueriesResponse], error) {
	if _, err := h.requireChatAdmin(ctx, req); err != nil {
		return nil, err
	}
	list := users.ListAdminQueries()
	out := &careerv1.ListAdminQueriesResponse{Queries: make([]*careerv1.AdminQuery, 0, len(list))}
	for _, q := range list {
		out.Queries = append(out.Queries, &careerv1.AdminQuery{
			Id: q.ID, Label: q.Label, Detail: q.Detail,
		})
	}
	return connect.NewResponse(out), nil
}

// RunAdminQuery runs one named query.
//
// No model is involved. The admin picked the query and the database
// answers it, so the number is exact rather than something generated,
// and it arrives in milliseconds rather than the twenty to thirty
// seconds an answer takes on this hardware.
func (h *Chat) RunAdminQuery(
	ctx context.Context, req *connect.Request[careerv1.RunAdminQueryRequest],
) (*connect.Response[careerv1.RunAdminQueryResponse], error) {
	me, err := h.requireChatAdmin(ctx, req)
	if err != nil {
		return nil, err
	}
	q, res, err := h.users.RunAdminQuery(ctx, strings.TrimSpace(req.Msg.GetId()))
	if errors.Is(err, users.ErrNoSuchAdminQuery) {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("no such query"))
	}
	if err != nil {
		h.log.Error("RunAdminQuery failed",
			slog.String("query", req.Msg.GetId()), slog.String("error", err.Error()))
		return nil, connect.NewError(connect.CodeInternal, errors.New("the query did not run"))
	}
	h.log.Info("admin query run",
		slog.Int64("admin", me.ID), slog.String("query", q.ID))
	out := &careerv1.RunAdminQueryResponse{
		Query:   &careerv1.AdminQuery{Id: q.ID, Label: q.Label, Detail: q.Detail},
		Columns: res.Columns,
		RanAt:   timestamppb.Now(),
	}
	for _, row := range res.Rows {
		out.Rows = append(out.Rows, &careerv1.AdminQueryRow{Cells: row})
	}
	return connect.NewResponse(out), nil
}
