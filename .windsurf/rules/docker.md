---
trigger: manual
---

- make sure you using docker 24 or later.
- if using docker use latest docker version, -id doing docker-compose yaml file attribute 'version' is obsolote in docker-compose files.
- make sure .dockerignore is up to date
- make sure docker-compose.yml is up to date, if used
- make sure docker-compose.dev.yml is up to date, if used
- docker-compose do not export all internal ports, only web ones.

whole solution is running under docker - all installations should be happening inside a containers.

to start dev please use make dev command