package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Decision values embedded in a signed one-click URL.
type Decision string

const (
	DecisionApprove Decision = "approve"
	DecisionDecline Decision = "decline"
)

var ErrDecisionTokenInvalid = errors.New("decision token invalid or expired")

// DecisionToken is a compact HMAC-signed token used in the admin approval
// email links. Layout, after base64url encoding:
//
//	v1.<user_id>.<decision>.<expires_at_unix>.<hmac_b64>
//
// where the HMAC is SHA-256 over "v1.<user_id>.<decision>.<expires_at_unix>"
// using a server-side secret. State change is single-use because the API
// only accepts the token when the user is still in `pending_approval`; a
// second click after the decision has been recorded fails cleanly.
type DecisionToken struct {
	UserID    int64
	Decision  Decision
	ExpiresAt time.Time
}

const decisionTokenVersion = "v1"

func SignDecision(secret []byte, userID int64, decision Decision, ttl time.Duration) (string, error) {
	if len(secret) < 32 {
		return "", errors.New("decision-token secret must be at least 32 bytes")
	}
	expires := time.Now().UTC().Add(ttl).Unix()
	payload := fmt.Sprintf("%s.%d.%s.%d", decisionTokenVersion, userID, decision, expires)

	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(payload))
	sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))

	return payload + "." + sig, nil
}

func VerifyDecision(secret []byte, token string) (*DecisionToken, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 5 || parts[0] != decisionTokenVersion {
		return nil, ErrDecisionTokenInvalid
	}
	payload := strings.Join(parts[:4], ".")

	wantSig, err := base64.RawURLEncoding.DecodeString(parts[4])
	if err != nil {
		return nil, ErrDecisionTokenInvalid
	}
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(payload))
	if !hmac.Equal(wantSig, mac.Sum(nil)) {
		return nil, ErrDecisionTokenInvalid
	}

	userID, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return nil, ErrDecisionTokenInvalid
	}
	var decision Decision
	switch Decision(parts[2]) {
	case DecisionApprove, DecisionDecline:
		decision = Decision(parts[2])
	default:
		return nil, ErrDecisionTokenInvalid
	}
	expUnix, err := strconv.ParseInt(parts[3], 10, 64)
	if err != nil {
		return nil, ErrDecisionTokenInvalid
	}
	expiresAt := time.Unix(expUnix, 0).UTC()
	if time.Now().UTC().After(expiresAt) {
		return nil, ErrDecisionTokenInvalid
	}

	return &DecisionToken{
		UserID:    userID,
		Decision:  decision,
		ExpiresAt: expiresAt,
	}, nil
}
