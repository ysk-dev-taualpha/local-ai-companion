# Roadmap

## Project Goal

ローカル環境で動作する常駐AIアシスタントを段階的に開発する。

最終的には、音声会話、キャラクター表示、ユーザーの作業環境や生活情報との連携を持つアシスタントを目指す。

## Development Principle

全体を薄く作るのではなく、各段階を単体の成果物として成立する水準まで仕上げてから次へ進む。

## Milestones

### v0.1: Python Conversation Core

テキスト入力に対して、LLMが安定したJSON応答を返す会話コアを作る。

Done:

- テキスト入力で会話できる
- 応答が所定のJSON形式で返る
- JSON検証とフォールバックがある
- 会話履歴が管理される
- ログが保存される
- LLMプロバイダを差し替えられる
- モックLLMでテストできる

### v0.2: Go Runtime Minimum

Go RuntimeからPython Conversation Coreを呼び出せるようにする。

Done:

- Goサービスが起動する
- Pythonサービスへリクエストできる
- request_idを管理できる
- timeoutとcancelを扱える
- 設定ファイルを読める
- ログが残る

### v0.3: Unity Text Connection

UnityからGo Runtimeへテキストを送り、JSON応答を受け取る。

Done:

- Unity最小UIからテキスト入力できる
- Unity-Go間で通信できる
- 応答本文を表示できる
- emotion / motion / speak_style を受信できる

### v0.4: TTS Output

応答テキストを音声として出力する。

Done:

- TTSサービスを呼び出せる
- 音声再生キューを管理できる
- 再生中断ができる
- 字幕と音声を同期できる

### v0.5: Voice Input

音声入力から会話できるようにする。

Done:

- マイク入力を扱える
- 話し始めと話し終わりを検出できる
- STT結果をテキストで確認できる
- 誤認識時にキャンセルできる
- TTS音声を再入力しない

### v0.6: Character Control

応答JSONに基づき、キャラクター表示を制御する。

Done:

- emotionに応じて表情を変えられる
- motionに応じて動作を切り替えられる
- TTS音声に合わせて口パクできる
- 待機状態と発話状態を分けられる

### Later: Memory, RAG, Autonomous Behavior

会話ログ、作業メモ、PC状態、予定などを扱い、必要に応じてAI側から働きかける常駐アシスタントへ拡張する。
