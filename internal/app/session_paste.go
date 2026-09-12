package app

import (
	"encoding/base64"
	"encoding/json"
	"strings"

	"github.com/ALexfonSchneider/expenses/internal/domain"
)

// authResponse is the shape of the archive's auth/token reply as seen in a
// browser's DevTools; pasting it whole is easier than copying fields.
type authResponse struct {
	RefreshToken string `json:"refreshToken"`
	Token        string `json:"token"`
	Profile      *struct {
		User *struct {
			TaxpayerPerson *struct {
				Phone string `json:"phone"`
			} `json:"taxpayerPerson"`
		} `json:"user"`
	} `json:"profile"`
}

// refreshClaims is the subject of the archive's refresh JWT: a JSON string
// that carries the device the token was issued to.
type refreshClaims struct {
	Sub string `json:"sub"`
}

type refreshSubject struct {
	RefreshContext struct {
		DeviceID string `json:"deviceId"`
		Phone    string `json:"phone"`
	} `json:"refreshContext"`
}

// expandPastedSession fills missing session fields from a pasted auth
// response and from the refresh token itself. Explicit values win.
func expandPastedSession(in domain.ReceiptSession) domain.ReceiptSession {
	raw := strings.TrimSpace(in.RefreshToken)
	if strings.HasPrefix(raw, "{") {
		var resp authResponse
		if err := json.Unmarshal([]byte(raw), &resp); err == nil && resp.RefreshToken != "" {
			in.RefreshToken = resp.RefreshToken
			if in.AccessToken == "" {
				in.AccessToken = resp.Token
			}
			if in.Phone == "" && resp.Profile != nil && resp.Profile.User != nil && resp.Profile.User.TaxpayerPerson != nil {
				in.Phone = resp.Profile.User.TaxpayerPerson.Phone
			}
		}
	}
	if subject, ok := decodeRefreshSubject(strings.TrimSpace(in.RefreshToken)); ok {
		if strings.TrimSpace(in.DeviceID) == "" {
			in.DeviceID = subject.RefreshContext.DeviceID
		}
		if strings.TrimSpace(in.Phone) == "" {
			in.Phone = subject.RefreshContext.Phone
		}
	}
	return in
}

// decodeRefreshSubject reads the unverified payload of the refresh JWT.
// The token is only parsed for convenience; the archive is the one that
// validates it.
func decodeRefreshSubject(token string) (refreshSubject, bool) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return refreshSubject{}, false
	}
	payload, err := base64.RawURLEncoding.DecodeString(strings.TrimRight(parts[1], "="))
	if err != nil {
		return refreshSubject{}, false
	}
	var claims refreshClaims
	if err := json.Unmarshal(payload, &claims); err != nil || claims.Sub == "" {
		return refreshSubject{}, false
	}
	var subject refreshSubject
	if err := json.Unmarshal([]byte(claims.Sub), &subject); err != nil {
		return refreshSubject{}, false
	}
	return subject, subject.RefreshContext.DeviceID != "" || subject.RefreshContext.Phone != ""
}
