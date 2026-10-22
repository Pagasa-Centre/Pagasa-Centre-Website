package dto

type LoginResponse struct {
	Name  string `json:"name"`
	Token string `json:"token"`
}

type SessionResponse struct {
	Name string `json:"name"`
}

func LoginFromSession(name, token string) LoginResponse {
	return LoginResponse{Name: name, Token: token}
}

func SessionFromName(name string) SessionResponse {
	return SessionResponse{Name: name}
}
