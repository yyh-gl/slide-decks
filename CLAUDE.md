# slide-decks

プレゼンテーション用スライドデッキのリポジトリ。

## 構造

```
slides/<YYMMDD_タイトル>/
  deck.md    # スライド本体（Deckset形式）
  images/    # 画像素材
  docs/      # 参考資料
```

## コマンド

- `make start title=<YYMMDD_タイトル>` - Decksetでスライドを開いてwatch
- `npm run lint` - textlintで全スライドをチェック

## Lint

textlint + `preset-ja-technical-writing` を使用。設定は `.textlintrc` を参照。

## CI

push時にtextlintが実行される（`.github/workflows/review.yml`）。

## ツール

- [Deckset](https://www.deckset.com/) - スライド表示
- [silicon](https://github.com/Aloxaf/silicon) - コードスニペット画像生成
- [laminate](https://github.com/Songmu/laminate) - コードブロック画像変換（mermaid, kotlin, go等）
