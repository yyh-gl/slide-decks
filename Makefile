.DEFAULT_GOAL := help
.PHONY: help
help: ## helpを表示
	@echo '  see:'
	@echo '   - https://github.com/yyh-gl/tech-blog'
	@echo ''
	@grep -E '^[%/a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-22s\033[0m %s\n", $$1, $$2}'

.PHONY: server
server: ## presentサーバを起動
	docker run --rm -it --name slide-decks \
	  -v `pwd`:/go/src/github.com/yyh-gl/slide-decks \
	  -p 3999:3999 \
	  slide-decks
