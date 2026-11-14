- 【データ指向アプリケーション要約】 第9章 一貫性と合意 読む
- Lincheckの内部実装についても触れたいな
  - モデル検査をどうやって実装しているか
- Lincheckで検知できない不具合もあることを共有
  - モデル検査の場合はどんなん？
    - 検査できる数には限りがある→https://kotlinlang.org/docs/lincheck-model-checking.html#deterministic-exploration
  - ストレステストの場合はどんなん？
- 線形化可能性
  - 解説スライド入れても良さそう: https://claude.ai/chat/e963c236-e4ae-471e-abd9-afd21ad58afb?artifact=2c5d9d2c-bf8d-4c35-b657-0751648cba36
- 静止一貫性（Quiescent Consistency）
  - 線形化可能性より制限を少し弱めたやつ
  - Lincheckでもサポート: https://kotlinlang.org/docs/lincheck-results-validation.html#sequential-specification
- Lincheckは確実に不具合を見つけられるわけではない
  - model checkingは作成シナリオ数、stress testは試行回数をそれぞれ1回にすると、検知できるはずの不具合を検知できないことがある
  - 不具合を見逃すリスク（偽陰性）
- Lincheckのiterationsとinvocationsの違いはなに？
- EventStructureStrategyってのも追加されようとしてる
  - https://github.com/JetBrains/lincheck/pull/1026

# 参考文献入れるときの用のメモ

- 論文
  - 『Linearizability: A Correctness Condition for Concurrent Objects 』
  - https://nikitakoval.org/publications/cav23-lincheck.pdf
- 公式Docs: https://kotlinlang.org/docs/lincheck-guide.html
- YouTube
  - https://www.youtube.com/watch?v=jZqkWfa11Js
  - https://www.youtube.com/watch?v=YNtUK9GK4pA
  - https://www.youtube.com/watch?v=EW7mkAOErWw
