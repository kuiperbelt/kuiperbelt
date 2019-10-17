package kuiperbelt

import (
	"bytes"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/lestrrat-go/jwx/jwa"
	"github.com/lestrrat-go/jwx/jws"
	"github.com/lestrrat-go/jwx/jwt"
	"github.com/pkg/errors"
)

const (
	jwtEndpointKey = "edp"
)

type uuidGen interface {
	NewRandom() (uuid.UUID, error)
}

type uuidGenWrapper struct{}

func (w *uuidGenWrapper) NewRandom() (uuid.UUID, error) {
	return uuid.NewRandom()
}

type sessionKeyGen struct {
	alg      jwa.SignatureAlgorithm
	privKey  *rsa.PrivateKey
	uuidGen  uuidGen
	endpoint string
}

func NewSessionKeyGen(endpoint string, key []byte) (*sessionKeyGen, error) {
	g := &sessionKeyGen{
		alg:      jwa.NoSignature,
		privKey:  nil,
		uuidGen:  &uuidGenWrapper{},
		endpoint: endpoint,
	}

	if key != nil {
		privKey, err := x509.ParsePKCS1PrivateKey(key)
		if err != nil {
			return nil, errors.Wrap(err, "cannot read sign key")
		}
		g.alg = jwa.RS256
		g.privKey = privKey
	}

	return g, nil
}

type jwtKey interface {
	String() string
}

func (g *sessionKeyGen) Generate() (jwtKey, error) {
	u, err := g.uuidGen.NewRandom()
	if err != nil {
		return nil, errors.Wrap(err, "failed to generate uuid")
	}

	t := jwt.New()
	t.Set(jwt.IssuerKey, "kuiperbelt")
	t.Set(jwt.SubjectKey, u.String())
	t.Set(jwt.IssuedAtKey, time.Now().Unix())
	t.Set(jwtEndpointKey, g.endpoint)
	payload, err := g.sign(t)
	if err != nil {
		return nil, errors.Wrap(err, "failed to sign JWT key")
	}
	buf := bytes.NewBuffer(payload)

	return buf, nil
}

func (g *sessionKeyGen) sign(t *jwt.Token) ([]byte, error) {
	if g.alg == jwa.NoSignature {
		buf := bytes.NewBuffer(make([]byte, 0, 512))
		var h jws.StandardHeaders
		h.Set(`alg`, jwa.NoSignature)
		h.Set(`typ`, `JWT`)
		hj, err := json.Marshal(h)
		if err != nil {
			return nil, errors.Wrap(err, "faild to encode header")
		}
		hb := base64.RawURLEncoding.EncodeToString(hj)
		buf.WriteString(hb)
		buf.WriteString(".")

		cj, err := json.Marshal(t)
		if err != nil {
			return nil, errors.Wrap(err, "faild to encode claims")
		}
		cb := base64.RawURLEncoding.EncodeToString(cj)
		buf.WriteString(cb)
		buf.WriteString(".")

		return buf.Bytes(), nil
	}

	return t.Sign(g.alg, g.privKey)
}

type KeyToken interface {
	Key() string
}

type keyToken struct {
	token *jwt.Token
}

func (t *keyToken) Key() string {
	return t.token.Subject()
}

func (g *sessionKeyGen) FromString(ts string) (KeyToken, error) {
	var t *jwt.Token
	if g.alg == jwa.NoSignature {
		var err error
		t, err = jwt.ParseString(ts)
		if err != nil {
			return nil, errors.Wrapf(err, "cannot parse jwt")
		}
	} else {
		var err error
		t, err = jwt.ParseString(ts, jwt.WithVerify(g.alg, g.privKey))
		if err != nil {
			return nil, errors.Wrapf(err, "failed to parse or verify jwt")
		}
	}

	return &keyToken{token: t}, nil
}
