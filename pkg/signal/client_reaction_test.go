package signal

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSendReaction(t *testing.T) {
	for _, test := range []struct {
		name      string
		reaction  string
		remove    bool
		method    string
		wantEmoji string
	}{
		{name: "add reaction", reaction: "😮", method: http.MethodPost, wantEmoji: "😮"},
		{name: "remove reaction", remove: true, method: http.MethodDelete},
	} {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				require.Equal(t, test.method, r.Method)
				require.Equal(t, "/v1/reactions/+15550001111", r.URL.Path)

				var body map[string]interface{}
				require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
				require.Equal(t, "+15552223333", body["recipient"])
				if test.remove {
					require.NotContains(t, body, "reaction")
				} else {
					require.Equal(t, test.wantEmoji, body["reaction"])
				}
				require.Equal(t, "+15550001111", body["target_author"])
				require.Equal(t, float64(1790056487089), body["timestamp"])
				w.WriteHeader(http.StatusNoContent)
			}))
			defer server.Close()

			client := NewClient(server.URL, "+15550001111", "test", "", server.Client())
			err := client.SendReaction(context.Background(), "+15552223333", test.reaction, "+15550001111", 1790056487089, test.remove)
			require.NoError(t, err)
		})
	}
}
