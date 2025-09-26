package web

import "github.com/EgorLis/my-docs/internal/domain"

type Repos struct {
	Users  domain.UsersRepo
	Docs   domain.DocsRepo
	Shares domain.SharesRepo
}

type AuthDeps struct {
	Hasher    domain.PasswordHasher
	Tokens    domain.TokenManager
	Blacklist domain.TokenBlacklist
}
