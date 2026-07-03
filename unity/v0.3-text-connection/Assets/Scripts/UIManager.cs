using System;
using System.Collections.Generic;
using UnityEngine;
using UnityEngine.UI;

namespace AICompanion
{
    /// <summary>
    /// Unity v0.3 テキスト接続 UI マネージャ。
    /// InputField + Button + ScrollView の制御および
    /// WebSocketClient とのやり取りを担当する。
    /// </summary>
    [Serializable]
    public class MessageJson
    {
        public string type;
        public string payload;
        public string request_id;
    }

    [Serializable]
    public class AiResponseJson
    {
        public string type;
        public string request_id;
        public string conversation_id;
        public AssistantJson assistant;
        public string error;
    }

    [Serializable]
    public class AssistantJson
    {
        public string text;
        public string emotion;
        public string motion;
        public string speak_style;
        public bool interruptible;
    }

    [Serializable]
    public class StateChangeJson
    {
        public string type;
        public string state;
    }

    [Serializable]
    public class ErrorJson
    {
        public string type;
        public string request_id;
        public string error;
    }

    public class UIManager : MonoBehaviour
    {
        [Header("UI References")]
        [SerializeField] private InputField _inputField;
        [SerializeField] private Button _sendButton;
        [SerializeField] private Text _responseText;
        [SerializeField] private ScrollRect _scrollRect;
        [SerializeField] private Text _statusText;
        [SerializeField] private Text _titleText;

        [Header("Settings")]
        [SerializeField] private string _wsUrl = "ws://192.168.12.112:8080/ws";
        [SerializeField] private int _maxResponseLines = 200;

        private WebSocketClient _wsClient;
        private readonly List<string> _responseLines = new();
        private string _currentState = "—";

        private void Start()
        {
            // タイトル
            if (_titleText != null)
                _titleText.text = "AI Companion v0.3";

            // WebSocket クライアント初期化
            _wsClient = new WebSocketClient(_wsUrl);
            _wsClient.OnConnected += HandleConnected;
            _wsClient.OnDisconnected += HandleDisconnected;
            _wsClient.OnMessageReceived += HandleMessageReceived;
            _wsClient.OnError += HandleError;

            // 送信ボタン
            if (_sendButton != null)
                _sendButton.onClick.AddListener(OnSendClicked);

            // Enter キーでも送信
            if (_inputField != null)
                _inputField.onEndEdit.AddListener(OnInputEndEdit);

            // 自動接続
            _ = _wsClient.ConnectAsync();

            UpdateStatus("接続中...");
        }

        private void OnDestroy()
        {
            _ = _wsClient?.DisconnectAsync();
            _wsClient?.Dispose();
        }

        /// <summary>
        /// 送信ボタン押下時の処理。
        /// </summary>
        public void OnSendClicked()
        {
            SendMessage();
        }

        /// <summary>
        /// InputField で Enter キーが押された時の処理。
        /// </summary>
        private void OnInputEndEdit(string text)
        {
            if (Input.GetKeyDown(KeyCode.Return) || Input.GetKeyDown(KeyCode.KeypadEnter))
            {
                SendMessage();
            }
        }

        private async void SendMessage()
        {
            if (_inputField == null) return;

            var text = _inputField.text.Trim();
            if (string.IsNullOrEmpty(text)) return;

            // 送信中は無効化
            SetSendingState(true);

            // ユーザー入力を表示
            AppendResponse($"<color=#888888>You:</color> {text}");

            var requestId = Guid.NewGuid().ToString();
            var json = JsonUtility.ToJson(new MessageJson
            {
                type = "text",
                payload = text,
                request_id = requestId
            });

            await _wsClient.SendAsync(json);

            // 入力フィールドをクリア
            _inputField.text = "";
            _inputField.ActivateInputField();
        }

        private void SetSendingState(bool sending)
        {
            if (_sendButton != null)
                _sendButton.interactable = !sending;

            if (_inputField != null)
                _inputField.interactable = !sending;
        }

        private void HandleConnected()
        {
            UnityMainThreadDispatcher.Enqueue(() =>
            {
                UpdateStatus("接続済み");
                SetSendingState(false);
            });
        }

