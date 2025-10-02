.DEFAULT_GOAL := help
.PHONY: help
help: ## helpを表示
	@echo '  see:'
	@echo '   - https://github.com/yyh-gl/slide-decks'
	@echo ''
	@grep -E '^[%/a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-22s\033[0m %s\n", $$1, $$2}'

.PHONY: start
start: ## 作業環境を起動
	deck open ./slides/${title}/deck.md
	deck apply --watch ./slides/${title}/deck.md
