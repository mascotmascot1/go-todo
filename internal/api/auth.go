package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/mascotmascot1/go-todo/internal/config"

	"github.com/golang-jwt/jwt/v5"
)

type claims struct {
	jwt.RegisteredClaims
	PassHash string `json:"pass_hash"`
}

type authRequest struct {
	Password string `json:"password"`
}

// signInHandler authenticates the user and returns a JWT token
// that can be used for further requests.
// It expects a JSON body with the password field.
// If the password is incorrect, it will return an error with 401 status code.
// If there is an error while creating the token, it will return an error with 500 status code.
func (h *Handlers) signInHandler(w http.ResponseWriter, r *http.Request) {
	caller := "signinHandler"

	if h.auth.Password == "" {
		h.logger.Printf("%s: authentication configuration is invalid: empty password\n", caller)
		h.writeJSON(w, response{Error: "server configuration error"}, http.StatusInternalServerError)
		return
	}
	if len(h.auth.SecretKey) == 0 {
		h.logger.Printf("%s: authentication configuration is invalid: empty secret key\n", caller)
		h.writeJSON(w, response{Error: "server configuration error"}, http.StatusInternalServerError)
		return
	}

	content, err := io.ReadAll(r.Body)
	if err != nil {
		h.logger.Printf("%s: failed to read body: %v\n", caller, err)
		h.writeJSON(w, response{Error: "failed to read request body"}, http.StatusBadRequest)
		return
	}

	var req authRequest
	if err := json.Unmarshal(content, &req); err != nil {
		h.logger.Printf("%s: json marshal error: %v\n", caller, err)
		h.writeJSON(w, response{Error: fmt.Sprintf("JSON deserialization failed: %v", err)}, http.StatusBadRequest)
		return
	}

	if h.auth.Password != req.Password {
		h.logger.Printf("%s: incorrect password provided\n", caller)
		h.writeJSON(w, response{Error: "incorrect password"}, http.StatusUnauthorized)
		return
	}

	newToken, err := createToken(h.auth)
	if err != nil {
		h.logger.Printf("%s: %v\n", caller, err)
		h.writeJSON(w, response{Error: "failed to create token"}, http.StatusInternalServerError)
		return
	}
	h.writeJSON(w, response{Token: newToken}, http.StatusOK)
}

// withAuth returns a middleware that checks if the JWT token is provided in the cookies.
// If the token is not provided, it will return an error with 401 status code.
// If the token is invalid, it will return an error with 401 status code.
// If the token is valid, it will call the next handler in the chain.
func (h *Handlers) withAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		caller := "auth middleware"

		if h.auth.Password != "" {
			cookie, err := r.Cookie("token")
			if err != nil {
				h.logger.Printf("%s: failed to get token cookie: %v\n", caller, err)
				h.writeJSON(w, "authentication required", http.StatusUnauthorized)
				return
			}
			if len(h.auth.SecretKey) == 0 {
				h.logger.Printf("%s: authentication configuration is invalid: empty secret key\n", caller)
				h.writeJSON(w, "server configuration error", http.StatusInternalServerError)
				return
			}

			tokenString := cookie.Value
			if err := validateToken(tokenString, h.auth.PasswordHash, h.auth.SecretKey); err != nil {
				h.logger.Printf("%s: %v\n", caller, err)
				h.writeJSON(w, "invalid JWT token", http.StatusUnauthorized)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

// createToken creates a JWT token that can be used for authentication.
// It takes authentication configuration as an argument and returns a signed token.
// If there is an error while creating the token, it will return an error with a description.
// The token will contain the password hash from the authentication configuration and expire after the TokenTTL has passed.
func createToken(auth *config.Auth) (string, error) {
	c := claims{
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(auth.TokenTTL)),

			IssuedAt: jwt.NewNumericDate(time.Now()),
		},
		PassHash: auth.PasswordHash,
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, &c)
	signedToken, err := token.SignedString(auth.SecretKey)
	if err != nil {
		return "", fmt.Errorf("failed to sign the jwt token: %v\n", err)
	}
	return signedToken, nil
}

// validateToken validates the given JWT token.
// It takes the token string, password hash from the authentication configuration and secret key as arguments.
// If the token is invalid, it will return an error with a description.
// If the token is valid, it will return nil.
func validateToken(tokenString, passwordHash string, secretKey []byte) error {
	var c claims

	parsedToken, err := jwt.ParseWithClaims(tokenString, &c, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method %v\n", t.Header["alg"])
		}
		return secretKey, nil
	})
	if err != nil {
		return fmt.Errorf("failed to parse token: %w", err)
	}

	if !parsedToken.Valid {
		return fmt.Errorf("token is invalid")
	}

	if c.PassHash != passwordHash {
		return fmt.Errorf("invalid password hash")
	}
	return nil
}