        private void HandleDisconnected(string reason)
        {
            UnityMainThreadDispatcher.Enqueue(() =>
            {
                UpdateStatus($"切断 ({reason})");
                SetSendingState(false);
            });
        }

        private void HandleError(string error)
        {
            UnityMainThreadDispatcher.Enqueue(() =>
            {
                AppendResponse($"<color=#ff6666>[Error] {error}</color>");
                SetSendingState(false);
            });
        }

        private void HandleMessageReceived(string json)
        {
            // メッセージタイプを判別して処理
            UnityMainThreadDispatcher.Enqueue(() =>
            {
                try
                {
                    // state_change を試す
                    var stateChange = JsonUtility.FromJson<StateChangeJson>(json);
                    if (stateChange.type == "state_change")
                    {
                        _currentState = stateChange.state;
                        UpdateStatus($"状態: {_currentState}");
                        return;
                    }
                }
                catch { /* 別タイプ */ }

                try
                {
                    // ai_response を試す
                    var aiResp = JsonUtility.FromJson<AiResponseJson>(json);
                    if (aiResp.type == "ai_response")
                    {
                        if (!string.IsNullOrEmpty(aiResp.error))
                        {
                            AppendResponse($"<color=#ff6666>[AI Error] {aiResp.error}</color>");
                        }
                        else if (aiResp.assistant != null)
                        {
                            AppendResponse($"<color=#66ccff>AI:</color> {aiResp.assistant.text}");
                        }
                        SetSendingState(false);
                        return;
                    }
                }
                catch { /* 別タイプ */ }

                try
                {
                    // error レスポンス（Python service error / timeout 等）
                    var errResp = JsonUtility.FromJson<ErrorJson>(json);
                    if (errResp.type == "error")
                    {
                        var errMsg = errResp.error ?? "Unknown error";
                        AppendResponse($"<color=#ff6666>[Server Error] {errMsg}</color>");
                        SetSendingState(false);
                        return;
                    }
                }
                catch { /* 別タイプ */ }

                // それ以外は ack など — そのまま表示
                try
                {
                    var msg = JsonUtility.FromJson<MessageJson>(json);
                    if (msg.type != null && msg.type.EndsWith("_ack"))
                    {
                        Debug.Log($"[UI] Ack: {msg.type} payload={msg.payload}");
                    }
                }
                catch
                {
                    AppendResponse($"<color=#aaaaaa>{json}</color>");
                }
            });
        }

        /// <summary>
        /// 応答表示エリアにテキストを追記する。
        /// 古い行は自動的に削除される。
        /// </summary>
        private void AppendResponse(string line)
        {
            _responseLines.Add(line);

            // 行数制限
            while (_responseLines.Count > _maxResponseLines)
                _responseLines.RemoveAt(0);

            if (_responseText != null)
            {
                _responseText.text = string.Join("\n", _responseLines);
            }

            // 自動スクロール
            if (_scrollRect != null)
            {
                Canvas.ForceUpdateCanvases();
                _scrollRect.verticalNormalizedPosition = 0f;
            }
        }

        private void UpdateStatus(string status)
        {
            if (_statusText != null)
                _statusText.text = status;
        }
    }

    /// <summary>
    /// WebSocket のコールバックをメインスレッドで処理するための
    /// 簡易ディスパッチャ。
    /// </summary>
    public class UnityMainThreadDispatcher : MonoBehaviour
    {
        private static readonly Queue<Action> _queue = new();
        private static UnityMainThreadDispatcher _instance;

        private void Awake()
        {
            if (_instance != null)
            {
                Destroy(gameObject);
                return;
            }
            _instance = this;
            DontDestroyOnLoad(gameObject);
        }

        private void Update()
        {
            lock (_queue)
            {
                while (_queue.Count > 0)
                {
                    _queue.Dequeue()?.Invoke();
                }
            }
        }

        public static void Enqueue(Action action)
        {
            lock (_queue)
            {
                _queue.Enqueue(action);
            }
        }

        [RuntimeInitializeOnLoadMethod(RuntimeInitializeLoadType.BeforeSceneLoad)]
        private static void Initialize()
        {
            if (_instance == null)
            {
                var go = new GameObject("UnityMainThreadDispatcher");
                go.AddComponent<UnityMainThreadDispatcher>();
            }
        }
    }
}
