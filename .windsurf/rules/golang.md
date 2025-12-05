---
trigger: always_on
---

- when using air for dev do not use github.com/cosmtrek/air@latest but use github.com/air-verse/air@latest
- minimum version of golang 1.25 or later
- alwasy golangci-lint lint veg gosec
- make sure there are always valid tests ( with mocks , no need to wait too long for them )