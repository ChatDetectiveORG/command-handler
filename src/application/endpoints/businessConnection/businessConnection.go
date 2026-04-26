package businessconnection

import (
	"time"

	h "github.com/ChatDetectiveORG/shared/handlers"
)

func NewBusinessConnectionEndpoint() h.Endpoint {
	ep := h.Endpoint{}
	ep.Init(
		"business_connection",
		*h.HandlerChain{}.Init(
			2*time.Minute,
			h.InitChainHandler(run, h.EndOnError),
		),
		h.BusinessConnectionChanged(),
	)
	return ep
}
