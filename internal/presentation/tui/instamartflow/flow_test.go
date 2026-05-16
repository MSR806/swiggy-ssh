package instamartflow

import (
	"context"
	"strings"
	"testing"

	appinstamart "swiggy-ssh/internal/application/instamart"
	domaininstamart "swiggy-ssh/internal/domain/instamart"
)

func TestInstamartAddressSelectionRequiredBeforeSearch(t *testing.T) {
	m := instamartModel{screen: instamartScreenHome}
	updated, cmd := m.handleHomeKey("/")
	if cmd != nil {
		t.Fatal("search without address should not call service")
	}
	got := updated.(instamartModel)
	if got.screen != instamartScreenHome {
		t.Fatalf("expected home screen, got %v", got.screen)
	}
	if !strings.Contains(got.err, "Choose a delivery address") {
		t.Fatalf("expected address error, got %q", got.err)
	}
}

func TestInstamartSearchUsesSelectedAddress(t *testing.T) {
	fake := &fakeInstamartService{}
	address := domaininstamart.Address{ID: "addr-1", Label: "Home"}
	m := instamartModel{ctx: context.Background(), service: fake, selectedAddress: &address}

	msg := m.searchProductsCmd("milk")()
	if _, ok := msg.(instamartProductsMsg); !ok {
		t.Fatalf("expected products message, got %T", msg)
	}
	if fake.searchInput.AddressID != "addr-1" || fake.searchInput.Query != "milk" {
		t.Fatalf("unexpected search input: %+v", fake.searchInput)
	}
}

func TestInstamartProductRowsRenderVariationsAndSponsored(t *testing.T) {
	m := instamartModel{
		screen: instamartScreenProductList,
		rows: []productVariationRow{{
			Product: domaininstamart.Product{DisplayName: "Bread", Promoted: true, InStock: true, Available: true},
			Variation: domaininstamart.ProductVariation{
				SpinID:              "spin-bread",
				DisplayName:         "Sandwich Bread",
				QuantityDescription: "400 g",
				Price:               domaininstamart.Price{OfferPrice: 49},
				InStock:             true,
			},
		}},
	}
	out := m.View()
	for _, want := range []string{"Sponsored", "Sandwich Bread", "400 g", "Rs 49"} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected %q in product output", want)
		}
	}
}

func TestInstamartUnavailableVariationCannotBeSelected(t *testing.T) {
	m := instamartModel{
		screen: instamartScreenProductList,
		rows: []productVariationRow{{
			Product:   domaininstamart.Product{DisplayName: "Bread", InStock: true, Available: false},
			Variation: domaininstamart.ProductVariation{SpinID: "spin-bread", DisplayName: "Bread", InStock: true},
		}},
	}

	updated, cmd := m.handleProductKey("1")
	if cmd != nil {
		t.Fatal("unavailable row should not start cart update")
	}
	got := updated.(instamartModel)
	if got.screen != instamartScreenProductList {
		t.Fatalf("expected to stay on product list, got %v", got.screen)
	}
	if !strings.Contains(got.err, "currently unavailable") {
		t.Fatalf("expected unavailable message, got %q", got.err)
	}
}

func TestInstamartCartUpdatesOnlyAfterVariationAndQuantity(t *testing.T) {
	fake := &fakeInstamartService{cart: cartWithItems([]domaininstamart.CartItem{{SpinID: "spin-milk", Name: "Milk 1 L", Quantity: 2, FinalPrice: 120}})}
	address := domaininstamart.Address{ID: "addr-1", Label: "Home"}
	m := instamartModel{
		ctx:             context.Background(),
		service:         fake,
		screen:          instamartScreenProductList,
		selectedAddress: &address,
		rows: []productVariationRow{{
			Product:   domaininstamart.Product{DisplayName: "Milk", InStock: true, Available: true},
			Variation: domaininstamart.ProductVariation{SpinID: "spin-milk", DisplayName: "Milk", QuantityDescription: "1 L", Price: domaininstamart.Price{OfferPrice: 60}, InStock: true},
		}},
	}

	updated, cmd := m.handleProductKey("1")
	if cmd != nil || fake.updateCalls != 0 {
		t.Fatal("selecting a variation must not update cart yet")
	}
	selected := updated.(instamartModel)
	selected.quantity = 2
	_, cmd = selected.handleQuantityKey("enter")
	if cmd == nil {
		t.Fatal("quantity confirmation should update cart")
	}
	_ = cmd()
	if fake.updateCalls != 1 {
		t.Fatalf("expected one update call, got %d", fake.updateCalls)
	}
	if len(fake.updateInput.Items) != 1 || fake.updateInput.Items[0].SpinID != "spin-milk" || fake.updateInput.Items[0].Quantity != 2 {
		t.Fatalf("unexpected update items: %+v", fake.updateInput.Items)
	}
}

