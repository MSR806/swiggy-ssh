package instamartflow

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	appinstamart "swiggy-ssh/internal/application/instamart"
	domainauth "swiggy-ssh/internal/domain/auth"
	domaininstamart "swiggy-ssh/internal/domain/instamart"

	tea "github.com/charmbracelet/bubbletea"
)

const cancellationGuidance = "To cancel your order, please call Swiggy customer care at 080-67466729."

// InstamartService is the application boundary used by the terminal UI.
// It intentionally mirrors only the use cases this view can trigger.
type InstamartService interface {
	GetAddresses(ctx context.Context) ([]domaininstamart.Address, error)
	SearchProducts(ctx context.Context, input appinstamart.SearchProductsInput) (domaininstamart.ProductSearchResult, error)
	GetGoToItems(ctx context.Context, input appinstamart.GetGoToItemsInput) (domaininstamart.ProductSearchResult, error)
	GetCart(ctx context.Context) (domaininstamart.Cart, error)
	UpdateCart(ctx context.Context, input appinstamart.UpdateCartInput) (domaininstamart.Cart, error)
	Checkout(ctx context.Context, input appinstamart.CheckoutInput) (domaininstamart.CheckoutResult, error)
	GetOrders(ctx context.Context, input appinstamart.GetOrdersInput) (domaininstamart.OrderHistory, error)
	TrackOrder(ctx context.Context, input appinstamart.TrackOrderInput) (domaininstamart.TrackingStatus, error)
}

// InstamartPlaceholderView remains for unauthenticated/legacy fallback paths.
// Successful authenticated sessions use InstamartAppView instead.
type InstamartPlaceholderView struct {
	UserID        string
	StatusMessage string
	Viewport      Viewport
	In            io.Reader
}

func (v InstamartPlaceholderView) Render(ctx context.Context, w io.Writer) error {
	return InstamartView{
		AddressLabel:  "Home",
		AddressLine:   "Mock address hidden until Instamart is connected",
		CartItemCount: 0,
		StatusMessage: v.StatusMessage,
		Viewport:      v.Viewport,
		In:            v.In,
	}.Render(ctx, w)
}

// InstamartView renders the static Instamart landing screen used by older tests
// and fallback paths that do not have an application service.
type InstamartView struct {
	AddressLabel  string
	AddressLine   string
	CartItemCount int
	StatusMessage string
	Viewport      Viewport
	In            io.Reader
}

// InstamartAppView renders the service-backed Instamart flow.
type InstamartAppView struct {
	Service       InstamartService
	UserID        string
	StatusMessage string
	Viewport      Viewport
	In            io.Reader
}

type instamartScreen int

const (
	instamartScreenStatic instamartScreen = iota
	instamartScreenLoadingAddresses
	instamartScreenAddressSelect
	instamartScreenHome
	instamartScreenSearchInput
	instamartScreenProductList
	instamartScreenQuantity
	instamartScreenLoading
	instamartScreenCartReview
	instamartScreenCheckoutConfirm
	instamartScreenOrderResult
	instamartScreenOrders
	instamartScreenTracking
	instamartScreenMessage
)

type instamartHomeChoice struct {
	label  string
	action string
}

var instamartHomeChoices = []instamartHomeChoice{
	{label: "Search products", action: "search"},
	{label: "Your go-to items", action: "goto"},
	{label: "View cart", action: "cart"},
	{label: "Track active order", action: "track"},
	{label: "Order history", action: "orders"},
	{label: "Cancel order help", action: "cancel"},
	{label: "Change address", action: "address"},
}

type productVariationRow struct {
	Product   domaininstamart.Product
	Variation domaininstamart.ProductVariation
}

type instamartModel struct {
	ctx      context.Context
	service  InstamartService
	viewport Viewport
	screen   instamartScreen
	loading  string

	staticAddressLabel string
	staticAddressLine  string
	staticCartCount    int

	status  string
	message string
	err     string

	cursor          int
	homeCursor      int
	addresses       []domaininstamart.Address
	selectedAddress *domaininstamart.Address

	searchQuery string
	products    []domaininstamart.Product
	rows        []productVariationRow
	selectedRow *productVariationRow
	quantity    int

	currentCart   domaininstamart.Cart
	intendedItems []domaininstamart.CartUpdateItem
	paymentMethod string
	reviewedCart  *domaininstamart.CartReviewSnapshot

	checkoutResult domaininstamart.CheckoutResult
	orders         domaininstamart.OrderHistory
	tracking       domaininstamart.TrackingStatus
}

