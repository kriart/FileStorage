package web

import (
	"net/http"

	"file-storage-server/internal/middleware"
	"file-storage-server/internal/user"
)

func (h *Handler) LoginPage(w http.ResponseWriter, r *http.Request) {
	h.render(w, r, "login.html", PageData{Title: "Login"})
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		h.render(w, r, "login.html", PageData{Title: "Login", Error: "Некорректная форма", Flash: &Flash{Kind: "error", Message: "Не удалось прочитать форму входа"}})
		return
	}

	response, err := h.auth.Login(r.Context(), user.LoginUserDTO{
		Email:    r.FormValue("email"),
		Password: r.FormValue("password"),
	})
	if err != nil {
		h.render(w, r, "login.html", PageData{Title: "Login", Error: "Не удалось войти", Flash: &Flash{Kind: "error", Message: "Не удалось войти: проверьте email и пароль"}})
		return
	}

	setAuthCookies(w, response)
	h.log(r, response.User.ID, "auth.login", "user", response.User.ID, nil)
	flashSuccess(w, "Вы вошли")
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (h *Handler) RegisterPage(w http.ResponseWriter, r *http.Request) {
	h.render(w, r, "register.html", PageData{Title: "Register"})
}

func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		h.render(w, r, "register.html", PageData{Title: "Register", Error: "Некорректная форма", Flash: &Flash{Kind: "error", Message: "Не удалось прочитать форму регистрации"}})
		return
	}

	response, err := h.auth.Register(r.Context(), user.RegisterUserDTO{
		Username: r.FormValue("username"),
		Email:    r.FormValue("email"),
		Password: r.FormValue("password"),
	})
	if err != nil {
		h.render(w, r, "register.html", PageData{Title: "Register", Error: "Не удалось зарегистрироваться", Flash: &Flash{Kind: "error", Message: "Не удалось зарегистрироваться"}})
		return
	}

	setAuthCookies(w, response)
	h.log(r, response.User.ID, "auth.register", "user", response.User.ID, nil)
	flashSuccess(w, "Аккаунт создан")
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	var userID *int64
	if currentUser, ok := middleware.CurrentUser(r.Context()); ok {
		userID = &currentUser.ID
	}
	if cookie, err := r.Cookie("refresh_token"); err == nil {
		_ = h.auth.RevokeRefreshToken(r.Context(), cookie.Value)
	}
	clearAuthCookies(w)
	h.logPtr(r, userID, "auth.logout", "user", userID, nil)
	flashSuccess(w, "Вы вышли")
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}
