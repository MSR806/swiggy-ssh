package http

import (
	"errors"
	"html/template"
	"net/http"
	"net/url"

	"swiggy-ssh/internal/application/auth"
)

var tmplLoginInfo = template.Must(template.New("login_info").Parse(`<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>swiggy.dev · Complete Login</title>
  <style>
    body { font-family: system-ui, sans-serif; max-width: 480px; margin: 80px auto; padding: 0 16px; color: #1a1a1a; }
    h1 { font-size: 1.4rem; margin-bottom: 8px; }
    p { color: #555; margin-bottom: 24px; }
    code { background: #f6f6f6; padding: 2px 6px; border-radius: 4px; }
  </style>
</head>
<body>
  <h1>swiggy.dev · Browser Login</h1>
  <p>Start login from the direct URL shown in your SSH session.</p>
  <p>If your link points here, reconnect and choose Instamart again to create a fresh one-time auth attempt.</p>
</body>
</html>
`))

var tmplLoginSuccess = template.Must(template.New("login_success").Parse(`<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>swiggy.dev · Login Complete</title>
  <style>
    body { font-family: system-ui, sans-serif; max-width: 480px; margin: 80px auto; padding: 0 16px; color: #1a1a1a; text-align: center; }
    .icon { font-size: 3rem; }
    h1 { font-size: 1.4rem; margin: 16px 0 8px; }
    p { color: #555; }
  </style>
</head>
<body>
  <div class="icon">✓</div>
  <h1>Login complete!</h1>
  <p>You may close this tab. Return to your terminal.</p>
</body>
</html>
`))

var tmplLoginError = template.Must(template.New("login_error").Parse(`<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>swiggy.dev · Login Error</title>
  <style>
    body { font-family: system-ui, sans-serif; max-width: 480px; margin: 80px auto; padding: 0 16px; color: #1a1a1a; }
    .error-box { background: #fff3f3; border: 1px solid #f5c2c2; border-radius: 6px; padding: 16px; color: #b00; margin-bottom: 24px; }
    a { color: #e85b1a; }
  </style>
</head>
<body>
  <h1>Login failed</h1>
  <div class="error-box">{{.Message}}</div>
  <p><a href="/login">← Try again</a></p>
</body>
</html>
`))

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}

