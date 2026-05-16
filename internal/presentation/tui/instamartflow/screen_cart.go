package instamartflow

import (
	"fmt"
	"strings"

	domaininstamart "swiggy-ssh/internal/domain/instamart"
)

func (m instamartModel) renderCartReview(sb *strings.Builder) {
	cart := m.currentCart
	sb.WriteString(line(brandStyle.Render(" review staged cart")))
	sb.WriteString(line(" deploying to: " + boldStyle.Render(defaultString(cart.AddressLabel, selectedAddressLabel(m.selectedAddress))) + creamStyle.Render(" — "+redactLine(cart.AddressDisplayLine))))
	if len(cart.StoreIDs) > 1 {
		sb.WriteString(line(" " + accentStyle.Render(fmt.Sprintf("warn: cart spans %d stores. Swiggy may split deploy.", len(cart.StoreIDs)))))
	}
	sb.WriteString(line(""))
	if len(cart.Items) == 0 {
		sb.WriteString(line(" working tree clean. cart empty."))
	} else {
		for _, item := range cart.Items {
			sb.WriteString(diffLine("+", fmt.Sprintf("%-3s %-38s Rs %d", fmt.Sprintf("%dx", item.Quantity), item.Name, item.FinalPrice)))
		}
	}
	sb.WriteString(line(""))
	for _, bill := range cart.Bill.Lines {
		sign := billLineSign(bill)
		sb.WriteString(diffLine(sign, fmt.Sprintf("%-43s %s", bill.Label, strings.TrimPrefix(strings.TrimSpace(bill.Value), sign))))
	}
	toPayLabel := defaultString(cart.Bill.ToPayLabel, "To Pay")
	toPayValue := cart.Bill.ToPayValue
	if toPayValue == "" && cart.TotalRupees > 0 {
		toPayValue = fmt.Sprintf("Rs %d", cart.TotalRupees)
	}
	sb.WriteString(diffLine("+", boldStyle.Render(fmt.Sprintf("%-43s %s", toPayLabel, toPayValue))))
	sb.WriteString(line(" Payment methods: " + strings.Join(cart.AvailablePaymentMethods, ", ")))
}

func diffLine(sign, body string) string {
	marker := successStyle.Render(sign)
	if sign == "-" {
		marker = errorStyle.Render(sign)
	}
	return line(" " + marker + " " + body)
}

func billLineSign(bill domaininstamart.BillLine) string {
	value := strings.TrimSpace(bill.Value)
	if strings.HasPrefix(value, "-") {
		return "-"
	}
	label := strings.ToLower(bill.Label)
	for _, token := range []string{"discount", "saving", "coupon", "offer", "cashback"} {
		if strings.Contains(label, token) {
			return "-"
		}
	}
	return "+"
}

func (m instamartModel) renderCheckoutConfirm(sb *strings.Builder) {
	sb.WriteString(line(brandStyle.Render(" ship order")))
	sb.WriteString(line(" This confirms a real Swiggy Instamart order."))
	sb.WriteString(line(""))
	sb.WriteString(line(okLine("address selected", m.hasAddress())))
	sb.WriteString(line(okLine("cart reviewed", m.reviewedCart != nil)))
	sb.WriteString(line(okLine("payment method available", len(m.currentCart.AvailablePaymentMethods) > 0)))
	sb.WriteString(line(okLine("amount below test limit", cartToPayRupees(m.currentCart) < 1000)))
	sb.WriteString(line(""))
	sb.WriteString(line(" deploying to: " + boldStyle.Render(defaultString(m.currentCart.AddressLabel, selectedAddressLabel(m.selectedAddress))) + creamStyle.Render(" — "+redactLine(m.currentCart.AddressDisplayLine))))
	sb.WriteString(line(" payment: " + boldStyle.Render(m.paymentMethod)))
	sb.WriteString(line(fmt.Sprintf(" total: %s", boldStyle.Render(fmt.Sprintf("Rs %d", cartToPayRupees(m.currentCart))))))
	if len(m.currentCart.StoreIDs) > 1 {
		sb.WriteString(line(" " + accentStyle.Render(fmt.Sprintf("warn: cart spans %d stores. Swiggy may split deploy.", len(m.currentCart.StoreIDs)))))
	}
	sb.WriteString(line(""))
	sb.WriteString(line(" press y to confirm order"))
	sb.WriteString(line(" aka git push --force groceries"))
}

func (m instamartModel) renderOrderResult(sb *strings.Builder) {
	sb.WriteString(line(brandStyle.Render(" deploy logs")))
	sb.WriteString(line(" [ok] git push --force origin groceries"))
	message := m.checkoutResult.Message
	if message == "" {
		message = "Checkout completed."
	}
	sb.WriteString(line(" " + successStyle.Render("[ok] "+message)))
	if m.checkoutResult.Status != "" {
		sb.WriteString(line(" [ok] status: " + m.checkoutResult.Status))
	}
	if m.checkoutResult.PaymentMethod != "" {
		sb.WriteString(line(" [ok] payment method: " + m.checkoutResult.PaymentMethod))
	}
	for _, orderID := range m.checkoutResult.OrderIDs {
		sb.WriteString(line(" [info] order_id=" + orderID))
	}
	stores := receiptStoreCount(m.checkoutResult)
	if stores > 0 {
		sb.WriteString(line(fmt.Sprintf(" [info] stores=%d", stores)))
	}
	if m.checkoutResult.CartTotal > 0 {
		sb.WriteString(line(fmt.Sprintf(" [info] total=Rs %d", m.checkoutResult.CartTotal)))
	}
	if m.checkoutElapsed > 0 {
		sb.WriteString(line(" [info] deployed_in=" + formatElapsed(m.checkoutElapsed)))
	}
}

func okLine(label string, ok bool) string {
	if ok {
		return "[ok] " + label
	}
	return "[wait] " + label
}

func receiptStoreCount(result domaininstamart.CheckoutResult) int {
	if result.MultiStore && len(result.OrderIDs) > 0 {
		return len(result.OrderIDs)
	}
	if result.MultiStore {
		return 2
	}
	if len(result.OrderIDs) > 0 {
		return 1
	}
	return 0
}
