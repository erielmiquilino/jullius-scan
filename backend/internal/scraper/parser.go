package scraper

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/erielfranco/jullius-scan/backend/internal/domain"
)

// ParsedReceipt holds the normalized data extracted from a SEFAZ NFC-e HTML page.
type ParsedReceipt struct {
	Store   domain.Store
	Receipt ReceiptData
	Items   []domain.Item
}

// ReceiptData holds receipt-level fields before they are linked to a DB record.
type ReceiptData struct {
	FiscalKey   string
	IssuedAt    time.Time
	TotalAmount float64
}

// ParseSEFAZHTML extracts receipt data from the rendered SEFAZ NFC-e HTML body.
//
// SEFAZ NFC-e pages have a semi-standard structure across Brazilian states:
//   - Store info (CNPJ, name, address) near the top
//   - Item table with description, quantity, unit, unit price, total price
//   - Totals section with the receipt total
//   - Fiscal key (chave de acesso) — a 44-digit number
//   - Issue date/time
//
// This parser uses regex-based extraction suitable for the MVP.
// It will need refinement as more state SEFAZ page variants are encountered.
func ParseSEFAZHTML(html string) (*ParsedReceipt, error) {
	if strings.TrimSpace(html) == "" {
		return nil, fmt.Errorf("empty HTML content")
	}

	result := &ParsedReceipt{}

	// Extract store information
	store, err := extractStore(html)
	if err != nil {
		return nil, fmt.Errorf("extract store: %w", err)
	}
	result.Store = *store

	// Extract fiscal key
	fiscalKey, err := extractFiscalKey(html)
	if err != nil {
		return nil, fmt.Errorf("extract fiscal key: %w", err)
	}
	result.Receipt.FiscalKey = fiscalKey

	// Extract total amount
	totalAmount, err := extractTotalAmount(html)
	if err != nil {
		return nil, fmt.Errorf("extract total amount: %w", err)
	}
	result.Receipt.TotalAmount = totalAmount

	// Extract issue date
	issuedAt, err := extractIssuedAt(html)
	if err != nil {
		return nil, fmt.Errorf("extract issued_at: %w", err)
	}
	result.Receipt.IssuedAt = issuedAt

	// Extract items
	items, err := extractItems(html)
	if err != nil {
		// Items extraction failure is non-fatal — we log but still persist the receipt
		items = []domain.Item{}
	}
	result.Items = items

	return result, nil
}

// --- Store extraction ---

var (
	// CNPJ pattern: 14 digits, optionally formatted as XX.XXX.XXX/XXXX-XX
	cnpjRegex = regexp.MustCompile(`(?i)CNPJ\s*[:\s]*(\d{2}\.?\d{3}\.?\d{3}/?\d{4}-?\d{2})`)

	// Store name: typically in a prominent element near CNPJ.
	// Look for text in elements after/near CNPJ or in header-like elements.
	storeNameRegex = regexp.MustCompile(`(?i)<(?:span|div|td|strong|b)[^>]*class="[^"]*(?:txtTopo|emit(?:ente)?N(?:ome)?|razao|nome)[^"]*"[^>]*>([^<]+)`)

	// Fallback: any text content right after CNPJ mention that looks like a company name.
	storeNameFallbackRegex = regexp.MustCompile(`(?i)(?:Razão Social|Nome|Emitente)\s*[:\s]*([^\n<]{3,80})`)

	// Address patterns
	storeAddressRegex = regexp.MustCompile(`(?i)(?:Endereço|Endereco|Logradouro)\s*[:\s]*([^\n<]{5,150})`)
)

func extractStore(html string) (*domain.Store, error) {
	store := &domain.Store{}

	// Extract CNPJ
	cnpjMatch := cnpjRegex.FindStringSubmatch(html)
	if len(cnpjMatch) < 2 {
		return nil, fmt.Errorf("CNPJ not found in HTML")
	}
	store.CNPJ = normalizeCNPJ(cnpjMatch[1])

	// Extract store name
	nameMatch := storeNameRegex.FindStringSubmatch(html)
	if len(nameMatch) >= 2 {
		store.Name = strings.TrimSpace(nameMatch[1])
	} else {
		fallbackMatch := storeNameFallbackRegex.FindStringSubmatch(html)
		if len(fallbackMatch) >= 2 {
			store.Name = strings.TrimSpace(fallbackMatch[1])
		} else {
			store.Name = "Unknown Store"
		}
	}

	// Extract address (optional)
	addrMatch := storeAddressRegex.FindStringSubmatch(html)
	if len(addrMatch) >= 2 {
		store.Address = strings.TrimSpace(addrMatch[1])
	}

	return store, nil
}

// normalizeCNPJ strips formatting from a CNPJ string, returning only digits.
func normalizeCNPJ(raw string) string {
	return strings.NewReplacer(".", "", "/", "", "-", "").Replace(raw)
}