type instamartAddressesMsg struct {
	addresses []domaininstamart.Address
	err       error
}

type instamartProductsMsg struct {
	result domaininstamart.ProductSearchResult
	err    error
}

type instamartCartMsg struct {
	cart domaininstamart.Cart
	err  error
}

type instamartCheckoutMsg struct {
	result domaininstamart.CheckoutResult
	err    error
}

type instamartOrdersMsg struct {
	history domaininstamart.OrderHistory
	err     error
}

type instamartTrackingMsg struct {
	status  domaininstamart.TrackingStatus
	history domaininstamart.OrderHistory
	err     error
}

func (m instamartModel) Init() tea.Cmd {
	if m.service == nil || m.screen == instamartScreenStatic {
		return ctxQuitCmd(m.ctx)
	}
	return tea.Batch(ctxQuitCmd(m.ctx), m.loadAddressesCmd())
}

func (m instamartModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKey(msg)
	case instamartAddressesMsg:
		if msg.err != nil {
			m.screen = instamartScreenMessage
			m.err = displayErr("Could not load addresses", msg.err)
			return m, nil
		}
		m.addresses = msg.addresses
		m.cursor = 0
		m.screen = instamartScreenAddressSelect
		if len(msg.addresses) == 0 {
			m.screen = instamartScreenMessage
			m.status = "No saved Instamart addresses were found. Add an address in Swiggy first."
		}
		return m, nil
	case instamartProductsMsg:
		if msg.err != nil {
			m.screen = instamartScreenHome
			m.err = displayErr("Could not load products", msg.err)
			return m, nil
		}
		m.products = msg.result.Products
		m.rows = flattenProductRows(msg.result.Products)
		m.cursor = 0
		m.screen = instamartScreenProductList
		if len(m.rows) == 0 {
			m.screen = instamartScreenHome
			m.status = "No matching products found. Try another search."
		}
		return m, nil
	case instamartCartMsg:
		if msg.err != nil {
			m.screen = instamartScreenHome
			m.err = displayErr("Could not update cart", msg.err)
			return m, nil
		}
		m.applyCart(msg.cart)
		m.screen = instamartScreenCartReview
		m.cursor = 0
		m.status = "Cart is up to date."
		return m, nil
	case instamartCheckoutMsg:
		if msg.err != nil {
			m.screen = instamartScreenCartReview
			m.err = displayErr("Checkout blocked", msg.err)
			return m, nil
		}
		m.checkoutResult = msg.result
		m.screen = instamartScreenOrderResult
		m.status = msg.result.Message
		return m, nil
	case instamartOrdersMsg:
		if msg.err != nil {
			m.screen = instamartScreenHome
			m.err = displayErr("Could not load orders", msg.err)
			return m, nil
		}
		m.orders = msg.history
		m.screen = instamartScreenOrders
		m.cursor = 0
		return m, nil
	case instamartTrackingMsg:
		if msg.err != nil {
			if len(msg.history.Orders) > 0 {
				m.orders = msg.history
				m.cursor = 0
			}
			m.screen = instamartScreenOrders
			m.err = displayErr("Tracking unavailable", msg.err)
			return m, nil
		}
		m.tracking = msg.status
		m.screen = instamartScreenTracking
		return m, nil
	}
	return m, nil
}

func (m instamartModel) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	if key == "ctrl+c" || key == "q" {
		return m, tea.Quit
	}
	if m.screen == instamartScreenStatic {
		return m.handleStaticKey(key)
	}
	if key == "esc" {
		if m.screen == instamartScreenCheckoutConfirm {
			return m.handleCheckoutConfirmKey(key)
		}
		m.screen = instamartScreenHome
		m.err = ""
		m.status = ""
		return m, nil
	}

	switch m.screen {
	case instamartScreenAddressSelect:
		return m.handleAddressKey(key)
	case instamartScreenHome:
		return m.handleHomeKey(key)
	case instamartScreenSearchInput:
		return m.handleSearchKey(msg)
	case instamartScreenProductList:
		return m.handleProductKey(key)
	case instamartScreenQuantity:
		return m.handleQuantityKey(key)
	case instamartScreenCartReview:
		return m.handleCartReviewKey(key)
	case instamartScreenCheckoutConfirm:
		return m.handleCheckoutConfirmKey(key)
	case instamartScreenOrders:
		return m.handleOrdersKey(key)
	case instamartScreenOrderResult, instamartScreenTracking, instamartScreenMessage:
		if key == "enter" || key == "b" || key == "h" {
			m.screen = instamartScreenHome
			m.err = ""
			return m, nil
		}
	}
	return m, nil
}