func TestInstamartQuantityUpdateSendsFullIntendedCart(t *testing.T) {
	fake := &fakeInstamartService{cart: cartWithItems([]domaininstamart.CartItem{
		{SpinID: "spin-milk", Name: "Milk 1 L", Quantity: 3, FinalPrice: 180},
		{SpinID: "spin-bread", Name: "Bread 400 g", Quantity: 1, FinalPrice: 49},
	})}
	address := domaininstamart.Address{ID: "addr-1", Label: "Home"}
	m := instamartModel{
		ctx:             context.Background(),
		service:         fake,
		screen:          instamartScreenQuantity,
		selectedAddress: &address,
		intendedItems: []domaininstamart.CartUpdateItem{
			{SpinID: "spin-milk", Quantity: 1},
			{SpinID: "spin-bread", Quantity: 1},
		},
		selectedRow: &productVariationRow{Variation: domaininstamart.ProductVariation{SpinID: "spin-milk"}},
		quantity:    3,
	}

	_, cmd := m.handleQuantityKey("enter")
	if cmd == nil {
		t.Fatal("expected update command")
	}
	_ = cmd()
	if len(fake.updateInput.Items) != 2 {
		t.Fatalf("expected full cart update, got %+v", fake.updateInput.Items)
	}
	if fake.updateInput.Items[0].SpinID != "spin-milk" || fake.updateInput.Items[0].Quantity != 3 {
		t.Fatalf("expected milk quantity replacement, got %+v", fake.updateInput.Items[0])
	}
	if fake.updateInput.Items[1].SpinID != "spin-bread" || fake.updateInput.Items[1].Quantity != 1 {
		t.Fatalf("expected bread to be preserved, got %+v", fake.updateInput.Items[1])
	}
}

func TestInstamartCartReviewRendersCheckoutDetails(t *testing.T) {
	m := instamartModel{screen: instamartScreenCartReview, currentCart: domaininstamart.Cart{
		AddressLabel:       "Work",
		AddressDisplayLine: "Tower, Bangalore",
		AddressLocation:    &domaininstamart.Location{Lat: 12.34, Lng: 56.78},
		Items:              []domaininstamart.CartItem{{SpinID: "spin-milk", Name: "Milk 1 L", Quantity: 2, FinalPrice: 120}},
		Bill: domaininstamart.BillBreakdown{
			Lines:       []domaininstamart.BillLine{{Label: "Item Total", Value: "Rs 120"}, {Label: "Fees", Value: "Rs 20"}},
			ToPayLabel:  "To Pay",
			ToPayValue:  "Rs 140",
			ToPayRupees: 140,
		},
		AvailablePaymentMethods: []string{"Cash"},
		StoreIDs:                []string{"store-1", "store-2"},
	}}
	out := m.View()
	for _, want := range []string{"Work", "Milk 1 L", "Item Total", "To Pay: Rs 140", "Cash", "Multi-store"} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected %q in cart review", want)
		}
	}
	if strings.Contains(out, "Tower, Bangalore") {
		t.Fatal("full address must not be rendered")
	}
	if strings.Contains(out, "12.34") || strings.Contains(out, "56.78") {
		t.Fatal("coordinates must not be rendered")
	}
}

func TestInstamartErrorsAreSanitized(t *testing.T) {
	out := displayErr("Checkout blocked", domaininstamart.ErrCheckoutRequiresReview)
	if !strings.Contains(out, "review the latest cart") {
		t.Fatalf("expected useful sentinel mapping, got %q", out)
	}
	out = displayErr("Checkout blocked", errWithSensitiveText("order real-order-123 address full-address"))
	if strings.Contains(out, "real-order-123") || strings.Contains(out, "full-address") {
		t.Fatalf("raw provider error leaked: %q", out)
	}
	if !strings.Contains(out, "Please try again") {
		t.Fatalf("expected generic fallback, got %q", out)
	}
}

func TestInstamartCheckoutRequiresExplicitConfirmation(t *testing.T) {
	fake := &fakeInstamartService{checkoutResult: domaininstamart.CheckoutResult{Message: "Instamart order placed successfully! Mock order confirmed."}}
	m := checkoutConfirmModel(fake)

	_, cmd := m.handleCartReviewKey("p")
	if cmd != nil || fake.checkoutCalls != 0 {
		t.Fatal("moving to confirmation must not call checkout")
	}
	confirm := checkoutConfirmModel(fake)
	_, cmd = confirm.handleCheckoutConfirmKey("y")
	if cmd == nil {
		t.Fatal("expected checkout command after explicit confirmation")
	}
	_ = cmd()
	if fake.checkoutCalls != 1 {
		t.Fatalf("expected checkout call, got %d", fake.checkoutCalls)
	}
	if !fake.checkoutInput.Confirmed {
		t.Fatal("checkout must pass Confirmed=true")
	}
	if fake.checkoutInput.AddressID != "addr-1" {
		t.Fatalf("checkout should use selected address, got %q", fake.checkoutInput.AddressID)
	}
}

