package instamartflow

import (
	"strconv"
	"strings"
)

func (m instamartModel) View() string {
	var sb strings.Builder
	sb.WriteString(top())
	sb.WriteString(headerLine(" "+brandStyle.Render("swiggy.ssh")+creamStyle.Render(" > Instamart"), m.headerRight()))
	sb.WriteString(line(" " + mutedStyle.Render(m.sessionStatus())))
	sb.WriteString(divider())
	sb.WriteString(line(" " + brandStyle.Render("Instamart") + creamStyle.Render(" — groceries and daily essentials in minutes.")))
	if m.status != "" {
		sb.WriteString(line(" " + successStyle.Render("✓ "+m.status)))
	}
	if m.err != "" {
		sb.WriteString(line(" " + errorStyle.Render(m.err)))
	}
	sb.WriteString(line(""))

	switch m.screen {
	case instamartScreenStatic:
		m.renderStatic(&sb)
	case instamartScreenLoadingAddresses:
		sb.WriteString(line(" Loading saved delivery addresses..."))
	case instamartScreenAddressSelect:
		m.renderAddresses(&sb)
	case instamartScreenHome:
		m.renderHome(&sb)
	case instamartScreenSearchInput:
		m.renderSearch(&sb)
	case instamartScreenProductList:
		m.renderProducts(&sb)
	case instamartScreenQuantity:
		m.renderQuantity(&sb)
	case instamartScreenLoading:
		sb.WriteString(line(" " + m.loading))
	case instamartScreenCartReview:
		m.renderCartReview(&sb)
	case instamartScreenCheckoutConfirm:
		m.renderCheckoutConfirm(&sb)
	case instamartScreenOrderResult:
		m.renderOrderResult(&sb)
	case instamartScreenOrders:
		m.renderOrders(&sb)
	case instamartScreenTracking:
		m.renderTracking(&sb)
	case instamartScreenMessage:
		message := m.message
		if message == "" {
			message = m.status
		}
		sb.WriteString(line(" " + message))
	case instamartScreenHelp:
		m.renderHelp(&sb)
	}

	sb.WriteString(line(""))
	sb.WriteString(divider())
	sb.WriteString(m.footer())
	sb.WriteString(bottom())
	return centerInViewport(sb.String(), m.viewport)
}

func (m instamartModel) headerRight() string {
	if m.screen == instamartScreenStatic {
		return creamStyle.Render("deploying to ") + brandStyle.Render(defaultString(m.staticAddressLabel, "Home"))
	}
	if m.selectedAddress != nil {
		return creamStyle.Render("deploying to ") + brandStyle.Render(addressLabel(*m.selectedAddress))
	}
	return mutedStyle.Render("choose address")
}

func (m instamartModel) sessionStatus() string {
	return "env=instamart  auth=ok  addr=" + m.sessionAddressLabel() + "  cart=" + strconv.Itoa(m.sessionCartCount()) + "  mode=" + m.screenMode()
}

func (m instamartModel) sessionAddressLabel() string {
	if m.screen == instamartScreenStatic {
		return defaultString(m.staticAddressLabel, "Home")
	}
	if m.selectedAddress != nil {
		return addressLabel(*m.selectedAddress)
	}
	return "unset"
}

func (m instamartModel) sessionCartCount() int {
	if m.screen == instamartScreenStatic {
		return m.staticCartCount
	}
	count := 0
	for _, item := range m.intendedItems {
		if item.Quantity > 0 {
			count += item.Quantity
		}
	}
	if count > 0 {
		return count
	}
	for _, item := range m.currentCart.Items {
		if item.Quantity > 0 {
			count += item.Quantity
		}
	}
	return count
}

func (m instamartModel) screenMode() string {
	switch m.screen {
	case instamartScreenStatic, instamartScreenHome:
		return "home"
	case instamartScreenLoadingAddresses, instamartScreenAddressSelect:
		return "address"
	case instamartScreenSearchInput, instamartScreenProductList:
		return "grep"
	case instamartScreenQuantity:
		return "stage"
	case instamartScreenLoading:
		return "loading"
	case instamartScreenCartReview:
		return "cart"
	case instamartScreenCheckoutConfirm:
		return "ship"
	case instamartScreenOrderResult:
		return "logs"
	case instamartScreenOrders:
		return "history"
	case instamartScreenTracking:
		return "tail"
	case instamartScreenHelp:
		return "help"
	default:
		return "message"
	}
}

func (m instamartModel) footer() string {
	switch m.screen {
	case instamartScreenAddressSelect:
		return footerLine(KeyHint{Key: "j/k", Label: "move"}, KeyHint{Key: "1-9", Label: "select"}, KeyHint{Key: "?", Label: "help"}, KeyHint{Key: "q", Label: "quit"})
	case instamartScreenHome, instamartScreenStatic:
		return footerLine(KeyHint{Key: "j/k", Label: "move"}, KeyHint{Key: "1-7", Label: "select"}, KeyHint{Key: "/", Label: "grep"}, KeyHint{Key: "c", Label: "cart"}, KeyHint{Key: "?", Label: "help"}, KeyHint{Key: "q", Label: "quit"})
	case instamartScreenSearchInput:
		return footerLine(KeyHint{Key: "enter", Label: "open results"}, KeyHint{Key: "esc", Label: "home"}, KeyHint{Key: "?", Label: "help"}, KeyHint{Key: "q", Label: "quit"})
	case instamartScreenProductList:
		return footerLine(KeyHint{Key: "j/k", Label: "move"}, KeyHint{Key: "1-9", Label: "choose"}, KeyHint{Key: "enter", Label: "choose"}, KeyHint{Key: "?", Label: "help"}, KeyHint{Key: "esc", Label: "home"})
	case instamartScreenQuantity:
		return footerLine(KeyHint{Key: "+/-", Label: "quantity"}, KeyHint{Key: "enter", Label: "stage"}, KeyHint{Key: "?", Label: "help"}, KeyHint{Key: "esc", Label: "home"})
	case instamartScreenCartReview:
		return footerLine(KeyHint{Key: "p/enter", Label: "ship"}, KeyHint{Key: "/", Label: "grep"}, KeyHint{Key: "?", Label: "help"}, KeyHint{Key: "b", Label: "home"})
	case instamartScreenCheckoutConfirm:
		return footerLine(KeyHint{Key: "y", Label: "confirm order"}, KeyHint{Key: "?", Label: "help"}, KeyHint{Key: "n", Label: "cancel"})
	case instamartScreenOrders:
		return footerLine(KeyHint{Key: "j/k", Label: "move"}, KeyHint{Key: "enter", Label: "tail"}, KeyHint{Key: "?", Label: "help"}, KeyHint{Key: "b", Label: "home"})
	case instamartScreenHelp:
		return footerLine(KeyHint{Key: "?", Label: "back"}, KeyHint{Key: "b", Label: "back"}, KeyHint{Key: "q", Label: "quit"})
	default:
		return footerLine(KeyHint{Key: "enter", Label: "home"}, KeyHint{Key: "?", Label: "help"}, KeyHint{Key: "q", Label: "quit"})
	}
}