// --- Fiscal key extraction ---

var (
	// Fiscal key (chave de acesso): exactly 44 digits, may have spaces between groups.
	fiscalKeyRegex = regexp.MustCompile(`(?i)(?:Chave de Acesso|chave|NFCe)\s*[:\s]*([\d\s]{44,60})`)

	// Direct 44-digit match in the page.
	fiscalKey44Regex = regexp.MustCompile(`(\d{4}\s?\d{4}\s?\d{4}\s?\d{4}\s?\d{4}\s?\d{4}\s?\d{4}\s?\d{4}\s?\d{4}\s?\d{4}\s?\d{4})`)
)

func extractFiscalKey(html string) (string, error) {
	// Try labeled extraction first
	match := fiscalKeyRegex.FindStringSubmatch(html)
	if len(match) >= 2 {
		key := strings.ReplaceAll(match[1], " ", "")
		if len(key) == 44 {
			return key, nil
		}
	}

	// Fallback: find any 44-digit sequence
	match = fiscalKey44Regex.FindStringSubmatch(html)
	if len(match) >= 2 {
		key := strings.ReplaceAll(match[1], " ", "")
		if len(key) == 44 {
			return key, nil
		}
	}

	return "", fmt.Errorf("fiscal key (44-digit chave de acesso) not found")
}

// --- Total amount extraction ---

var (
	// Total amount: look for "Total" or "Valor Total" followed by a BRL currency value.
	totalRegex = regexp.MustCompile(`(?i)(?:Valor\s+Total|Total\s+(?:da\s+)?(?:Nota|NFC-?e))\s*[:\s]*R?\$?\s*([\d.,]+)`)
)

func extractTotalAmount(html string) (float64, error) {
	match := totalRegex.FindStringSubmatch(html)
	if len(match) < 2 {
		return 0, fmt.Errorf("total amount not found")
	}
	return parseBRLAmount(match[1])
}

// parseBRLAmount converts a Brazilian currency string (1.234,56) to float64.
func parseBRLAmount(raw string) (float64, error) {
	cleaned := strings.TrimSpace(raw)
	// Brazilian format: dots as thousand separators, comma as decimal separator.
	cleaned = strings.ReplaceAll(cleaned, ".", "")
	cleaned = strings.ReplaceAll(cleaned, ",", ".")
	val, err := strconv.ParseFloat(cleaned, 64)
	if err != nil {
		return 0, fmt.Errorf("parse BRL amount %q: %w", raw, err)
	}
	return val, nil
}

// --- Issue date extraction ---

var (
	// Date pattern: DD/MM/YYYY HH:MM:SS or DD/MM/YYYY
	dateRegex = regexp.MustCompile(`(?i)(?:Data\s+(?:de\s+)?Emiss[aã]o|Emiss[aã]o)\s*[:\s]*(\d{2}/\d{2}/\d{4}\s*\d{0,2}:?\d{0,2}:?\d{0,2})`)

	// Fallback: any date that looks like DD/MM/YYYY near time
	dateFallbackRegex = regexp.MustCompile(`(\d{2}/\d{2}/\d{4}\s+\d{2}:\d{2}:\d{2})`)
)

func extractIssuedAt(html string) (time.Time, error) {
	match := dateRegex.FindStringSubmatch(html)
	if len(match) >= 2 {
		return parseBRDate(strings.TrimSpace(match[1]))
	}

	match = dateFallbackRegex.FindStringSubmatch(html)
	if len(match) >= 2 {
		return parseBRDate(strings.TrimSpace(match[1]))
	}

	return time.Time{}, fmt.Errorf("issue date not found")
}

