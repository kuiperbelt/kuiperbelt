package kuiperbelt

import "github.com/lestrrat-go/jwx/jwt"
import "testing"

func TestNewJWTKey(t *testing.T) {
	g, err := NewSessionKeyGen("localhost:9180", nil)
	if err != nil {
		t.Errorf("unexpected error: %s", err)
	}

	k, err := g.Generate()
	if err != nil {
		t.Errorf("unexpected error: %s", err)
	}
	t.Logf("jwt=%s", k)

	token, err := jwt.ParseString(k.String())
	if err != nil {
		t.Errorf("unexpected error: %s", err)
	}
	if token.Issuer() != "kuiperbelt" {
		t.Errorf("unexpected issuer of token: %s", token.Issuer())
	}
	buf, err := token.MarshalJSON()
	if err != nil {
		t.Errorf("unexpected error: %s", err)
	}
	t.Logf("payload=%s", string(buf))

	vt, err := g.FromString(k.String())
	if err != nil {
		t.Errorf("unexpected error: %s", err)
	}

	if vt.Key() != token.Subject() {
		t.Errorf(
			"not match subject(key): %s vs %s",
			vt.Key(), token.Subject(),
		)
	}
}
