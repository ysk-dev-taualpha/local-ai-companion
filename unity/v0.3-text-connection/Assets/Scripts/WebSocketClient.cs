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
    /// WebSocket Runtime (192.168.12.112:8090) の /ws エンドポイントに接続する
    /// WebSocket クライアント。自動再接続機能付き。
    /// </summary>
    public class WebSocketClient : IDisposable
    {
        private const string DefaultUrl = "ws://192.168.12.112:8090/ws";
        private const int ReceiveBufferSize = 4096;
        private const float ReconnectDelaySeconds = 3f;

        private readonly string _url;
        private readonly string _name;
        private readonly SemaphoreSlim _sendLock = new(1, 1);
        private ClientWebSocket _ws;
        private CancellationTokenSource _cts;
        private bool _isDisposed;
        private bool _reconnectScheduled;

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

        public WebSocketClient(string url = null, string name = "ws")
        {
            _url = url ?? DefaultUrl;
            _name = name;
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

                _reconnectScheduled = false;
                Debug.Log($"[WebSocketClient:{_name}] Connected to {_url}");
                OnConnected?.Invoke();

                // バックグラウンドで受信ループ開始
                _ = ReceiveLoopAsync(_cts.Token);
            }
            catch (Exception ex)
            {
                Debug.LogWarning($"[WebSocketClient:{_name}] Connect failed: {_url}: {ex.Message}");
                OnError?.Invoke(ex.Message);
                // 自動再接続
                ScheduleReconnect();
            }
        }

        /// <summary>
        /// JSON メッセージを WebSocket で送信する。
        /// </summary>
        public async Task SendAsync(string json)
        {
            if (!IsConnected)
            {
                Debug.LogWarning($"[WebSocketClient:{_name}] Cannot send: not connected");
                OnError?.Invoke("Not connected");
                return;
            }

            try
            {
                var buffer = Encoding.UTF8.GetBytes(json);
                var ct = _cts?.Token ?? CancellationToken.None;
                await _sendLock.WaitAsync(ct);
                try
                {
                    if (!IsConnected)
                    {
                        Debug.LogWarning($"[WebSocketClient:{_name}] Cannot send: disconnected while waiting");
                        OnError?.Invoke("Not connected");
                        return;
                    }
                    await _ws.SendAsync(
                        new ArraySegment<byte>(buffer),
                        WebSocketMessageType.Text,
                        endOfMessage: true,
                        ct);
                }
                finally
                {
                    _sendLock.Release();
                }
            }
            catch (Exception ex)
            {
                Debug.LogError($"[WebSocketClient:{_name}] Send failed: {ex.Message}");
                OnError?.Invoke(ex.Message);
            }
        }

        public async Task<bool> SendBinaryAsync(byte[] data)
        {
            if (!IsConnected)
            {
                Debug.LogWarning($"[WebSocketClient:{_name}] Cannot send binary: not connected");
                OnError?.Invoke("Not connected");
                return false;
            }

            try
            {
                var ct = _cts?.Token ?? CancellationToken.None;
                await _sendLock.WaitAsync(ct);
                try
                {
                    if (!IsConnected)
                    {
                        Debug.LogWarning($"[WebSocketClient:{_name}] Cannot send binary: disconnected while waiting");
                        OnError?.Invoke("Not connected");
                        return false;
                    }
                    await _ws.SendAsync(
                        new ArraySegment<byte>(data),
                        WebSocketMessageType.Binary,
                        endOfMessage: true,
                        ct);
                }
                finally
                {
                    _sendLock.Release();
                }
                return true;
            }
            catch (Exception ex)
            {
                Debug.LogError($"[WebSocketClient:{_name}] Binary send failed: {ex.Message}");
                OnError?.Invoke(ex.Message);
                return false;
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
                        Debug.Log($"[WebSocketClient:{_name}] Server closed connection");
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
                            Debug.Log($"[WebSocketClient:{_name}] Received: {message}");
                            OnMessageReceived?.Invoke(message);
                        }
                    }
                }
            }
            catch (OperationCanceledException)
            {
                Debug.Log($"[WebSocketClient:{_name}] Receive loop cancelled");
            }
            catch (WebSocketException ex)
            {
                Debug.LogWarning($"[WebSocketClient:{_name}] WebSocket error: {ex.Message}");
                OnError?.Invoke(ex.Message);
            }
            catch (Exception ex)
            {
                Debug.LogError($"[WebSocketClient:{_name}] Receive error: {ex.Message}");
                OnError?.Invoke(ex.Message);
            }
            finally
            {
                OnDisconnected?.Invoke("Connection closed");
                // 自動再接続（Dispose 時は除く）
                if (!_isDisposed)
                {
                    ScheduleReconnect();
                }
            }
        }

        private void ScheduleReconnect()
        {
            if (_isDisposed || _reconnectScheduled)
                return;
            _reconnectScheduled = true;
            _ = ReconnectAfterDelayAsync();
        }

        private async Task ReconnectAfterDelayAsync()
        {
            await Task.Delay(TimeSpan.FromSeconds(ReconnectDelaySeconds));
            if (!_isDisposed)
            {
                _reconnectScheduled = false;
                Debug.Log($"[WebSocketClient:{_name}] Attempting reconnect to {_url}...");
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
            _sendLock.Dispose();

            OnConnected = null;
            OnDisconnected = null;
            OnMessageReceived = null;
            OnError = null;
        }
    }
}