func TestInstamartCheckoutBlocksStaleCartAddress(t *testing.T) {
	fake := &fakeInstamartService{}
	m := checkoutConfirmModel(fake)
	m.screen = instamartScreenCartReview
	m.currentCart.AddressID = "addr-other"
	updated, cmd := m.handleCartReviewKey("p")
	if cmd != nil {
		t.Fatal("stale cart address should not proceed to checkout confirmation")
	}
	if !strings.Contains(updated.(instamartModel).err, "Cart address no longer matches") {
		t.Fatalf("expected stale address error, got %q", updated.(instamartModel).err)
	}
}

func TestInstamartTrackingWithoutLocationUsesSafeMessage(t *testing.T) {
	fake := &fakeInstamartService{orders: domaininstamart.OrderHistory{Orders: []domaininstamart.OrderSummary{{OrderID: "real-order-hidden", Status: "CONFIRMED", Active: true, ItemCount: 1, TotalRupees: 140}}}}
	m := instamartModel{ctx: context.Background(), service: fake}
	msg := m.loadOrdersCmd(true)()
	ordersMsg, ok := msg.(instamartOrdersMsg)
	if !ok {
		t.Fatalf("expected orders message, got %T", msg)
	}
	updated, _ := m.Update(ordersMsg)
	out := updated.(instamartModel).View()
	if fake.trackCalls != 0 {
		t.Fatal("tracking must not be called without hidden coordinates")
	}
	if !strings.Contains(out, "Tracking is unavailable for this order in the terminal") {
		t.Fatal("expected safe tracking fallback message")
	}
	if strings.Contains(out, "real-order-hidden") {
		t.Fatal("order id must not be rendered in fallback output")
	}
}

func TestInstamartOrderHistoryEnterTracksSelectedOrder(t *testing.T) {
	location := &domaininstamart.Location{Lat: 12.9, Lng: 77.6}
	fake := &fakeInstamartService{tracking: domaininstamart.TrackingStatus{StatusMessage: "Order is getting packed", ETAText: "5 mins"}}
	m := instamartModel{
		ctx:     context.Background(),
		service: fake,
		screen:  instamartScreenOrders,
		orders:  domaininstamart.OrderHistory{Orders: []domaininstamart.OrderSummary{{OrderID: "order-1", Status: "CONFIRMED", Active: true, Location: location}}},
	}

	_, cmd := m.handleOrdersKey("enter")
	if cmd == nil {
		t.Fatal("expected tracking command")
	}
	_ = cmd()
	if fake.trackCalls != 1 {
		t.Fatalf("expected tracking call, got %d", fake.trackCalls)
	}
}

func TestInstamartCancellationGuidanceDoesNotCallService(t *testing.T) {
	fake := &fakeInstamartService{}
	m := instamartModel{ctx: context.Background(), service: fake, screen: instamartScreenHome}
	updated, cmd := m.runHomeAction("cancel")
	if cmd != nil {
		t.Fatal("cancel guidance should not call service")
	}
	out := updated.(instamartModel).View()
	if !strings.Contains(out, cancellationGuidance) {
		t.Fatal("expected exact cancellation guidance")
	}
	if strings.Contains(out, "✓ "+cancellationGuidance) {
		t.Fatal("cancellation guidance must not render as a success status")
	}
	if fake.anyCalls() {
		t.Fatal("cancellation guidance must not call provider service")
	}
}

func TestInstamartViewCartRequiresSelectedAddress(t *testing.T) {
	fake := &fakeInstamartService{cart: cartWithItems(nil)}
	m := instamartModel{ctx: context.Background(), service: fake, screen: instamartScreenHome}
	updated, cmd := m.handleHomeKey("c")
	if cmd != nil {
		t.Fatal("cart should not load before address selection")
	}
	if fake.getCartCalls != 0 {
		t.Fatal("service GetCart should not be called before address selection")
	}
	if !strings.Contains(updated.(instamartModel).err, "Choose a delivery address") {
		t.Fatalf("expected address error, got %q", updated.(instamartModel).err)
	}
}