func (m instamartModel) handleStaticKey(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor < len(instamartHomeChoices)-1 {
			m.cursor++
		}
	case "enter", "b":
		return m, tea.Quit
	}
	return m, nil
}

func (m instamartModel) handleAddressKey(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor < len(m.addresses)-1 {
			m.cursor++
		}
	case "enter":
		return m.selectAddress(m.cursor)
	default:
		if idx, ok := numberKeyIndex(key); ok {
			return m.selectAddress(idx)
		}
	}
	return m, nil
}

func (m instamartModel) selectAddress(idx int) (tea.Model, tea.Cmd) {
	if idx < 0 || idx >= len(m.addresses) {
		m.err = "Choose one of the saved addresses before continuing."
		return m, nil
	}
	m.selectedAddress = &m.addresses[idx]
	m.screen = instamartScreenHome
	m.homeCursor = 0
	m.status = "Delivering to " + addressLabel(m.addresses[idx])
	m.err = ""
	return m, nil
}

func (m instamartModel) handleHomeKey(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "up", "k":
		if m.homeCursor > 0 {
			m.homeCursor--
		}
	case "down", "j":
		if m.homeCursor < len(instamartHomeChoices)-1 {
			m.homeCursor++
		}
	case "/":
		return m.startSearch(), nil
	case "c":
		return m.runHomeAction("cart")
	case "a":
		m.screen = instamartScreenAddressSelect
		m.cursor = 0
		return m, nil
	case "enter":
		return m.runHomeAction(instamartHomeChoices[m.homeCursor].action)
	default:
		if idx, ok := numberKeyIndex(key); ok && idx < len(instamartHomeChoices) {
			return m.runHomeAction(instamartHomeChoices[idx].action)
		}
	}
	return m, nil
}

func (m instamartModel) runHomeAction(action string) (tea.Model, tea.Cmd) {
	switch action {
	case "search":
		return m.startSearch(), nil
	case "goto":
		if !m.hasAddress() {
			m.err = "Choose a delivery address first."
			return m, nil
		}
		m.screen = instamartScreenLoading
		m.loading = "Loading your go-to items..."
		return m, m.loadGoToItemsCmd()
	case "cart":
		if !m.hasAddress() {
			m.err = "Choose a delivery address first."
			return m, nil
		}
		return m.loadCart("Loading cart...")
	case "track":
		m.screen = instamartScreenLoading
		m.loading = "Looking for active orders..."
		return m, m.loadOrdersCmd(true)
	case "orders":
		m.screen = instamartScreenLoading
		m.loading = "Loading recent orders..."
		return m, m.loadOrdersCmd(false)
	case "cancel":
		m.screen = instamartScreenMessage
		m.message = cancellationGuidance
		m.status = ""
		m.err = ""
		return m, nil
	case "address":
		m.screen = instamartScreenAddressSelect
		m.cursor = 0
		return m, nil
	}
	return m, nil
}

func (m instamartModel) startSearch() instamartModel {
	if !m.hasAddress() {
		m.err = "Choose a delivery address first."
		return m
	}
	m.screen = instamartScreenSearchInput
	m.searchQuery = ""
	m.err = ""
	m.status = ""
	return m
}

func (m instamartModel) handleSearchKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	switch key {
	case "enter":
		query := strings.TrimSpace(m.searchQuery)
		if query == "" {
			m.err = "Type a product name before searching."
			return m, nil
		}
		m.screen = instamartScreenLoading
		m.loading = "Searching for " + query + "..."
		return m, m.searchProductsCmd(query)
	case "backspace", "ctrl+h":
		if len(m.searchQuery) > 0 {
			runes := []rune(m.searchQuery)
			m.searchQuery = string(runes[:len(runes)-1])
		}
	default:
		if msg.Type == tea.KeyRunes {
			m.searchQuery += msg.String()
		}
	}
	return m, nil
}

func (m instamartModel) handleProductKey(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor < len(m.rows)-1 {
			m.cursor++
		}
	case "enter":
		return m.selectProductRow(m.cursor)
	default:
		if idx, ok := numberKeyIndex(key); ok {
			return m.selectProductRow(idx)
		}
	}
	return m, nil
}

