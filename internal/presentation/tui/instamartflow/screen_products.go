package instamartflow

import (
	"fmt"
	"strconv"
	"strings"
)

func (m instamartModel) renderSearch(sb *strings.Builder) {
	title := "grep products"
	if strings.TrimSpace(m.searchQuery) != "" {
		title += ": " + m.searchQuery
	}
	sb.WriteString(line(brandStyle.Render(" " + title)))
	sb.WriteString(line(" Type query, press enter to open results."))
	sb.WriteString(line(""))
	sb.WriteString(line(" query: " + boldStyle.Render(m.searchQuery) + cursorStyle.Render("_")))

	if m.searchPreviewLoading {
		frame := searchSpinnerFrames[m.searchPreviewSpinner%len(searchSpinnerFrames)]
		sb.WriteString(line(""))
		sb.WriteString(line(" " + frame + " scanning index..."))
		return
	}
	if m.searchPreviewErr != "" {
		sb.WriteString(line(""))
		sb.WriteString(line(" " + errorStyle.Render(m.searchPreviewErr)))
		return
	}
	if !m.searchPreviewLoaded || m.searchPreviewQuery != m.searchQuery {
		return
	}

	sb.WriteString(line(""))
	sb.WriteString(line(fmt.Sprintf(" preview · matched %d products in %s", len(m.searchPreviewRows), formatElapsed(m.searchPreviewElapsed))))
	if len(m.searchPreviewRows) == 0 {
		sb.WriteString(line(" No matching products found yet."))
		return
	}
	renderProductTable(sb, m.searchPreviewRows, 0, 5)
	if len(m.searchPreviewRows) > 5 {
		sb.WriteString(line(fmt.Sprintf(" ...and %d more", len(m.searchPreviewRows)-5)))
	}
}

func (m instamartModel) renderProducts(sb *strings.Builder) {
	title := "grep products"
	if strings.TrimSpace(m.searchQuery) != "" {
		title += ": " + m.searchQuery
	} else {
		title = "recent cache"
	}
	sb.WriteString(line(brandStyle.Render(" " + title)))
	sb.WriteString(line(" Choose exact pack. Cart changes after quantity confirmation."))
	sb.WriteString(line(""))
	renderProductTable(sb, m.rows, m.cursor, 0)
}

func renderProductTable(sb *strings.Builder, rows []productVariationRow, cursor, limit int) {
	sb.WriteString(line("   #   code  item                         pack      price"))
	for i, row := range rows {
		if limit > 0 && i >= limit {
			break
		}
		label := productTableRow(i, row)
		if i == cursor {
			sb.WriteString(line(cursorStyle.Render("> ") + boldStyle.Render(label)))
		} else {
			sb.WriteString(line("   " + label))
		}
	}
}

func productTableRow(index int, row productVariationRow) string {
	name := defaultString(row.Variation.DisplayName, row.Product.DisplayName)
	if row.Product.Promoted {
		name = "Sponsored " + name
	}
	pack := defaultString(row.Variation.QuantityDescription, "-")
	status := "200"
	price := fmt.Sprintf("Rs %d", row.Variation.Price.OfferPrice)
	if !productRowAvailable(row) {
		status = "409"
		price = "out of stock"
	}
	return fmt.Sprintf("%-3d %-5s %-28s %-9s %s", index+1, status, truncateTerminal(name, 28), truncateTerminal(pack, 9), price)
}

func (m instamartModel) renderQuantity(sb *strings.Builder) {
	if m.selectedRow == nil {
		sb.WriteString(line(" No variation selected."))
		return
	}
	sb.WriteString(line(brandStyle.Render(" stage item")))
	status := "200 available"
	if !productRowAvailable(*m.selectedRow) {
		status = "409 unavailable"
	}
	sb.WriteString(line(fmt.Sprintf(" item: %s", defaultString(m.selectedRow.Variation.DisplayName, m.selectedRow.Product.DisplayName))))
	sb.WriteString(line(fmt.Sprintf(" pack: %s", defaultString(m.selectedRow.Variation.QuantityDescription, "-"))))
	sb.WriteString(line(fmt.Sprintf(" price: Rs %d", m.selectedRow.Variation.Price.OfferPrice)))
	sb.WriteString(line(" status: " + status))
	sb.WriteString(line(" action: stage item"))
	sb.WriteString(line(fmt.Sprintf(" quantity: %s", boldStyle.Render(strconv.Itoa(m.quantity)))))
	sb.WriteString(line(""))
	sb.WriteString(line(" Press enter to update the whole intended cart."))
	sb.WriteString(line(" Set quantity to 0 to remove this variation."))
}

func productRowAvailable(row productVariationRow) bool {
	return row.Product.InStock && row.Product.Available && row.Variation.InStock
}

func truncateTerminal(value string, width int) string {
	runes := []rune(value)
	if len(runes) <= width {
		return value
	}
	if width <= 3 {
		return string(runes[:width])
	}
	return string(runes[:width-3]) + "..."
}
