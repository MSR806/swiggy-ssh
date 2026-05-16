package instamartflow

import (
	"fmt"
	"strings"
)

func (m instamartModel) renderCartReview(sb *strings.Builder) {
	cart := m.currentCart
	sb.WriteString(line(brandStyle.Render(" Review cart")))
	sb.WriteString(line(" Address: " + boldStyle.Render(defaultString(cart.AddressLabel, selectedAddressLabel(m.selectedAddress))) + creamStyle.Render(" — "+redactLine(cart.AddressDisplayLine))))
	if len(cart.StoreIDs) > 1 {
		sb.WriteString(line(" " + accentStyle.Render(fmt.Sprintf("Multi-store cart: %d stores. Swiggy will split orders automatically.", len(cart.StoreIDs)))))
	}
	sb.WriteString(line(""))
	if len(cart.Items) == 0 {
		sb.WriteString(line(" Cart is empty."))
	} else {
		for _, item := range cart.Items {
			sb.WriteString(line(fmt.Sprintf(" %dx %s · Rs %d", item.Quantity, item.Name, item.FinalPrice)))
		}
	}
	sb.WriteString(line(""))
	for _, bill := range cart.Bill.Lines {
		sb.WriteString(line(fmt.Sprintf(" %s: %s", bill.Label, bill.Value)))
	}
	toPayLabel := defaultString(cart.Bill.ToPayLabel, "To Pay")
	toPayValue := cart.Bill.ToPayValue
	if toPayValue == "" && cart.TotalRupees > 0 {
		toPayValue = fmt.Sprintf("Rs %d", cart.TotalRupees)
	}
	sb.WriteString(line(" " + boldStyle.Render(toPayLabel+": "+toPayValue)))
	sb.WriteString(line(" Payment methods: " + strings.Join(cart.AvailablePaymentMethods, ", ")))
}

func (m instamartModel) renderCheckoutConfirm(sb *strings.Builder) {
	sb.WriteString(line(brandStyle.Render(" Confirm checkout")))
	sb.WriteString(line(" Press y or enter to place this Instamart order."))
	sb.WriteString(line(" Address: " + boldStyle.Render(defaultString(m.currentCart.AddressLabel, selectedAddressLabel(m.selectedAddress))) + creamStyle.Render(" — "+redactLine(m.currentCart.AddressDisplayLine))))
	if len(m.currentCart.Items) > 0 {
		for _, item := range m.currentCart.Items {
			sb.WriteString(line(fmt.Sprintf(" %dx %s · Rs %d", item.Quantity, item.Name, item.FinalPrice)))
		}
	}
	for _, bill := range m.currentCart.Bill.Lines {
		sb.WriteString(line(fmt.Sprintf(" %s: %s", bill.Label, bill.Value)))
	}
	sb.WriteString(line(" Payment methods: " + strings.Join(m.currentCart.AvailablePaymentMethods, ", ")))
	sb.WriteString(line(" Selected payment: " + boldStyle.Render(m.paymentMethod)))
	sb.WriteString(line(fmt.Sprintf(" Total: %s", boldStyle.Render(fmt.Sprintf("Rs %d", cartToPayRupees(m.currentCart))))))
	if len(m.currentCart.StoreIDs) > 1 {
		sb.WriteString(line(" " + accentStyle.Render("Multi-store warning acknowledged before checkout.")))
	}
}

func (m instamartModel) renderOrderResult(sb *strings.Builder) {
	sb.WriteString(line(brandStyle.Render(" Order result")))
	message := m.checkoutResult.Message
	if message == "" {
		message = "Checkout completed."
	}
	sb.WriteString(line(" " + successStyle.Render(message)))
	if m.checkoutResult.Status != "" {
		sb.WriteString(line(" Status: " + m.checkoutResult.Status))
	}
	if m.checkoutResult.PaymentMethod != "" {
		sb.WriteString(line(" Payment: " + m.checkoutResult.PaymentMethod))
	}
}
