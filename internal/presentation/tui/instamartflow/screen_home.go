package instamartflow

import (
	"fmt"
	"strings"
)

func (m instamartModel) renderStatic(sb *strings.Builder) {
	sb.WriteString(line(" " + brandStyle.Render("Address:") + " " + boldStyle.Render(defaultString(m.staticAddressLabel, "Home")) + creamStyle.Render(" — "+redactLine(m.staticAddressLine))))
	sb.WriteString(line(" " + brandStyle.Render("Cart:") + " " + creamStyle.Render(fmt.Sprintf("%d items", m.staticCartCount))))
	sb.WriteString(line(""))
	for i, choice := range instamartHomeChoices[:5] {
		label := fmt.Sprintf("%d. %s", i+1, choice.label)
		if m.cursor == i {
			sb.WriteString(line(cursorStyle.Render("> ") + boldStyle.Render(label)))
		} else {
			sb.WriteString(line("   " + label))
		}
	}
}

func (m instamartModel) renderAddresses(sb *strings.Builder) {
	sb.WriteString(line(brandStyle.Render(" select deployment address")))
	sb.WriteString(line(""))
	for i, address := range m.addresses {
		label := fmt.Sprintf("%d. %s", i+1, addressLabel(address))
		if address.PhoneMasked != "" {
			label += " · " + address.PhoneMasked
		}
		lineText := label + " — " + redactLine(address.DisplayLine)
		if m.cursor == i {
			sb.WriteString(line(cursorStyle.Render("> ") + boldStyle.Render(lineText)))
		} else {
			sb.WriteString(line("   " + lineText))
		}
	}
}

func (m instamartModel) renderHome(sb *strings.Builder) {
	if m.selectedAddress == nil {
		sb.WriteString(line(" Choose an address before searching or checkout."))
	} else {
		sb.WriteString(line(" " + brandStyle.Render("deploying to:") + " " + boldStyle.Render(addressLabel(*m.selectedAddress)) + creamStyle.Render(" — "+redactLine(m.selectedAddress.DisplayLine))))
	}
	sb.WriteString(line(" " + brandStyle.Render("Cart:") + " " + creamStyle.Render(fmt.Sprintf("%d items", len(m.intendedItems)))))
	sb.WriteString(line(""))
	for i, choice := range instamartHomeChoices {
		label := fmt.Sprintf("%d. %s", i+1, choice.label)
		if m.homeCursor == i {
			sb.WriteString(line(cursorStyle.Render("> ") + boldStyle.Render(label)))
		} else {
			sb.WriteString(line("   " + label))
		}
	}
}

func (m instamartModel) renderHelp(sb *strings.Builder) {
	sb.WriteString(line(brandStyle.Render(" swiggy.dev keys")))
	sb.WriteString(line(""))
	sb.WriteString(line(" j/k        move"))
	sb.WriteString(line(" /          grep products"))
	sb.WriteString(line(" c          staged cart"))
	sb.WriteString(line(" enter      choose"))
	sb.WriteString(line(" +/-        change quantity"))
	sb.WriteString(line(" p          ship from cart"))
	sb.WriteString(line(" b          back home"))
	sb.WriteString(line(" q          quit"))
}