func (m instamartModel) selectProductRow(idx int) (tea.Model, tea.Cmd) {
	if idx < 0 || idx >= len(m.rows) {
		m.err = "Choose a listed product variation."
		return m, nil
	}
	row := m.rows[idx]
	if strings.TrimSpace(row.Variation.SpinID) == "" {
		m.err = "That product variation cannot be added from the terminal."
		return m, nil
	}
	if !row.Product.InStock || !row.Product.Available || !row.Variation.InStock {
		m.err = "That product variation is currently unavailable."
		return m, nil
	}
	m.selectedRow = &row
	m.quantity = existingQuantity(m.intendedItems, row.Variation.SpinID)
	if m.quantity <= 0 {
		m.quantity = 1
	}
	m.screen = instamartScreenQuantity
	m.err = ""
	return m, nil
}

func (m instamartModel) handleQuantityKey(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "up", "k", "+", "=":
		m.quantity++
	case "down", "j", "-":
		if m.quantity > 0 {
			m.quantity--
		}
	case "enter":
		if m.selectedRow == nil || strings.TrimSpace(m.selectedRow.Variation.SpinID) == "" {
			m.err = "Choose an exact variation before updating cart."
			return m, nil
		}
		m.screen = instamartScreenLoading
		m.loading = "Updating cart..."
		items := upsertCartItem(m.intendedItems, m.selectedRow.Variation.SpinID, m.quantity)
		return m, m.updateCartCmd(items)
	default:
		if n, err := strconv.Atoi(key); err == nil && n >= 0 {
			m.quantity = n
		}
	}
	return m, nil
}

func (m instamartModel) handleCartReviewKey(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "enter", "p":
		if len(m.currentCart.Items) == 0 {
			m.err = "Add an item before checkout."
			return m, nil
		}
		if strings.TrimSpace(m.currentCart.AddressID) != "" && strings.TrimSpace(m.currentCart.AddressID) != strings.TrimSpace(m.selectedAddressID()) {
			m.err = "Cart address no longer matches the selected address. Change address or update the cart again."
			return m, nil
		}
		if len(m.currentCart.AvailablePaymentMethods) == 0 {
			m.err = "No terminal payment method is available for this cart."
			return m, nil
		}
		m.paymentMethod = strings.TrimSpace(m.currentCart.AvailablePaymentMethods[0])
		m.reviewedCart = &domaininstamart.CartReviewSnapshot{
			AddressID:     m.selectedAddressID(),
			Items:         append([]domaininstamart.CartUpdateItem(nil), m.intendedItems...),
			ToPayRupees:   cartToPayRupees(m.currentCart),
			PaymentMethod: m.paymentMethod,
		}
		m.screen = instamartScreenCheckoutConfirm
		m.err = ""
		return m, nil
	case "s", "/":
		return m.startSearch(), nil
	case "h", "b":
		m.screen = instamartScreenHome
		return m, nil
	}
	return m, nil
}

func (m instamartModel) handleCheckoutConfirmKey(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "y", "enter":
		m.screen = instamartScreenLoading
		m.loading = "Placing Instamart order..."
		return m, m.checkoutCmd()
	case "n", "b", "esc":
		m.screen = instamartScreenCartReview
		m.status = "Checkout cancelled."
		return m, nil
	}
	return m, nil
}

func (m instamartModel) handleOrdersKey(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor < len(m.orders.Orders)-1 {
			m.cursor++
		}
	case "enter":
		if m.cursor < 0 || m.cursor >= len(m.orders.Orders) {
			m.err = "Choose an order to track."
			return m, nil
		}
		order := m.orders.Orders[m.cursor]
		if order.Location == nil {
			m.tracking = domaininstamart.TrackingStatus{}
			m.screen = instamartScreenTracking
			m.err = ""
			return m, nil
		}
		m.screen = instamartScreenLoading
		m.loading = "Tracking selected order..."
		return m, m.trackOrderCmd(order)
	case "b", "h":
		m.screen = instamartScreenHome
		return m, nil
	}
	return m, nil
}

func (m instamartModel) loadCart(loading string) (tea.Model, tea.Cmd) {
	m.screen = instamartScreenLoading
	m.loading = loading
	return m, m.loadCartCmd()
}

func (m *instamartModel) applyCart(cart domaininstamart.Cart) {
	m.currentCart = cart
	m.intendedItems = cartItemsToUpdateItems(cart.Items)
}

func (m instamartModel) hasAddress() bool {
	return m.selectedAddress != nil && strings.TrimSpace(m.selectedAddress.ID) != ""
}