// parseBRDate parses a Brazilian date string (DD/MM/YYYY HH:MM:SS) to time.Time.
func parseBRDate(raw string) (time.Time, error) {
	layouts := []string{
		"02/01/2006 15:04:05",
		"02/01/2006 15:04",
		"02/01/2006",
	}
	for _, layout := range layouts {
		t, err := time.Parse(layout, raw)
		if err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("unable to parse date %q", raw)
}

// --- Items extraction ---

var (
	// Items in NFC-e pages are typically in a table or repeated div structure.
	// Common patterns: description, quantity, unit, unit price, total price.
	//
	// Pattern: look for rows with item data. SEFAZ pages vary, so we use
	// a flexible pattern that matches common table-row or div-based layouts.
	itemRowRegex = regexp.MustCompile(`(?is)<tr[^>]*class="[^"]*(?:item|prod)[^"]*"[^>]*>(.*?)</tr>`)

	// Extract individual fields from within an item row.
	itemDescRegex  = regexp.MustCompile(`(?is)<(?:span|td)[^>]*class="[^"]*(?:txtTit|descricao|desc|produto)[^"]*"[^>]*>([^<]+)`)
	itemQtyRegex   = regexp.MustCompile(`(?i)(?:Qtde|Qty|Quant)[.:]?\s*([\d.,]+)`)
	itemUnitRegex  = regexp.MustCompile(`(?i)(?:UN|Unid|Unidade)[.:]?\s*(\S+)`)
	itemPriceRegex = regexp.MustCompile(`(?i)(?:Vl\.?\s*Unit|Unitário|Unit)[.:]?\s*R?\$?\s*([\d.,]+)`)
	itemTotalRegex = regexp.MustCompile(`(?i)(?:Vl\.?\s*Total|Total)[.:]?\s*R?\$?\s*([\d.,]+)`)
)

func extractItems(html string) ([]domain.Item, error) {
	rows := itemRowRegex.FindAllStringSubmatch(html, -1)
	if len(rows) == 0 {
		// Try alternative: div-based item blocks used by some states.
		return extractItemsDivBased(html)
	}

	var items []domain.Item
	for _, row := range rows {
		if len(row) < 2 {
			continue
		}
		rowHTML := row[1]

		item := domain.Item{}

		// Description
		descMatch := itemDescRegex.FindStringSubmatch(rowHTML)
		if len(descMatch) >= 2 {
			item.Description = strings.TrimSpace(descMatch[1])
		}
		if item.Description == "" {
			continue // skip rows without a description
		}

		// Quantity
		qtyMatch := itemQtyRegex.FindStringSubmatch(rowHTML)
		if len(qtyMatch) >= 2 {
			item.Quantity, _ = parseBRLAmount(qtyMatch[1])
		}
		if item.Quantity == 0 {
			item.Quantity = 1
		}

		// Unit
		unitMatch := itemUnitRegex.FindStringSubmatch(rowHTML)
		if len(unitMatch) >= 2 {
			item.Unit = strings.TrimSpace(unitMatch[1])
		}
		if item.Unit == "" {
			item.Unit = "UN"
		}

		// Unit price
		priceMatch := itemPriceRegex.FindStringSubmatch(rowHTML)
		if len(priceMatch) >= 2 {
			item.UnitPrice, _ = parseBRLAmount(priceMatch[1])
		}

		// Total price
		totalMatch := itemTotalRegex.FindStringSubmatch(rowHTML)
		if len(totalMatch) >= 2 {
			item.TotalPrice, _ = parseBRLAmount(totalMatch[1])
		}

		// If we have unit price but no total, calculate it.
		if item.TotalPrice == 0 && item.UnitPrice > 0 {
			item.TotalPrice = item.UnitPrice * item.Quantity
		}

		items = append(items, item)
	}

	if len(items) == 0 {
		return nil, fmt.Errorf("no items extracted from table rows")
	}

	return items, nil
}

// extractItemsDivBased tries to extract items from div-based layouts used by some SEFAZ sites.
var itemDivRegex = regexp.MustCompile(`(?is)<div[^>]*class="[^"]*(?:det(?:alhe)?Item|item|produto)[^"]*"[^>]*>(.*?)</div>\s*(?:</div>|<div)`)

func extractItemsDivBased(html string) ([]domain.Item, error) {
	blocks := itemDivRegex.FindAllStringSubmatch(html, -1)
	if len(blocks) == 0 {
		return nil, fmt.Errorf("no item blocks found in HTML")
	}

	var items []domain.Item
	for _, block := range blocks {
		if len(block) < 2 {
			continue
		}
		blockHTML := block[1]

		item := domain.Item{}

		descMatch := itemDescRegex.FindStringSubmatch(blockHTML)
		if len(descMatch) >= 2 {
			item.Description = strings.TrimSpace(descMatch[1])
		}
		if item.Description == "" {
			continue
		}

		qtyMatch := itemQtyRegex.FindStringSubmatch(blockHTML)
		if len(qtyMatch) >= 2 {
			item.Quantity, _ = parseBRLAmount(qtyMatch[1])
		}
		if item.Quantity == 0 {
			item.Quantity = 1
		}

		unitMatch := itemUnitRegex.FindStringSubmatch(blockHTML)
		if len(unitMatch) >= 2 {
			item.Unit = strings.TrimSpace(unitMatch[1])
		}
		if item.Unit == "" {
			item.Unit = "UN"
		}

		priceMatch := itemPriceRegex.FindStringSubmatch(blockHTML)
		if len(priceMatch) >= 2 {
			item.UnitPrice, _ = parseBRLAmount(priceMatch[1])
		}

		totalMatch := itemTotalRegex.FindStringSubmatch(blockHTML)
		if len(totalMatch) >= 2 {
			item.TotalPrice, _ = parseBRLAmount(totalMatch[1])
		}

		if item.TotalPrice == 0 && item.UnitPrice > 0 {
			item.TotalPrice = item.UnitPrice * item.Quantity
		}

		items = append(items, item)
	}

	if len(items) == 0 {
		return nil, fmt.Errorf("no items extracted from div blocks")
	}

	return items, nil
}
