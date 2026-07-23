package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// httpEmbed devuelve un embedFunc que pega al endpoint OpenAI-compatible /embeddings.
// Mismo patrón que httpChat: net/http, sin SDK.
func httpEmbed(baseURL, apiKey, model string) embedFunc {
	// Timeout corto: el embed corre en CADA turno (antes del chat). Si el endpoint se cuelga,
	// mejor rendirse rápido y caer a reglas que dejar el turno esperando 20s.
	client := &http.Client{Timeout: 5 * time.Second}
	return func(text string) ([]float32, error) {
		body, _ := json.Marshal(map[string]any{"model": model, "input": text})
		url := strings.TrimRight(baseURL, "/") + "/embeddings"
		req, err := http.NewRequest("POST", url, bytes.NewReader(body))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+apiKey)
		resp, err := client.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		raw, _ := io.ReadAll(resp.Body)
		if resp.StatusCode != 200 {
			return nil, fmt.Errorf("embeddings HTTP %d: %s", resp.StatusCode, string(raw))
		}
		var out struct {
			Data []struct {
				Embedding []float32 `json:"embedding"`
			} `json:"data"`
		}
		if err := json.Unmarshal(raw, &out); err != nil || len(out.Data) == 0 {
			return nil, fmt.Errorf("respuesta embeddings inesperada: %s", string(raw))
		}
		return out.Data[0].Embedding, nil
	}
}