func TestInstamartQuantityZeroSendsEmptyReplacementForLastItem(t *testing.T) {
	fake := &fakeInstamartService{}
	address := domaininstamart.Address{ID: "addr-1", Label: "Home"}
	m := instamartModel{
		ctx:             context.Background(),
		service:         fake,
		selectedAddress: &address,
		intendedItems:   []domaininstamart.CartUpdateItem{{SpinID: "spin-milk", Quantity: 1}},
		selectedRow:     &productVariationRow{Variation: domaininstamart.ProductVariation{SpinID: "spin-milk"}},
		quantity:        0,
	}
	_, cmd := m.handleQuantityKey("enter")
	if cmd != nil {
		_ = cmd()
	} else {
		t.Fatal("expected update command")
	}
	if fake.updateCalls != 1 {
		t.Fatalf("expected one update call, got %d", fake.updateCalls)
	}
	if len(fake.updateInput.Items) != 0 {
		t.Fatalf("expected empty replacement list, got %+v", fake.updateInput.Items)
	}
}

func checkoutConfirmModel(fake *fakeInstamartService) instamartModel {
	address := domaininstamart.Address{ID: "addr-1", Label: "Home"}
	return instamartModel{
		ctx:             context.Background(),
		service:         fake,
		screen:          instamartScreenCheckoutConfirm,
		selectedAddress: &address,
		currentCart: domaininstamart.Cart{
			AddressID:               "addr-1",
			Items:                   []domaininstamart.CartItem{{SpinID: "spin-milk", Name: "Milk", Quantity: 1, FinalPrice: 60}},
			Bill:                    domaininstamart.BillBreakdown{ToPayValue: "Rs 80", ToPayRupees: 80},
			AvailablePaymentMethods: []string{"Cash"},
		},
		intendedItems: []domaininstamart.CartUpdateItem{{SpinID: "spin-milk", Quantity: 1}},
		paymentMethod: "Cash",
		reviewedCart: &domaininstamart.CartReviewSnapshot{
			AddressID:     "addr-1",
			Items:         []domaininstamart.CartUpdateItem{{SpinID: "spin-milk", Quantity: 1}},
			ToPayRupees:   80,
			PaymentMethod: "Cash",
		},
	}
}

func cartWithItems(items []domaininstamart.CartItem) domaininstamart.Cart {
	return domaininstamart.Cart{
		AddressID:               "addr-1",
		AddressLabel:            "Home",
		Items:                   items,
		Bill:                    domaininstamart.BillBreakdown{ToPayLabel: "To Pay", ToPayValue: "Rs 100", ToPayRupees: 100},
		TotalRupees:             100,
		AvailablePaymentMethods: []string{"Cash"},
	}
}

type fakeInstamartService struct {
	searchInput    appinstamart.SearchProductsInput
	updateInput    appinstamart.UpdateCartInput
	checkoutInput  appinstamart.CheckoutInput
	cart           domaininstamart.Cart
	orders         domaininstamart.OrderHistory
	tracking       domaininstamart.TrackingStatus
	checkoutResult domaininstamart.CheckoutResult
	updateCalls    int
	getCartCalls   int
	checkoutCalls  int
	trackCalls     int
}

func (f *fakeInstamartService) GetAddresses(context.Context) ([]domaininstamart.Address, error) {
	return []domaininstamart.Address{{ID: "addr-1", Label: "Home", DisplayLine: "Test address", PhoneMasked: "****0001"}}, nil
}

func (f *fakeInstamartService) SearchProducts(_ context.Context, input appinstamart.SearchProductsInput) (domaininstamart.ProductSearchResult, error) {
	f.searchInput = input
	return domaininstamart.ProductSearchResult{}, nil
}

func (f *fakeInstamartService) GetGoToItems(context.Context, appinstamart.GetGoToItemsInput) (domaininstamart.ProductSearchResult, error) {
	return domaininstamart.ProductSearchResult{}, nil
}

func (f *fakeInstamartService) GetCart(context.Context) (domaininstamart.Cart, error) {
	f.getCartCalls++
	return f.cart, nil
}

func (f *fakeInstamartService) UpdateCart(_ context.Context, input appinstamart.UpdateCartInput) (domaininstamart.Cart, error) {
	f.updateCalls++
	f.updateInput = input
	return f.cart, nil
}

func (f *fakeInstamartService) Checkout(_ context.Context, input appinstamart.CheckoutInput) (domaininstamart.CheckoutResult, error) {
	f.checkoutCalls++
	f.checkoutInput = input
	return f.checkoutResult, nil
}

func (f *fakeInstamartService) GetOrders(_ context.Context, input appinstamart.GetOrdersInput) (domaininstamart.OrderHistory, error) {
	return f.orders, nil
}

func (f *fakeInstamartService) TrackOrder(context.Context, appinstamart.TrackOrderInput) (domaininstamart.TrackingStatus, error) {
	f.trackCalls++
	return f.tracking, nil
}

func (f *fakeInstamartService) anyCalls() bool {
	return f.updateCalls > 0 || f.checkoutCalls > 0 || f.trackCalls > 0 || f.getCartCalls > 0 || f.searchInput.Query != ""
}

type errWithSensitiveText string

func (e errWithSensitiveText) Error() string { return string(e) }
