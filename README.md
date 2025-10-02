These are slide decks for presentations.

> [!NOTE]
> Slides created by using [`present`](https://pkg.go.dev/golang.org/x/tools/cmd/present) are moved to [`old` branch](https://github.com/yyh-gl/slide-decks/tree/old).


# installation

## [Aloxaf/silicon](https://github.com/Aloxaf/silicon)

`$ brew install silicon`

## [Songmu/laminate](https://github.com/Songmu/laminate)

`$ brew install songmu/tap/laminate`

config file: `~/.config/laminate/config.yaml`
```
commands:
- lang: mermaid
  run: 'mmdc -i - -o "{{output}}" --quiet'
  ext: png
- lang: '{kotlin,go,rust,python,java,javascript,typescript}'
  run: 'silicon -l "{{lang}}" --background "#ffffff" --no-window-controls --pad-horiz 0 --pad-vert 0 -o "{{output}}"'
- lang: '{ebnf}'
  run: 'silicon -l "go" --background "#ffffff" --no-window-controls --pad-horiz 0 --pad-vert 0 -o "{{output}}"'
  ext: png
- lang: '*'
  run: ['convert', '-background', 'white', '-fill', 'black', 'label:{{input}}', '{{output}}']
```
