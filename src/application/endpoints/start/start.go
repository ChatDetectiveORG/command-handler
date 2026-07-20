package start

import (
	"time"

	h "github.com/ChatDetectiveORG/shared/handlers"
)

func NewStartEndpoint() h.Endpoint {
	ep := h.Endpoint{}
	ep.Init(
		"start",
		*h.HandlerChain{}.Init(
			2*time.Minute,
			h.InitChainHandler(run, h.EndOnError),
		),
		h.Command([]string{"start"}),
	)
	return ep
}

func NewShowContactsEndpoint() h.Endpoint {
	ep := h.Endpoint{}
	ep.Init(
		"show_contacts",
		*h.HandlerChain{}.Init(
			30*time.Second,
			h.InitChainHandler(runShowContacts, h.EndOnError),
		),
		h.UniqueCallback(showContactsUnique),
	)
	return ep
}
