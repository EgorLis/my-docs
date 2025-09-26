package doc

import (
	"log"

	"github.com/EgorLis/my-docs/internal/domain"
)

type Handler struct {
	Log     *log.Logger
	Users   domain.UsersRepo
	Docs    domain.DocsRepo
	Shares  domain.SharesRepo
	Storage domain.BlobStorage
	Cache   domain.Cache

	ListTTL int // секунд
	DocTTL  int // секунд
}
