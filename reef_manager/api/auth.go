package api

import (
	"crypto/subtle"
	"fmt"
	"net/http"

	"github.com/davecgh/go-spew/spew"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const SessionName = "reef"

const tokenQuery = "token"
const sessionIDField = "session_id"
const isAdminField = "is_admin"

type AuthHandlerT struct {
	AdminToken string
}

var AuthHandler AuthHandlerT

type AuthRequest struct {
	Token *string `json:"token"`
}

type AuthResponse struct {
	Id      string `json:"id"`
	IsAdmin bool   `json:"isAdmin"`
}

func InitAuthHandler(adminToken string) {
	AuthHandler = AuthHandlerT{
		AdminToken: adminToken,
	}
}

func (h *AuthHandlerT) HandleAuth(ctx *gin.Context) {
	var req AuthRequest

	if err := ctx.ShouldBindJSON(&req); err != nil {
		badRequest(ctx, fmt.Sprintf("invalid auth request: %s", err.Error()))
		return
	}

	res := h.processAuth(ctx, req.Token)
	if res == nil {
		return
	}

	ctx.JSON(
		http.StatusCreated,
		*res,
	)
}

func (h *AuthHandlerT) processAuth(ctx *gin.Context, token *string) *AuthResponse {
	session := sessions.Default(ctx)

	isAdmin := false
	if token != nil {
		if subtle.ConstantTimeCompare([]byte(*token), []byte(h.AdminToken)) != 1 {
			respondErr(ctx, "login failed", "invalid access token", http.StatusUnauthorized)
			return nil
		}
		isAdmin = true
	}

	old := extractExistingSession(session)

	if old != nil {
		log.Debugf("used old session ID `%s` admin=%v in auth handler", old.Id, old.IsAdmin)
		return old
	}

	uid, err := uuid.NewV7()
	if err != nil {
		respondErr(ctx, "login failed", "internal error", http.StatusInternalServerError)
		return nil
	}

	id := uid.String()

	session.Set(sessionIDField, id)
	session.Set(isAdminField, isAdmin)

	if err := session.Save(); err != nil {
		respondErr(ctx, "login failed", "internal error", http.StatusInternalServerError)
		return nil
	}

	return &AuthResponse{
		Id:      id,
		IsAdmin: isAdmin,
	}
}

func extractExistingSession(session sessions.Session) *AuthResponse {
	id := session.Get(sessionIDField)
	isAdmin := session.Get(isAdminField)

	spew.Dump(id, isAdmin)

	if id == nil || isAdmin == nil {
		return nil
	}

	return &AuthResponse{
		Id:      id.(string),
		IsAdmin: isAdmin.(bool),
	}
}

//
// Auth Middleware.
//

func (h *AuthHandlerT) ReefAuth() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		log.Debugf("Auth middleware for `%s` `%s`", ctx.Request.URL, ctx.RemoteIP())

		session := sessions.Default(ctx)
		s := extractExistingSession(session)

		if s == nil {
			var token *string

			tokenStr, found := ctx.GetQuery(tokenQuery)
			if found {
				token = &tokenStr
			}

			if h.processAuth(ctx, token) == nil {
				ctx.AbortWithStatus(http.StatusUnauthorized)
			}
		} else {
			ctx.Set(SessionName, *s)
		}

		ctx.Next()
	}
}
