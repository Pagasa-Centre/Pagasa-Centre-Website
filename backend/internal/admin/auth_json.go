package admin

import (
	"encoding/json"
	"io"
	"net/http"
)

func decodeLoginBody(r *http.Request, dst any) error {
	const maxBody = 1 << 20
	body, err := io.ReadAll(io.LimitReader(r.Body, maxBody))
	if err != nil {
		return err
	}
	return json.Unmarshal(body, dst)
}
