---
trigger: always_on
---

# Makefile general rules
- make sure Makefile is up to date, always
- makefile use colors 
- makefile list targets
- example of codes is using <code></code> tags 
- if docker is used make sure there is a code for creating networks ( example below)

# Makefile colors 
- make file should have colors liek below code: 
<code>
ifneq (,$(findstring xterm,${TERM}))
   BLACK        := $(shell tput -Txterm setaf 0)
   RED          := $(shell tput -Txterm setaf 1)
   GREEN        := $(shell tput -Txterm setaf 2)
   YELLOW       := $(shell tput -Txterm setaf 3)
   LIGHTPURPLE  := $(shell tput -Txterm setaf 4)
   PURPLE       := $(shell tput -Txterm setaf 5)
   BLUE         := $(shell tput -Txterm setaf 6)
   WHITE        := $(shell tput -Txterm setaf 7)
   RESET := $(shell tput -Txterm sgr0)
else
   BLACK        := ""
   RED          := ""
   GREEN        := ""
   YELLOW       := ""
   LIGHTPURPLE  := ""
   PURPLE       := ""
   BLUE         := ""
   WHITE        := ""
   RESET        := ""
endif
</code>

# Makefile listing targets
- Makefile should have listing option like this:
<code>
help:
    @grep -h -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-40s\033[0m %s\n", $$1, $$2}'
</code>

# Makefile targets definitions
- each section should have two hashes to define a command for listing like this:
<code>
docker-setup-network: ## Creates required networks
docker-start: ## Start docker
</code>

- if docker is used in a project make sure there is a command to create missing networks like:
<code>
docker-setup-network: ## Creates required networks
   @echo "${BLUE}Creating docker networks(if not exists):${RESET}"
   @for NETWORK in $(DOCKER_NETWORKS) ; do \
      echo " ${GREEN}$$NETWORK${RESET}" ; \
      docker network create $$NETWORK >/dev/null 2>&1 || true ; \
   done
</code>


