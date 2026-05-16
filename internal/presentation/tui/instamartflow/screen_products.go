package instamartflow

import (
	"fmt"
	"strconv"
	"strings"
)

func (m instamartModel) renderSearch(sb *strings.Builder) {
	sb.WriteString(line(brandStyle.Render(" Search products")))
	sb.WriteString(line(" Type a product name, then press enter."))
	sb.WriteString(line(""))
	sb.WriteString(line(" Query: " + boldStyle.Render(m.searchQuery)))
}

func (m instamartModel) renderProducts(sb *strings.Builder) {
	sb.WriteString(line(brandStyle.Render(" Choose exact pack / variation")))
	sb.WriteString(line(" Cart updates only after this selection and quantity confirmation."))
	sb.WriteString(line(""))
	for i, row := range m.rows {
		label := fmt.Sprintf("%d. %s", i+1, productRowText(row))
		if m.cursor == i {
			sb.WriteString(line(cursorStyle.Render("> ") + boldStyle.Render(label)))
		} else {
			sb.WriteString(line("   " + label))
		}
	}
}

func (m instamartModel) renderQuantity(sb *strings.Builder) {
	if m.selectedRow == nil {
		sb.WriteString(line(" No variation selected."))
		return
	}
	sb.WriteString(line(brandStyle.Render(" Quantity")))
	sb.WriteString(line(" " + productRowText(*m.selectedRow)))
	sb.WriteString(line(""))
	sb.WriteString(line(fmt.Sprintf(" Quantity: %s", boldStyle.Render(strconv.Itoa(m.quantity)))))
	sb.WriteString(line(" Press enter to update the whole intended cart."))
	sb.WriteString(line(" Set quantity to 0 to remove this variation."))
}
