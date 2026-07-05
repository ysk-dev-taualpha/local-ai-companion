using System;
using System.Collections;
using System.Collections.Generic;
using System.Text;
using UnityEngine;
using UnityEngine.EventSystems;
using UnityEngine.Networking;
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
    public class ConversationRequestJson
    {
        public string message;
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

    [Serializable]
    public class AudioMessageJson
    {
        public string type;
        public string request_id;
        public string data;
        public string audio;
        public string audio_base64;
        public string format;
        public string mime_type;
    }

    [Serializable]
    public class AudioControlJson
    {
        public string type;
        public string request_id;
        public string action;
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
        [SerializeField] private string _wsUrl = "ws://192.168.12.112:8090/ws";
        [SerializeField] private string _httpFallbackUrl = "http://192.168.12.112:8090/v1/conversation";
        [SerializeField] private int _maxResponseLines = 200;

        [Header("Audio")]
        [SerializeField] private AudioSource _audioSource;
        [SerializeField] private int _maxAudioQueueSize = 8;

        private WebSocketClient _wsClient;
        private readonly List<string> _responseLines = new();
        private readonly Queue<AudioClip> _audioQueue = new();
        private string _currentState = "UNKNOWN";
        private bool _httpFallbackMode;
        private Coroutine _audioPlaybackCoroutine;

        [RuntimeInitializeOnLoadMethod(RuntimeInitializeLoadType.AfterSceneLoad)]
        private static void Bootstrap()
        {
            if (FindObjectOfType<UIManager>() != null)
                return;

            var go = new GameObject("AICompanionUI");
            go.AddComponent<UIManager>();
        }

        private void Start()
        {
            EnsureUIReferences();

            // タイトル
            if (_titleText != null)
                _titleText.text = "AI Companion v0.3";

            if (string.IsNullOrEmpty(_wsUrl))
            {
                _httpFallbackMode = true;
                UpdateStatus("HTTP 接続");
            }
            else
            {
                // WebSocket クライアント初期化
                _wsClient = new WebSocketClient(_wsUrl);
                _wsClient.OnConnected += HandleConnected;
                _wsClient.OnDisconnected += HandleDisconnected;
                _wsClient.OnMessageReceived += HandleMessageReceived;
                _wsClient.OnError += HandleError;
            }

            // 送信ボタン
            if (_sendButton != null)
                _sendButton.onClick.AddListener(OnSendClicked);

            // Enter キーでも送信
            if (_inputField != null)
                _inputField.onEndEdit.AddListener(OnInputEndEdit);

            if (_wsClient != null)
            {
                // 自動接続
                _ = _wsClient.ConnectAsync();
                UpdateStatus("接続中...");
            }
        }

        private void OnDestroy()
        {
            _ = _wsClient?.DisconnectAsync();
            _wsClient?.Dispose();
            StopAudioPlayback();
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

            if (_httpFallbackMode || _wsClient == null || !_wsClient.IsConnected)
            {
                StartCoroutine(SendHttpFallback(text, requestId));
            }
            else
            {
                await _wsClient.SendAsync(json);
            }

            // 入力フィールドをクリア
            _inputField.text = "";
            _inputField.ActivateInputField();
        }

        private IEnumerator SendHttpFallback(string text, string requestId)
        {
            var json = JsonUtility.ToJson(new ConversationRequestJson
            {
                message = text,
                request_id = requestId
            });
            var body = Encoding.UTF8.GetBytes(json);

            using (var req = new UnityWebRequest(_httpFallbackUrl, "POST"))
            {
                req.uploadHandler = new UploadHandlerRaw(body);
                req.downloadHandler = new DownloadHandlerBuffer();
                req.timeout = 30;
                req.SetRequestHeader("Content-Type", "application/json");

                UpdateStatus("送信中...");
                yield return req.SendWebRequest();

                if (req.result != UnityWebRequest.Result.Success)
                {
                    var detail = $"HTTP Error: {req.error} status={req.responseCode} url={_httpFallbackUrl}";
                    Debug.LogError($"[UI] {detail}");
                    AppendResponse($"<color=#ff6666>[{detail}]</color>");
                    if (!string.IsNullOrEmpty(req.downloadHandler.text))
                        AppendResponse($"<color=#ff9999>{req.downloadHandler.text}</color>");
                    UpdateStatus($"HTTP エラー {req.responseCode}: {req.error}");
                    SetSendingState(false);
                    yield break;
                }

                DisplayAiResponse(req.downloadHandler.text);
                UpdateStatus("HTTP 接続");
                SetSendingState(false);
            }
        }

        private void DisplayAiResponse(string json)
        {
            try
            {
                var aiResp = JsonUtility.FromJson<AiResponseJson>(json);
                if (aiResp != null && aiResp.assistant != null)
                {
                    AppendResponse($"<color=#66ccff>AI:</color> {aiResp.assistant.text}");
                    return;
                }

                if (aiResp != null && !string.IsNullOrEmpty(aiResp.error))
                {
                    AppendResponse($"<color=#ff6666>[AI Error] {aiResp.error}</color>");
                    return;
                }
            }
            catch
            {
                // Fall through to raw display.
            }

            AppendResponse($"<color=#aaaaaa>{json}</color>");
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
                AppendResponse("<color=#88cc88>[Connected]</color>");
                SetSendingState(false);
            });
        }

        private void HandleDisconnected(string reason)
        {
            UnityMainThreadDispatcher.Enqueue(() =>
            {
                UpdateStatus($"切断 ({reason})");
                AppendResponse($"<color=#ffcc66>[Disconnected] {reason}</color>");
                SetSendingState(false);
            });
        }

        private void HandleError(string error)
        {
            UnityMainThreadDispatcher.Enqueue(() =>
            {
                _httpFallbackMode = !string.IsNullOrEmpty(_httpFallbackUrl);
                if (_httpFallbackMode)
                    UpdateStatus("HTTP 接続");
                else
                    AppendResponse($"<color=#ff6666>[Error] {error}</color>");
                SetSendingState(false);
            });
        }

        private void HandleMessageReceived(string json)
        {
            UnityMainThreadDispatcher.Enqueue(() =>
            {
                if (TryHandleStateChange(json))
                    return;
                if (TryHandleAudioMessage(json))
                    return;
                if (TryHandleAudioControl(json))
                    return;
                if (TryHandleAiResponse(json))
                    return;
                if (TryHandleError(json))
                    return;
                if (TryHandleAck(json))
                    return;

                AppendResponse($"<color=#aaaaaa>{json}</color>");
            });
        }

        private bool TryHandleStateChange(string json)
        {
            try
            {
                var stateChange = JsonUtility.FromJson<StateChangeJson>(json);
                if (stateChange == null || stateChange.type != "state_change")
                    return false;

                _currentState = stateChange.state;
                UpdateStatus($"State: {_currentState}");
                if (_currentState == "SPEAKING")
                {
                    EnsureAudioPlayback();
                }
                else if (_currentState == "IDLE")
                {
                    StopAudioPlayback();
                }
                return true;
            }
            catch
            {
                return false;
            }
        }

        private bool TryHandleAudioMessage(string json)
        {
            try
            {
                var audioResp = JsonUtility.FromJson<AudioMessageJson>(json);
                if (audioResp == null || audioResp.type != "audio")
                    return false;

                HandleAudioMessage(audioResp);
                return true;
            }
            catch (Exception ex)
            {
                AppendResponse($"<color=#ff6666>[Audio Error] {ex.Message}</color>");
                return true;
            }
        }

        private bool TryHandleAudioControl(string json)
        {
            try
            {
                var control = JsonUtility.FromJson<AudioControlJson>(json);
                if (control == null || control.type != "audio_control")
                    return false;

                switch (control.action)
                {
                    case "stop":
                        StopAudioPlayback();
                        AppendResponse("<color=#ffcc66>[Audio] stopped</color>");
                        return true;
                    case "clear_queue":
                        ClearAudioQueue();
                        AppendResponse("<color=#ffcc66>[Audio] queue cleared</color>");
                        return true;
                    default:
                        AppendResponse($"<color=#ff6666>[Audio Control Error] unsupported action: {control.action}</color>");
                        return true;
                }
            }
            catch (Exception ex)
            {
                AppendResponse($"<color=#ff6666>[Audio Control Error] {ex.Message}</color>");
                return true;
            }
        }

        private bool TryHandleAiResponse(string json)
        {
            try
            {
                var aiResp = JsonUtility.FromJson<AiResponseJson>(json);
                if (aiResp == null || (aiResp.type != "ai_response" && aiResp.assistant == null))
                    return false;

                if (!string.IsNullOrEmpty(aiResp.error))
                {
                    AppendResponse($"<color=#ff6666>[AI Error] {aiResp.error}</color>");
                }
                else if (aiResp.assistant != null)
                {
                    AppendResponse($"<color=#66ccff>AI:</color> {aiResp.assistant.text}");
                }
                SetSendingState(false);
                return true;
            }
            catch
            {
                return false;
            }
        }

        private bool TryHandleError(string json)
        {
            try
            {
                var errResp = JsonUtility.FromJson<ErrorJson>(json);
                if (errResp == null || errResp.type != "error")
                    return false;

                var errMsg = errResp.error ?? "Unknown error";
                AppendResponse($"<color=#ff6666>[Server Error] {errMsg}</color>");
                SetSendingState(false);
                return true;
            }
            catch
            {
                return false;
            }
        }

        private bool TryHandleAck(string json)
        {
            try
            {
                var msg = JsonUtility.FromJson<MessageJson>(json);
                if (msg == null || msg.type == null || !msg.type.EndsWith("_ack"))
                    return false;

                Debug.Log($"[UI] Ack: {msg.type} payload={msg.payload}");
                AppendResponse($"<color=#aaaaaa>[Ack] {msg.type}</color>");
                return true;
            }
            catch
            {
                return false;
            }
        }

        private void HandleAudioMessage(AudioMessageJson audioResp)
        {
            var base64 = FirstNonEmpty(audioResp.data, audioResp.audio, audioResp.audio_base64);
            if (string.IsNullOrEmpty(base64))
            {
                AppendResponse("<color=#ff6666>[Audio Error] missing base64 data</color>");
                return;
            }

            try
            {
                var bytes = Convert.FromBase64String(base64);
                var clip = WavAudioClip.Create(bytes, $"AICompanionAudio-{audioResp.request_id}");
                EnqueueAudio(clip);
                AppendResponse($"<color=#88ccff>[Audio] queued {clip.length:0.00}s</color>");
            }
            catch (Exception ex)
            {
                Debug.LogError($"[UI] Audio decode failed: {ex.Message}");
                AppendResponse($"<color=#ff6666>[Audio Error] {ex.Message}</color>");
            }
        }

        private void EnqueueAudio(AudioClip clip)
        {
            while (_audioQueue.Count >= _maxAudioQueueSize)
            {
                var dropped = _audioQueue.Dequeue();
                if (dropped != null)
                    Destroy(dropped);
            }

            _audioQueue.Enqueue(clip);
            EnsureAudioPlayback();
        }

        private void EnsureAudioPlayback()
        {
            if (_audioSource == null)
                _audioSource = gameObject.AddComponent<AudioSource>();

            if (_audioPlaybackCoroutine == null && _audioQueue.Count > 0)
                _audioPlaybackCoroutine = StartCoroutine(PlayQueuedAudio());
        }

        private IEnumerator PlayQueuedAudio()
        {
            while (_audioQueue.Count > 0)
            {
                var clip = _audioQueue.Dequeue();
                if (clip == null)
                    continue;

                _audioSource.clip = clip;
                _audioSource.Play();
                UpdateStatus("Audio playing");

                while (_audioSource != null && _audioSource.isPlaying)
                    yield return null;

                Destroy(clip);
            }

            _audioPlaybackCoroutine = null;
            if (_currentState != "SPEAKING")
                UpdateStatus($"State: {_currentState}");
        }

        private void StopAudioPlayback()
        {
            if (_audioPlaybackCoroutine != null)
            {
                StopCoroutine(_audioPlaybackCoroutine);
                _audioPlaybackCoroutine = null;
            }

            if (_audioSource != null)
            {
                _audioSource.Stop();
                if (_audioSource.clip != null)
                    Destroy(_audioSource.clip);
                _audioSource.clip = null;
            }

            ClearAudioQueue();
        }

        private void ClearAudioQueue()
        {
            while (_audioQueue.Count > 0)
            {
                var clip = _audioQueue.Dequeue();
                if (clip != null)
                    Destroy(clip);
            }
        }

        private static string FirstNonEmpty(params string[] values)
        {
            foreach (var value in values)
            {
                if (!string.IsNullOrEmpty(value))
                    return value;
            }
            return null;
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

        private void EnsureUIReferences()
        {
            if (_inputField != null && _sendButton != null && _responseText != null && _scrollRect != null)
                return;

            var canvas = CreateCanvas();

            if (FindObjectOfType<EventSystem>() == null)
            {
                var eventSystem = new GameObject("EventSystem");
                eventSystem.AddComponent<EventSystem>();
                eventSystem.AddComponent<StandaloneInputModule>();
            }

            var root = CreateRect("AICompanionPanel", canvas.transform);
            root.anchorMin = Vector2.zero;
            root.anchorMax = Vector2.one;
            root.offsetMin = Vector2.zero;
            root.offsetMax = Vector2.zero;

            _titleText = CreateText("TitleText", root, "AI Companion v0.3", 28, TextAnchor.MiddleLeft);
            SetRect(_titleText.rectTransform, new Vector2(0f, 1f), new Vector2(1f, 1f), new Vector2(24f, -72f), new Vector2(-24f, -24f));

            _statusText = CreateText("StatusText", root, "接続中...", 14, TextAnchor.MiddleLeft);
            _statusText.color = Color.gray;
            SetRect(_statusText.rectTransform, new Vector2(0f, 1f), new Vector2(1f, 1f), new Vector2(24f, -104f), new Vector2(-24f, -76f));

            var scrollObject = new GameObject("ScrollView", typeof(RectTransform), typeof(Image), typeof(ScrollRect));
            scrollObject.transform.SetParent(root, false);
            var scrollRectTransform = scrollObject.GetComponent<RectTransform>();
            SetRect(scrollRectTransform, new Vector2(0f, 0f), new Vector2(1f, 1f), new Vector2(24f, 88f), new Vector2(-24f, -120f));
            var scrollImage = scrollObject.GetComponent<Image>();
            scrollImage.color = new Color(0.02f, 0.02f, 0.02f, 0.96f);

            var content = CreateRect("Content", scrollRectTransform);
            content.anchorMin = Vector2.zero;
            content.anchorMax = Vector2.one;
            content.pivot = new Vector2(0.5f, 1f);
            content.offsetMin = new Vector2(16f, 12f);
            content.offsetMax = new Vector2(-16f, -12f);

            _responseText = CreateText("ResponseText", content, "", 16, TextAnchor.UpperLeft);
            _responseText.supportRichText = true;
            _responseText.horizontalOverflow = HorizontalWrapMode.Wrap;
            _responseText.verticalOverflow = VerticalWrapMode.Truncate;
            SetRect(_responseText.rectTransform, Vector2.zero, Vector2.one, Vector2.zero, Vector2.zero);

            _scrollRect = scrollObject.GetComponent<ScrollRect>();
            _scrollRect.content = content;
            _scrollRect.viewport = null;
            _scrollRect.horizontal = false;
            _scrollRect.vertical = true;

            _inputField = CreateInputField(root);
            SetRect(_inputField.GetComponent<RectTransform>(), new Vector2(0f, 0f), new Vector2(1f, 0f), new Vector2(24f, 24f), new Vector2(-128f, 72f));

            _sendButton = CreateButton(root);
            SetRect(_sendButton.GetComponent<RectTransform>(), new Vector2(1f, 0f), new Vector2(1f, 0f), new Vector2(-112f, 24f), new Vector2(-24f, 72f));
        }

        private static Canvas CreateCanvas()
        {
            var canvasObject = new GameObject("Canvas", typeof(Canvas), typeof(CanvasScaler), typeof(GraphicRaycaster));
            var canvas = canvasObject.GetComponent<Canvas>();
            canvas.renderMode = RenderMode.ScreenSpaceOverlay;
            canvas.sortingOrder = 1000;

            var scaler = canvasObject.GetComponent<CanvasScaler>();
            scaler.uiScaleMode = CanvasScaler.ScaleMode.ScaleWithScreenSize;
            scaler.referenceResolution = new Vector2(1280f, 720f);
            scaler.matchWidthOrHeight = 0.5f;

            return canvas;
        }

        private static RectTransform CreateRect(string name, Transform parent)
        {
            var go = new GameObject(name, typeof(RectTransform));
            go.transform.SetParent(parent, false);
            return go.GetComponent<RectTransform>();
        }

        private static Text CreateText(string name, Transform parent, string text, int fontSize, TextAnchor alignment)
        {
            var go = new GameObject(name, typeof(RectTransform), typeof(Text));
            go.transform.SetParent(parent, false);
            var label = go.GetComponent<Text>();
            label.text = text;
            label.font = Resources.GetBuiltinResource<Font>("LegacyRuntime.ttf");
            label.fontSize = fontSize;
            label.alignment = alignment;
            label.color = Color.white;
            return label;
        }

        private static InputField CreateInputField(Transform parent)
        {
            var go = new GameObject("InputField", typeof(RectTransform), typeof(Image), typeof(InputField));
            go.transform.SetParent(parent, false);
            go.GetComponent<Image>().color = Color.white;

            var text = CreateText("Text", go.transform, "", 16, TextAnchor.MiddleLeft);
            text.color = Color.black;
            SetRect(text.rectTransform, Vector2.zero, Vector2.one, new Vector2(12f, 6f), new Vector2(-12f, -6f));

            var placeholder = CreateText("Placeholder", go.transform, "メッセージを入力...", 16, TextAnchor.MiddleLeft);
            placeholder.color = new Color(0.45f, 0.45f, 0.45f, 1f);
            SetRect(placeholder.rectTransform, Vector2.zero, Vector2.one, new Vector2(12f, 6f), new Vector2(-12f, -6f));

            var input = go.GetComponent<InputField>();
            input.textComponent = text;
            input.placeholder = placeholder;
            return input;
        }

        private static Button CreateButton(Transform parent)
        {
            var go = new GameObject("SendButton", typeof(RectTransform), typeof(Image), typeof(Button));
            go.transform.SetParent(parent, false);
            go.GetComponent<Image>().color = new Color(0.16f, 0.42f, 0.82f, 1f);

            var label = CreateText("Text", go.transform, "送信", 16, TextAnchor.MiddleCenter);
            SetRect(label.rectTransform, Vector2.zero, Vector2.one, Vector2.zero, Vector2.zero);
            return go.GetComponent<Button>();
        }

        private static void SetRect(RectTransform rect, Vector2 anchorMin, Vector2 anchorMax, Vector2 offsetMin, Vector2 offsetMax)
        {
            rect.anchorMin = anchorMin;
            rect.anchorMax = anchorMax;
            rect.offsetMin = offsetMin;
            rect.offsetMax = offsetMax;
        }
    }

    internal static class WavAudioClip
    {
        public static AudioClip Create(byte[] wavBytes, string name)
        {
            if (wavBytes == null || wavBytes.Length < 44)
                throw new ArgumentException("WAV data is too short");
            if (ReadString(wavBytes, 0, 4) != "RIFF" || ReadString(wavBytes, 8, 4) != "WAVE")
                throw new ArgumentException("Audio message must contain RIFF/WAVE data");

            var offset = 12;
            var audioFormat = 0;
            var channels = 0;
            var sampleRate = 0;
            var bitsPerSample = 0;
            var dataOffset = -1;
            var dataSize = 0;

            while (offset + 8 <= wavBytes.Length)
            {
                var chunkId = ReadString(wavBytes, offset, 4);
                var chunkSize = BitConverter.ToInt32(wavBytes, offset + 4);
                var chunkDataOffset = offset + 8;

                if (chunkDataOffset + chunkSize > wavBytes.Length)
                    throw new ArgumentException("WAV chunk extends past end of data");

                if (chunkId == "fmt ")
                {
                    audioFormat = BitConverter.ToInt16(wavBytes, chunkDataOffset);
                    channels = BitConverter.ToInt16(wavBytes, chunkDataOffset + 2);
                    sampleRate = BitConverter.ToInt32(wavBytes, chunkDataOffset + 4);
                    bitsPerSample = BitConverter.ToInt16(wavBytes, chunkDataOffset + 14);
                }
                else if (chunkId == "data")
                {
                    dataOffset = chunkDataOffset;
                    dataSize = chunkSize;
                }

                offset = chunkDataOffset + chunkSize + (chunkSize % 2);
            }

            if (dataOffset < 0)
                throw new ArgumentException("WAV data chunk was not found");
            if (channels <= 0 || sampleRate <= 0 || bitsPerSample <= 0)
                throw new ArgumentException("WAV format chunk is invalid");
            if (audioFormat != 1 && audioFormat != 3)
                throw new ArgumentException($"Unsupported WAV format: {audioFormat}");

            var bytesPerSample = bitsPerSample / 8;
            var sampleCount = dataSize / bytesPerSample;
            var frameCount = sampleCount / channels;
            var samples = new float[sampleCount];

            for (var i = 0; i < sampleCount; i++)
            {
                var sampleOffset = dataOffset + (i * bytesPerSample);
                samples[i] = ReadSample(wavBytes, sampleOffset, bitsPerSample, audioFormat);
            }

            var clip = AudioClip.Create(string.IsNullOrEmpty(name) ? "AICompanionAudio" : name, frameCount, channels, sampleRate, false);
            clip.SetData(samples, 0);
            return clip;
        }

        private static float ReadSample(byte[] bytes, int offset, int bitsPerSample, int audioFormat)
        {
            if (audioFormat == 3 && bitsPerSample == 32)
                return Mathf.Clamp(BitConverter.ToSingle(bytes, offset), -1f, 1f);

            switch (bitsPerSample)
            {
                case 8:
                    return (bytes[offset] - 128) / 128f;
                case 16:
                    return BitConverter.ToInt16(bytes, offset) / 32768f;
                case 24:
                    var value = bytes[offset] | (bytes[offset + 1] << 8) | (bytes[offset + 2] << 16);
                    if ((value & 0x800000) != 0)
                        value |= unchecked((int)0xff000000);
                    return value / 8388608f;
                case 32:
                    return BitConverter.ToInt32(bytes, offset) / 2147483648f;
                default:
                    throw new ArgumentException($"Unsupported WAV bit depth: {bitsPerSample}");
            }
        }

        private static string ReadString(byte[] bytes, int offset, int count)
        {
            return Encoding.ASCII.GetString(bytes, offset, count);
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
        private static int _mainThreadId;

        private void Awake()
        {
            _mainThreadId = System.Threading.Thread.CurrentThread.ManagedThreadId;
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
            if (_mainThreadId == 0 || System.Threading.Thread.CurrentThread.ManagedThreadId == _mainThreadId)
            {
                action?.Invoke();
                return;
            }

            lock (_queue)
            {
                _queue.Enqueue(action);
            }
        }

        [RuntimeInitializeOnLoadMethod(RuntimeInitializeLoadType.BeforeSceneLoad)]
        private static void Initialize()
        {
            _mainThreadId = System.Threading.Thread.CurrentThread.ManagedThreadId;
            if (_instance == null)
            {
                var go = new GameObject("UnityMainThreadDispatcher");
                go.AddComponent<UnityMainThreadDispatcher>();
            }
        }
    }
}
