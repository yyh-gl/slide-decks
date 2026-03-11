# MangaScope における SupervisorJob の特徴

## 1. SupervisorJob とは

通常の `Job` では、**子コルーチンが1つでも失敗すると、他の全子コルーチンがキャンセルされる**。`SupervisorJob` はこの伝播を止め、**子の失敗が兄弟に波及しない**。

```
// 通常の Job
Parent Job
├── Child A (失敗) → Parent も失敗 → Child B もキャンセル
└── Child B

// SupervisorJob
Parent SupervisorJob
├── Child A (失敗) → Parent は影響なし → Child B は継続
└── Child B
```

## 2. MangaScope での使われ方

**86行目**: `private val supervisorJob = SupervisorJob()`

**94行目**: `Dispatchers.IO + threadLocalContext() + customContext + supervisorJob`

ポイント:

- **独立した並行タスクの耐障害性** — `asyncWithMetrics` / `launchWithMetrics` で起動される複数タスクが互いに影響しない。例えば「お気に入り取得」が失敗しても「ホーム取得」は続行できる
- **親なしのルートJob** — `SupervisorJob()` に親Jobを渡していないため、このScope自体がルートになる。外部からのキャンセル伝播を受けない
- **Graceful shutdown の制御点** — `close()` で `supervisorJob.children` を走査し、全子タスクの完了を待ってから `supervisorJob.cancel()` を呼ぶ（101〜142行目）

## 3. shutdown シーケンスの設計

```
① shuttingDown = true        → 新規タスク受付を拒否（coroutineContext getter で error）
② children を1つずつ join    → 実行中タスクの完了を待機（タイムアウト付き）
③ supervisorJob.cancel()     → 残存タスクに一括キャンセル通知
④ supervisorJob.onJoin       → キャンセル完了を待機（タイムアウト付き）
```

これは **graceful shutdown パターン** そのもので、SupervisorJob だからこそ「一部の子がキャンセルに応じなくても他の子は正常終了できる」。

## 4. 発表で強調すべき点

| 観点 | 内容 |
|------|------|
| **なぜ SupervisorJob か** | API並列呼び出しで1つの失敗が全体を壊さないため |
| **通常Jobとの違い** | 子→親へのエラー伝播がない（兄弟も巻き込まない） |
| **注意点** | 子の例外は自動で処理されない。`CoroutineExceptionHandler` か各子での try-catch が必要（このコードでは `runWithStats` 内の try-catch で対応: 253行目） |
| **shutdown設計** | SupervisorJob を cancel しても子は CancellationException を受け取るだけ。suspension point がないと止まらない |
| **親Jobなし** | `SupervisorJob()` に引数なし = 構造化並行性の最上位。ライフサイクルを自分で管理する責任がある（`close()` で実装済み） |

## 5. よくある落とし穴（発表ネタ向き）

`SupervisorJob` を使っても `coroutineScope { }` 内で `launch` すると通常Jobの挙動になる。SupervisorJob の効果は **直接の子** にのみ適用される点は、発表で触れると実践的な学びになる。
