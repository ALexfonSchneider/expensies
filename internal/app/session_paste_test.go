package app

import (
	"encoding/base64"
	"testing"

	"github.com/ALexfonSchneider/expenses/internal/domain"
)

func fakeRefreshToken(t *testing.T, deviceID, phone string) string {
	t.Helper()
	sub := `{"type":"REFRESH_TOKEN","refreshContext":{"authType":"SMS","login":"` + phone + `","id":1,"deviceId":"` + deviceID + `","phone":"` + phone + `"},"expiration":null}`
	payload := base64.RawURLEncoding.EncodeToString([]byte(`{"sub":` + quote(sub) + `}`))
	return "eyJhbGciOiJIUzUxMiJ9." + payload + ".sig"
}

func quote(s string) string {
	b := []byte{'"'}
	for _, c := range s {
		switch c {
		case '"':
			b = append(b, '\\', '"')
		case '\\':
			b = append(b, '\\', '\\')
		default:
			b = append(b, string(c)...)
		}
	}
	return string(append(b, '"'))
}

func TestExpandPastedSession_FromJWT(t *testing.T) {
	token := fakeRefreshToken(t, "DEV123", "79990000000")
	out := expandPastedSession(domain.ReceiptSession{RefreshToken: token})
	if out.DeviceID != "DEV123" || out.Phone != "79990000000" || out.RefreshToken != token {
		t.Errorf("expanded = %+v", out)
	}
}

func TestExpandPastedSession_FromAuthResponse(t *testing.T) {
	token := fakeRefreshToken(t, "DEV123", "79990000000")
	body := `{"refreshToken":` + quote(token) + `,"refreshTokenExpiresIn":null,"token":"access.jwt.x","tokenExpireIn":"2026-09-12T16:02:48.531Z","profile":{"user":{"taxpayerPerson":{"phone":"79990000000"},"authType":"SMS"}}}`
	out := expandPastedSession(domain.ReceiptSession{RefreshToken: body, DeviceID: "explicit"})
	if out.RefreshToken != token || out.AccessToken != "access.jwt.x" || out.Phone != "79990000000" {
		t.Errorf("expanded = %+v", out)
	}
	if out.DeviceID != "explicit" {
		t.Errorf("explicit device id must win, got %q", out.DeviceID)
	}
}

func TestExpandPastedSession_PlainTokenUntouched(t *testing.T) {
	out := expandPastedSession(domain.ReceiptSession{RefreshToken: "opaque", Phone: "1"})
	if out.RefreshToken != "opaque" || out.Phone != "1" || out.DeviceID != "" {
		t.Errorf("expanded = %+v", out)
	}
}
