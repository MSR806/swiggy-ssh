package instamartflow

import "strings"

func (m instamartModel) View() string {
	var sb strings.Builder
	sb.WriteString(top())
	sb.WriteString(headerLine(" "+brandStyle.Render("swiggy.ssh")+creamStyle.Render(" > Instamart"), m.headerRight()))
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
	}

	sb.WriteString(line(""))
	sb.WriteString(divider())
	sb.WriteString(m.footer())
	sb.WriteString(bottom())
	return centerInViewport(sb.String(), m.viewport)
}

func (m instamartModel) headerRight() string {
	if m.screen == instamartScreenStatic {
		return creamStyle.Render("Delivering to ") + brandStyle.Render(defaultString(m.staticAddressLabel, "Home"))
	}
	if m.selectedAddress != nil {
		return creamStyle.Render("Delivering to ") + brandStyle.Render(addressLabel(*m.selectedAddress))
	}
	return mutedStyle.Render("choose address")
}

func (m instamartModel) footer() string {
	switch m.screen {
	case instamartScreenAddressSelect:
		return footerLine(KeyHint{Key: "j/k", Label: "move"}, KeyHint{Key: "1-9", Label: "select"}, KeyHint{Key: "q", Label: "quit"})
	case instamartScreenHome, instamartScreenStatic:
		return footerLine(KeyHint{Key: "/", Label: "search"}, KeyHint{Key: "c", Label: "cart"}, KeyHint{Key: "a", Label: "address"}, KeyHint{Key: "q", Label: "quit"})
	case instamartScreenSearchInput:
		return footerLine(KeyHint{Key: "enter", Label: "search"}, KeyHint{Key: "esc", Label: "home"}, KeyHint{Key: "q", Label: "quit"})
	case instamartScreenProductList:
		return footerLine(KeyHint{Key: "j/k", Label: "move"}, KeyHint{Key: "enter", Label: "choose"}, KeyHint{Key: "esc", Label: "home"})
	case instamartScreenQuantity:
		return footerLine(KeyHint{Key: "+/-", Label: "quantity"}, KeyHint{Key: "enter", Label: "update cart"}, KeyHint{Key: "esc", Label: "home"})
	case instamartScreenCartReview:
		return footerLine(KeyHint{Key: "p/enter", Label: "checkout"}, KeyHint{Key: "/", Label: "add item"}, KeyHint{Key: "b", Label: "home"})
	case instamartScreenCheckoutConfirm:
		return footerLine(KeyHint{Key: "y/enter", Label: "confirm"}, KeyHint{Key: "n", Label: "cancel"})
	default:
		return footerLine(KeyHint{Key: "enter", Label: "home"}, KeyHint{Key: "q", Label: "quit"})
	}
}
