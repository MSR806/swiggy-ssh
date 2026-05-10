package httpserver

import (
	"errors"
	"html/template"
	"net/http"
	"strings"

	"swiggy-ssh/internal/auth"
)

var tmplLoginForm = template.Must(template.New("login_form").Parse(`<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>swiggy.dev · Complete Login</title>
  <style>
    body { font-family: system-ui, sans-serif; max-width: 480px; margin: 80px auto; padding: 0 16px; color: #1a1a1a; }
    h1 { font-size: 1.4rem; margin-bottom: 8px; }
    p { color: #555; margin-bottom: 24px; }
    label { display: block; font-weight: 600; margin-bottom: 6px; }
    input[type=text] { width: 100%; box-sizing: border-box; padding: 10px 12px; font-size: 1.1rem; border: 1px solid #ccc; border-radius: 6px; letter-spacing: 0.1em; }
    button { margin-top: 16px; width: 100%; padding: 10px; font-size: 1rem; background: #e85b1a; color: #fff; border: none; border-radius: 6px; cursor: pointer; }
    button:hover { background: #c94d15; }
  </style>
</head>
<body>
  <h1>swiggy.dev · Complete Login</h1>
  <p>Enter the code shown in your terminal to finish signing in.</p>
  <form method="POST" action="/login">
    <label for="code">Login code</label>
    <input type="text" id="code" name="code" placeholder="XXXX-XXXX" autocomplete="off" autofocus>
    <button type="submit">Complete Login</button>
  </form>
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
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = tmplLoginForm.Execute(w, nil)
}

func (s *Server) handleLoginPost(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		s.renderError(w, "Something went wrong. Please try again.")
		return
	}

	code := strings.TrimSpace(r.FormValue("code"))
	if code == "" {
		s.renderError(w, "Please enter a login code.")
		return
	}

	err := s.svc.CompleteLoginCode(r.Context(), code)
	if err == nil {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_ = tmplLoginSuccess.Execute(w, nil)
		return
	}

	switch {
	case errors.Is(err, auth.ErrLoginCodeNotFound):
		s.renderError(w, "Login code not found or expired. Please start a new SSH session.")
	case errors.Is(err, auth.ErrLoginCodeAlreadyUsed):
		s.renderError(w, "Login code has already been used. Please start a new SSH session.")
	default:
		s.logger.WarnContext(r.Context(), "complete login code unexpected error", "error", err)
		s.renderError(w, "Something went wrong. Please try again.")
	}
}

func (s *Server) renderError(w http.ResponseWriter, msg string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = tmplLoginError.Execute(w, struct{ Message string }{Message: msg})
}