func (m instamartModel) selectedAddressID() string {
	if m.selectedAddress == nil {
		return ""
	}
	return m.selectedAddress.ID
}

func (m instamartModel) loadAddressesCmd() tea.Cmd {
	return func() tea.Msg {
		addresses, err := m.service.GetAddresses(m.ctx)
		return instamartAddressesMsg{addresses: addresses, err: err}
	}
}

func (m instamartModel) searchProductsCmd(query string) tea.Cmd {
	return func() tea.Msg {
		result, err := m.service.SearchProducts(m.ctx, appinstamart.SearchProductsInput{AddressID: m.selectedAddressID(), Query: query})
		return instamartProductsMsg{result: result, err: err}
	}
}

func (m instamartModel) loadGoToItemsCmd() tea.Cmd {
	return func() tea.Msg {
		result, err := m.service.GetGoToItems(m.ctx, appinstamart.GetGoToItemsInput{AddressID: m.selectedAddressID()})
		return instamartProductsMsg{result: result, err: err}
	}
}

func (m instamartModel) loadCartCmd() tea.Cmd {
	return func() tea.Msg {
		cart, err := m.service.GetCart(m.ctx)
		return instamartCartMsg{cart: cart, err: err}
	}
}

func (m instamartModel) updateCartCmd(items []domaininstamart.CartUpdateItem) tea.Cmd {
	return func() tea.Msg {
		cart, err := m.service.UpdateCart(m.ctx, appinstamart.UpdateCartInput{SelectedAddressID: m.selectedAddressID(), Items: items})
		return instamartCartMsg{cart: cart, err: err}
	}
}

func (m instamartModel) checkoutCmd() tea.Cmd {
	return func() tea.Msg {
		result, err := m.service.Checkout(m.ctx, appinstamart.CheckoutInput{
			AddressID:     m.selectedAddressID(),
			PaymentMethod: m.paymentMethod,
			Confirmed:     true,
			ReviewedCart:  m.reviewedCart,
		})
		return instamartCheckoutMsg{result: result, err: err}
	}
}

func (m instamartModel) loadOrdersCmd(activeOnly bool) tea.Cmd {
	return func() tea.Msg {
		history, err := m.service.GetOrders(m.ctx, appinstamart.GetOrdersInput{Count: 10, ActiveOnly: activeOnly})
		if err != nil || !activeOnly || len(history.Orders) == 0 {
			return instamartOrdersMsg{history: history, err: err}
		}
		order := history.Orders[0]
		if order.Location == nil {
			return instamartOrdersMsg{history: history, err: nil}
		}
		status, trackErr := m.service.TrackOrder(m.ctx, appinstamart.TrackOrderInput{OrderID: order.OrderID, Location: order.Location})
		return instamartTrackingMsg{status: status, history: history, err: trackErr}
	}
}

func (m instamartModel) trackOrderCmd(order domaininstamart.OrderSummary) tea.Cmd {
	return func() tea.Msg {
		if order.Location == nil {
			return instamartTrackingMsg{}
		}
		status, err := m.service.TrackOrder(m.ctx, appinstamart.TrackOrderInput{OrderID: order.OrderID, Location: order.Location})
		return instamartTrackingMsg{status: status, err: err}
	}
}

func (v InstamartView) Render(ctx context.Context, w io.Writer) error {
	in := v.In
	if in == nil {
		in = strings.NewReader("")
	}
	m := instamartModel{
		ctx:                ctx,
		viewport:           v.Viewport,
		screen:             instamartScreenStatic,
		staticAddressLabel: v.AddressLabel,
		staticAddressLine:  v.AddressLine,
		staticCartCount:    v.CartItemCount,
		status:             v.StatusMessage,
	}
	p := tea.NewProgram(m, tea.WithOutput(w), tea.WithInput(in), tea.WithoutSignals())
	_, err := p.Run()
	return err
}

func (v InstamartAppView) Render(ctx context.Context, w io.Writer) error {
	ctx = domainauth.ContextWithUserID(ctx, v.UserID)
	in := v.In
	if in == nil {
		in = strings.NewReader("")
	}
	m := instamartModel{
		ctx:      ctx,
		service:  v.Service,
		viewport: v.Viewport,
		screen:   instamartScreenLoadingAddresses,
		status:   v.StatusMessage,
		quantity: 1,
	}
	if v.Service == nil {
		m.screen = instamartScreenMessage
		m.err = "Instamart is unavailable in this session."
	}
	p := tea.NewProgram(m, tea.WithOutput(w), tea.WithInput(in), tea.WithoutSignals())
	_, err := p.Run()
	return err
}