func (s *Server) handleLoginGet(w http.ResponseWriter, r *http.Request) {
	attempt := r.URL.Query().Get("attempt")
	if attempt != "" {
		http.Redirect(w, r, "/auth/start?attempt="+url.QueryEscape(attempt), http.StatusFound)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = tmplLoginInfo.Execute(w, nil)
}

func (s *Server) handleLoginPost(w http.ResponseWriter, r *http.Request) {
	attempt := r.URL.Query().Get("attempt")
	if attempt == "" && r.ParseForm() == nil {
		attempt = r.FormValue("attempt")
	}
	if attempt == "" {
		s.renderError(w, "Login codes are no longer used. Open the direct login URL shown in your terminal.")
		return
	}
	http.Redirect(w, r, "/auth/start?attempt="+url.QueryEscape(attempt), http.StatusFound)
}

func (s *Server) handleAuthStart(w http.ResponseWriter, r *http.Request) {
	attempt := r.URL.Query().Get("attempt")
	if attempt == "" {
		s.renderError(w, "Missing auth attempt. Please start a new SSH session.")
		return
	}

	if s.provider != "mock" {
		started, err := s.startProviderAuth(r, attempt)
		if err == nil {
			http.Redirect(w, r, started.RedirectURL, http.StatusFound)
			return
		}
		s.renderAuthError(w, r, err)
		return
	}

	// Mock/dev mode completes the one-time attempt directly and lets the terminal continue.
	err := s.completeAttempt(r, attempt)
	if err == nil {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_ = tmplLoginSuccess.Execute(w, nil)
		return
	}

	switch {
	case errors.Is(err, auth.ErrAuthAttemptNotFound):
		s.renderError(w, "Auth attempt not found or expired. Please start a new SSH session.")
	case errors.Is(err, auth.ErrAuthAttemptAlreadyUsed):
		s.renderError(w, "Auth attempt has already been used. Please start a new SSH session.")
	default:
		s.logger.WarnContext(r.Context(), "complete auth attempt unexpected error", "error", err)
		s.renderError(w, "Something went wrong. Please try again.")
	}
}

func (s *Server) handleAuthCallback(w http.ResponseWriter, r *http.Request) {
	attempt := r.URL.Query().Get("state")
	if attempt == "" && s.provider == "mock" {
		attempt = r.URL.Query().Get("attempt")
	}
	if attempt == "" {
		s.renderError(w, "Missing auth state. Please start a new SSH session.")
		return
	}
	code := r.URL.Query().Get("code")
	if s.provider != "mock" && code == "" {
		s.renderError(w, "Missing Swiggy authorization code. Please start a new SSH session.")
		return
	}
	record, err := s.svc.GetAuthAttempt(r.Context(), attempt)
	if err != nil {
		s.renderAuthError(w, r, err)
		return
	}
	if record.Status != auth.AuthAttemptStatusPending {
		s.renderAuthError(w, r, auth.ErrAuthAttemptAlreadyUsed)
		return
	}
	if s.provider != "mock" {
		if s.auth == nil {
			s.renderAuthError(w, r, auth.ErrBrowserAuthProviderUnavailable)
			return
		}
		_, err = s.auth.ExecuteCallback(r.Context(), auth.BrowserAuthCallbackInput{
			State:       attempt,
			Code:        code,
			CallbackURL: s.publicBaseURL + "/auth/callback",
		})
		if err != nil {
			s.renderAuthError(w, r, err)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_ = tmplLoginSuccess.Execute(w, nil)
		return
	}
	err = s.completeAttempt(r, attempt)
	if err != nil {
		s.renderAuthError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = tmplLoginSuccess.Execute(w, nil)
}

func (s *Server) startProviderAuth(r *http.Request, attempt string) (auth.StartBrowserAuthOutput, error) {
	if s.startAuth == nil {
		return auth.StartBrowserAuthOutput{}, auth.ErrBrowserAuthProviderUnavailable
	}
	return s.startAuth.Execute(r.Context(), auth.StartBrowserAuthInput{
		AttemptToken: attempt,
		CallbackURL:  s.publicBaseURL + "/auth/callback",
	})
}

func (s *Server) completeAttempt(r *http.Request, attempt string) error {
	if s.auth != nil {
		_, err := s.auth.Execute(r.Context(), attempt)
		return err
	}
	return s.svc.CompleteAuthAttempt(r.Context(), attempt)
}

func (s *Server) renderAuthError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, auth.ErrAuthAttemptNotFound):
		s.renderError(w, "Auth attempt not found or expired. Please start a new SSH session.")
	case errors.Is(err, auth.ErrAuthAttemptAlreadyUsed):
		s.renderError(w, "Auth attempt has already been used. Please start a new SSH session.")
	case errors.Is(err, auth.ErrBrowserAuthProviderUnavailable):
		s.renderError(w, "Swiggy browser login is not configured yet. Please use mock provider for local development.")
	case errors.Is(err, auth.ErrBrowserAuthProviderCallback):
		s.renderError(w, "Swiggy login callback could not be completed. Please start a new SSH session.")
	case errors.Is(err, auth.ErrOAuthAccountUserRequired):
		s.renderError(w, "This auth link is not attached to a durable SSH identity. Reconnect with a known SSH key and try again.")
	default:
		s.logger.WarnContext(r.Context(), "browser auth unexpected error", "error", err)
		s.renderError(w, "Something went wrong. Please try again.")
	}
}

func (s *Server) renderError(w http.ResponseWriter, msg string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = tmplLoginError.Execute(w, struct{ Message string }{Message: msg})
}
