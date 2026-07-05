package tts

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewVOICEVOX(t *testing.T) {
	client := NewVOICEVOX("http://127.0.0.1:50021", 3)
	if client == nil {
		t.Fatal("expected non-nil client")
	}
	if client.baseURL != "http://127.0.0.1:50021" {
		t.Errorf("expected baseURL 'http://127.0.0.1:50021', got %s", client.baseURL)
	}
	if client.speakerID != 3 {
		t.Errorf("expected speakerID 3, got %d", client.speakerID)
	}
	if client.client == nil {
		t.Error("expected non-nil HTTP client")
	}
}

func TestNewVOICEVOX_DifferentSpeaker(t *testing.T) {
	client := NewVOICEVOX("http://localhost:50021", 1)
	if client.speakerID != 1 {
		t.Errorf("expected speakerID 1, got %d", client.speakerID)
	}
}

func TestSpeak_EmptyText(t *testing.T) {
	client := NewVOICEVOX("http://127.0.0.1:50021", 3)
	_, err := client.Speak("")
	if err == nil {
		t.Error("expected error for empty text")
	}
	if err.Error() != "tts: empty text" {
		t.Errorf("expected 'tts: empty text', got '%s'", err.Error())
	}
}

func TestSpeak_Success(t *testing.T) {
	// VOICEVOX Engine のモックサーバー
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/audio_query":
			// audio_query: text + speaker → JSON クエリパラメータ
			text := r.URL.Query().Get("text")
			speaker := r.URL.Query().Get("speaker")
			if text == "" {
				t.Errorf("audio_query: expected text param")
			}
			if speaker == "" {
				t.Errorf("audio_query: expected speaker param")
			}
			// モックの audio_query レスポンス
			queryResp := map[string]interface{}{
				"accent_phrases":      []interface{}{},
				"speedScale":          1.0,
				"pitchScale":          0.0,
				"intonationScale":     1.0,
				"volumeScale":         1.0,
				"prePhonemeLength":    0.1,
				"postPhonemeLength":   0.1,
				"outputSamplingRate":  24000,
				"outputStereo":        false,
				"kana":                "テスト",
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(queryResp)

		case "/synthesis":
			// synthesis: speaker + JSON body → WAV バイナリ
			speaker := r.URL.Query().Get("speaker")
			if speaker == "" {
				t.Errorf("synthesis: expected speaker param")
			}
			// モックの WAV データ（最小限の WAV ヘッダ + ダミーデータ）
			wavData := make([]byte, 44) // WAV ヘッダサイズ
			wavData[0] = 'R'
			wavData[1] = 'I'
			wavData[2] = 'F'
			wavData[3] = 'F'
			w.Header().Set("Content-Type", "audio/wav")
			w.Write(wavData)

		default:
			t.Errorf("unexpected path: %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	client := NewVOICEVOX(srv.URL, 3)
	data, err := client.Speak("こんにちは")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(data) == 0 {
		t.Error("expected non-empty audio data")
	}
}

func TestSpeak_AudioQueryError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("internal error"))
	}))
	defer srv.Close()

	client := NewVOICEVOX(srv.URL, 3)
	_, err := client.Speak("こんにちは")
	if err == nil {
		t.Error("expected error for audio_query failure")
	}
}

func TestSpeak_SynthesisError(t *testing.T) {
	// audio_query は成功、synthesis は失敗
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		switch r.URL.Path {
		case "/audio_query":
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"accent_phrases": []interface{}{},
				"speedScale":     1.0,
			})
		case "/synthesis":
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte("synthesis engine unavailable"))
		}
	}))
	defer srv.Close()

	client := NewVOICEVOX(srv.URL, 3)
	_, err := client.Speak("こんにちは")
	if err == nil {
		t.Error("expected error for synthesis failure")
	}
}

func TestSpeak_InvalidBaseURL(t *testing.T) {
	// 無効な URL の場合は audio_query でエラーになる
	client := &VOICEVOXClient{
		baseURL:   "://invalid-url",
		speakerID: 3,
		client:    &http.Client{},
	}
	_, err := client.Speak("こんにちは")
	if err == nil {
		t.Error("expected error for invalid base URL")
	}
}

func TestTTSClientInterface(t *testing.T) {
	// VOICEVOXClient が TTSClient インターフェースを満たすことを確認
	var _ TTSClient = (*VOICEVOXClient)(nil) // コンパイル時チェック（tts.go に既にあるが念のため再確認）
	// このテストはコンパイルが通れば成功
}
