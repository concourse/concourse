
bootstrap: ## Install build and development dependencies


clean:


prep: ## Build the dynamically generated source files
# generate: ## Generate dynamically generated code
# generate-check: ## Check go code generation is on par


vet: ## Vet Go code


check: ## Lint the source code
# fmt: ## Format Go code
# fmt-check: fmt ## Check go code formatting


test: ## Run unit tests
# testflight:
# wats:

tidy:


dev: ## Build and install a development build


install-fly:

help:
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'
