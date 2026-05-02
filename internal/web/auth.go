package web

import (
	"net/http"

	"github.com/gorilla/sessions"
	"golang.org/x/crypto/bcrypt"
)

const (
	sessionName    = "home-router"
	sessionKeyAuth = "authenticated"
)

type Auth struct {
	store        sessions.Store
	passwordHash string
}

func NewAuth(secret, passwordHash string) *Auth {
	store := sessions.NewCookieStore([]byte(secret))
	store.Options = &sessions.Options{
		Path:     "/",
		MaxAge:   86400,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
	}
	return &Auth{
		store:        store,
		passwordHash: passwordHash,
	}
}

func (a *Auth) VerifyPassword(password string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(a.passwordHash), []byte(password))
	return err == nil
}

func (a *Auth) Login(w http.ResponseWriter, r *http.Request) error {
	sess, err := a.store.Get(r, sessionName)
	if err != nil {
		sess, _ = a.store.New(r, sessionName)
	}
	sess.Values[sessionKeyAuth] = true
	return sess.Save(r, w)
}

func (a *Auth) Logout(w http.ResponseWriter, r *http.Request) error {
	sess, err := a.store.Get(r, sessionName)
	if err != nil {
		return err
	}
	sess.Values[sessionKeyAuth] = false
	sess.Options.MaxAge = -1
	return sess.Save(r, w)
}

func (a *Auth) IsAuthenticated(r *http.Request) bool {
	sess, err := a.store.Get(r, sessionName)
	if err != nil {
		return false
	}
	auth, ok := sess.Values[sessionKeyAuth].(bool)
	return ok && auth
}

func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}
