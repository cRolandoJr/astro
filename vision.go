package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// visionFunc mira una imagen y responde una pregunta sobre ella. Seam: real por HTTP o fake en tests.
type visionFunc func(question, imagePath string) (string, error)

// httpVision manda la imagen (base64) + la pregunta a un endpoint OpenAI-compatible multimodal
// (chat/completions con content de tipo image_url). Timeout más largo: la visión es lenta + sube imagen.
func httpVision(baseURL, apiKey, model string) visionFunc {
	client := &http.Client{Timeout: 40 * time.Second}
	return func(question, imagePath string) (string, error) {
		raw, err := os.ReadFile(imagePath)
		if err != nil {
			return "", fmt.Errorf("no pude leer la captura: %w", err)
		}
		dataURI := "data:image/png;base64," + base64.StdEncoding.EncodeToString(raw)
		body, _ := json.Marshal(map[string]any{
			"model": model,
			"messages": []map[string]any{
				{"role": "system", "content": "Sos Astro. Respondé BREVE (1-2 frases), en español, para leer en voz alta, sobre lo que ves en la imagen."},
				{"role": "user", "content": []any{
					map[string]string{"type": "text", "text": question},
					map[string]any{"type": "image_url", "image_url": map[string]string{"url": dataURI}},
				}},
			},
		})
		url := strings.TrimRight(baseURL, "/") + "/chat/completions"
		req, err := http.NewRequest("POST", url, bytes.NewReader(body))
		if err != nil {
			return "", err
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+apiKey)
		resp, err := client.Do(req)
		if err != nil {
			return "", err
		}
		defer resp.Body.Close()
		out, _ := io.ReadAll(resp.Body)
		if resp.StatusCode != 200 {
			return "", fmt.Errorf("visión HTTP %d: %s", resp.StatusCode, string(out))
		}
		var parsed struct {
			Choices []struct {
				Message struct {
					Content string `json:"content"`
				} `json:"message"`
			} `json:"choices"`
		}
		if err := json.Unmarshal(out, &parsed); err != nil || len(parsed.Choices) == 0 {
			return "", fmt.Errorf("respuesta visión inesperada: %s", string(out))
		}
		return parsed.Choices[0].Message.Content, nil
	}
}