func flattenProductRows(products []domaininstamart.Product) []productVariationRow {
	rows := make([]productVariationRow, 0)
	for _, product := range products {
		for _, variation := range product.Variations {
			rows = append(rows, productVariationRow{Product: product, Variation: variation})
		}
	}
	return rows
}

func productRowText(row productVariationRow) string {
	prefix := ""
	if row.Product.Promoted {
		prefix = "Sponsored · "
	}
	name := defaultString(row.Variation.DisplayName, row.Product.DisplayName)
	pack := row.Variation.QuantityDescription
	price := row.Variation.Price.OfferPrice
	stock := ""
	if !row.Product.InStock || !row.Product.Available || !row.Variation.InStock {
		stock = " · unavailable"
	}
	if pack != "" {
		return fmt.Sprintf("%s%s · %s · Rs %d%s", prefix, name, pack, price, stock)
	}
	return fmt.Sprintf("%s%s · Rs %d%s", prefix, name, price, stock)
}

func upsertCartItem(items []domaininstamart.CartUpdateItem, spinID string, quantity int) []domaininstamart.CartUpdateItem {
	updated := make([]domaininstamart.CartUpdateItem, 0, len(items)+1)
	found := false
	for _, item := range items {
		if item.SpinID == spinID {
			if quantity > 0 {
				updated = append(updated, domaininstamart.CartUpdateItem{SpinID: spinID, Quantity: quantity})
			}
			found = true
			continue
		}
		updated = append(updated, item)
	}
	if !found && quantity > 0 {
		updated = append(updated, domaininstamart.CartUpdateItem{SpinID: spinID, Quantity: quantity})
	}
	return updated
}

func cartItemsToUpdateItems(items []domaininstamart.CartItem) []domaininstamart.CartUpdateItem {
	updateItems := make([]domaininstamart.CartUpdateItem, 0, len(items))
	for _, item := range items {
		if strings.TrimSpace(item.SpinID) == "" || item.Quantity <= 0 {
			continue
		}
		updateItems = append(updateItems, domaininstamart.CartUpdateItem{SpinID: item.SpinID, Quantity: item.Quantity})
	}
	return updateItems
}

func existingQuantity(items []domaininstamart.CartUpdateItem, spinID string) int {
	for _, item := range items {
		if item.SpinID == spinID {
			return item.Quantity
		}
	}
	return 0
}

func cartToPayRupees(cart domaininstamart.Cart) int {
	if cart.Bill.ToPayRupees > 0 {
		return cart.Bill.ToPayRupees
	}
	return cart.TotalRupees
}

func numberKeyIndex(key string) (int, bool) {
	n, err := strconv.Atoi(key)
	if err != nil || n < 1 || n > 9 {
		return 0, false
	}
	return n - 1, true
}

func addressLabel(address domaininstamart.Address) string {
	return defaultString(address.Label, defaultString(address.Category, "Saved address"))
}

func selectedAddressLabel(address *domaininstamart.Address) string {
	if address == nil {
		return "Selected address"
	}
	return addressLabel(*address)
}

func defaultString(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func redactLine(_ string) string {
	return "address hidden"
}

func displayErr(prefix string, err error) string {
	suffix := " Please try again."
	switch {
	case err == nil:
		return prefix
	case errors.Is(err, appinstamart.ErrAddressRequired):
		return prefix + ": choose a delivery address first."
	case errors.Is(err, appinstamart.ErrVariantRequired):
		return prefix + ": choose an exact product variation."
	case errors.Is(err, appinstamart.ErrCartEmpty):
		return prefix + ": your cart is empty."
	case errors.Is(err, appinstamart.ErrCheckoutRequiresReview):
		return prefix + ": review the latest cart before checkout."
	case errors.Is(err, appinstamart.ErrCheckoutRequiresConfirmation):
		return prefix + ": confirm checkout first."
	case errors.Is(err, appinstamart.ErrCheckoutAmountLimit):
		return prefix + ": cart total must be below Rs 1000 in the terminal."
	case errors.Is(err, appinstamart.ErrPaymentMethodUnavailable):
		return prefix + ": selected payment method is unavailable."
	case errors.Is(err, appinstamart.ErrTrackingLocationUnavailable):
		return prefix + ": tracking is unavailable in the terminal."
	default:
		return prefix + ":" + suffix
	}
}
