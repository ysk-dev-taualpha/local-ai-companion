using System;
using System.Collections.Generic;
using System.Net.WebSockets;
using System.Text;
using System.Threading;
using System.Threading.Tasks;
using UnityEngine;

namespace AICompanion
{
    /// <summary>
    /// Go Runtime (192.168.12.112:8080) の /ws エンドポイントに接続する
    /// WebSocket クライアント。自動再接続機能付き。
    /// </summary>
    public class WebSocketClient : IDisposable
    {
        private const string DefaultUrl = "ws://192.168.12.112:8080/ws";
        private const int ReceiveBufferSize = 4096;
        private const float ReconnectDelaySeconds = 3f;

        private readonly string _url;
        private ClientWebSocket _ws;
        private CancellationTokenSource _cts;
        private bool _isDisposed;

        /// <summary>接続確立時に発火。</summary>
        public event Action OnConnected;

        /// <summary>切断時に発火。引数は切断理由。</summary>
        public event Action<string> OnDisconnected;

        /// <summary>テキストメッセージ受信時に発火。引数は JSON 文字列。</summary>
        public event Action<string> OnMessageReceived;

        /// <summary>エラー発生時に発火。引数はエラーメッセージ。</summary>
        public event Action<string> OnError;

        /// <summary>接続中かどうか。</summary>
        public bool IsConnected => _ws?.State == WebSocketState.Open;

        public WebSocketClient(string url = null)
        {
            _url = url ?? DefaultUrl;
        }

        /// <summary>
        /// WebSocket 接続を開始し、バックグラウンドで受信ループを開始する。
        /// </summary>
        public async Task ConnectAsync()
        {
            if (_isDisposed) return;

            _cts?.Cancel();
            _cts?.Dispose();
            _cts = new CancellationTokenSource();

            try
            {
                _ws?.Dispose();
                _ws = new ClientWebSocket();
                await _ws.ConnectAsync(new Uri(_url), _cts.Token);

                Debug.Log($"[WebSocketClient] Connected to {_url}");
                OnConnected?.Invoke();

                // バックグラウンドで受信ループ開始
                _ = ReceiveLoopAsync(_cts.Token);
            }
            catch (Exception ex)
            {
                Debug.LogWarning($"[WebSocketClient] Connect failed: {ex.Message}");
                OnError?.Invoke(ex.Message);
                // 自動再接続
                _ = ReconnectAfterDelayAsync();
            }
        }

        /// <summary>
        /// JSON メッセージを WebSocket で送信する。
        /// </summary>
        public async Task SendAsync(string json)
        {
            if (!IsConnected)
            {
                Debug.LogWarning("[WebSocketClient] Cannot send: not connected");
                OnError?.Invoke("Not connected");
                return;
            }

            try
            {
                var buffer = Encoding.UTF8.GetBytes(json);
                await _ws.SendAsync(
                    new ArraySegment<byte>(buffer),
                    WebSocketMessageType.Text,
                    endOfMessage: true,
                    _cts.Token);
            }
            catch (Exception ex)
            {
                Debug.LogError($"[WebSocketClient] Send failed: {ex.Message}");
                OnError?.Invoke(ex.Message);
            }
        }

        /// <summary>
        /// 切断する。
        /// </summary>
        public async Task DisconnectAsync()
        {
            _cts?.Cancel();

            if (_ws?.State == WebSocketState.Open)
            {
                try
                {
                    await _ws.CloseAsync(
                        WebSocketCloseStatus.NormalClosure,
                        "Client closing",
                        CancellationToken.None);
                }
                catch (Exception ex)
                {
                    Debug.LogWarning($"[WebSocketClient] Close error: {ex.Message}");
                }
            }
        }

        private async Task ReceiveLoopAsync(CancellationToken ct)
        {
            var buffer = new byte[ReceiveBufferSize];
            var fragments = new List<byte>();

            try
            {
                while (_ws?.State == WebSocketState.Open && !ct.IsCancellationRequested)
                {
                    var result = await _ws.ReceiveAsync(
                        new ArraySegment<byte>(buffer), ct);

                    if (result.MessageType == WebSocketMessageType.Close)
                    {
                        Debug.Log("[WebSocketClient] Server closed connection");
                        await _ws.CloseAsync(
                            WebSocketCloseStatus.NormalClosure,
                            "Ack",
                            CancellationToken.None);
                        break;
                    }

                    if (result.MessageType == WebSocketMessageType.Text)
                    {
                        // フレーム断片を蓄積
                        fragments.AddRange(new ArraySegment<byte>(buffer, 0, result.Count));

                        if (result.EndOfMessage)
                        {
                            var message = Encoding.UTF8.GetString(fragments.ToArray());
                            fragments.Clear();
                            Debug.Log($"[WebSocketClient] Received: {message}");
                            OnMessageReceived?.Invoke(message);
                        }
                    }
                }
            }
            catch (OperationCanceledException)
            {
                Debug.Log("[WebSocketClient] Receive loop cancelled");
            }
            catch (WebSocketException ex)
            {
                Debug.LogWarning($"[WebSocketClient] WebSocket error: {ex.Message}");
                OnError?.Invoke(ex.Message);
            }
            catch (Exception ex)
            {
                Debug.LogError($"[WebSocketClient] Receive error: {ex.Message}");
                OnError?.Invoke(ex.Message);
            }
            finally
            {
                OnDisconnected?.Invoke("Connection closed");
                // 自動再接続（Dispose 時は除く）
                if (!_isDisposed)
                {
                    _ = ReconnectAfterDelayAsync();
                }
            }
        }

        private async Task ReconnectAfterDelayAsync()
        {
            await Task.Delay(TimeSpan.FromSeconds(ReconnectDelaySeconds));
            if (!_isDisposed)
            {
                Debug.Log("[WebSocketClient] Attempting reconnect...");
                await ConnectAsync();
            }
        }

        public void Dispose()
        {
            if (_isDisposed) return;
            _isDisposed = true;

            _cts?.Cancel();
            _cts?.Dispose();
            _cts = null;

            _ws?.Dispose();
            _ws = null;

            OnConnected = null;
            OnDisconnected = null;
            OnMessageReceived = null;
            OnError = null;
        }
    }
}
