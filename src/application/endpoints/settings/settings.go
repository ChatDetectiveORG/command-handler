package settings

import (
	"time"

	e "github.com/ChatDetectiveORG/shared/errors"
	h "github.com/ChatDetectiveORG/shared/handlers"
	tele "gopkg.in/telebot.v4"

	constants "github.com/ChatDetectiveORG/shared/constants"
)

func NewSettingsEndpoint() h.Endpoint {
	ep := h.Endpoint{}
	ep.Init(
		"settings",
		*h.HandlerChain{}.Init(
			30*time.Second,
			h.InitChainHandler(runShowSettings, h.EndOnError),
		),
		h.Or(
			h.Command([]string{"settings"}),
			h.TextCommand(constants.BtnSettings),
		),
	)
	return ep
}

func NewToggleDeletedEndpoint() h.Endpoint {
	return newToggleEndpoint(constants.UniqueToggleDeleted, "toggle_deleted", runToggleDeleted)
}

func NewToggleEditedEndpoint() h.Endpoint {
	return newToggleEndpoint(constants.UniqueToggleEdited, "toggle_edited", runToggleEdited)
}

func NewToggleSelfMediaEndpoint() h.Endpoint {
	return newToggleEndpoint(constants.UniqueToggleSelfMedia, "toggle_self_media", runToggleSelfMedia)
}

func NewToggleExtExportEndpoint() h.Endpoint {
	return newToggleEndpoint(constants.UniqueToggleExtExport, "toggle_ext_export", runToggleExtExport)
}

func newToggleEndpoint(unique, name string, fn func(tele.Update, *h.HandlerChainHashe) *e.ErrorInfo) h.Endpoint {
	ep := h.Endpoint{}
	ep.Init(
		name,
		*h.HandlerChain{}.Init(
			30*time.Second,
			h.InitChainHandler(fn, h.EndOnError),
		),
		h.UniqueCallback(unique),
	)
	return ep
}
