package scraper

import (
	"fmt"
	"log/slog"
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
	// SC SEFAZ: <span class="totalNumb txtMax">VALUE</span> marks the payable total.
	totalNumbMaxRegex = regexp.MustCompile(`(?i)<span[^>]*class="[^"]*totalNumb\s+txtMax[^"]*"[^>]*>([\d.,]+)`)
)

func extractTotalAmount(html string) (float64, error) {
	match := totalRegex.FindStringSubmatch(html)
	if len(match) >= 2 {
		return parseBRLAmount(match[1])
	}
	match = totalNumbMaxRegex.FindStringSubmatch(html)
	if len(match) >= 2 {
		return parseBRLAmount(match[1])
	}
	return 0, fmt.Errorf("total amount not found")
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
	// Matches rows by class (RS/RJ style) or by id containing "Item" (SC style).
	itemRowRegex = regexp.MustCompile(`(?is)<tr[^>]*(?:class="[^"]*(?:item|prod)[^"]*"|id="[^"]*[Ii]tem[^"]*")[^>]*>(.*?)</tr>`)
	// SC SEFAZ: item total value lives in <span class="valor">.
	itemValorRegex = regexp.MustCompile(`(?i)<span[^>]*class="valor"[^>]*>([\d.,]+)`)

	// Extract individual fields from within an item row.
	itemDescRegex  = regexp.MustCompile(`(?is)<(?:span|td)[^>]*class="[^"]*(?:txtTit|descricao|desc|produto)[^"]*"[^>]*>([^<]+)`)
	itemQtyRegex   = regexp.MustCompile(`(?i)(?:Qtde|Qty|Quant)[.:]?\s*([\d.,]+)`)
	itemUnitRegex  = regexp.MustCompile(`(?i)(?:UN|Unid|Unidade)[.:]?\s*(\S+)`)
	itemPriceRegex = regexp.MustCompile(`(?i)(?:Vl\.?\s*Unit|Unitário|Unit)[.:]?\s*R?\$?\s*([\d.,]+)`)
	itemTotalRegex = regexp.MustCompile(`(?i)(?:Vl\.?\s*Total|Total)[.:]?\s*R?\$?\s*([\d.,]+)`)

	// SC SEFAZ puts labels inside <strong>…</strong> and values immediately after.
	// e.g. <strong>Qtde.:</strong>0,875  <strong>UN: </strong>KG  <strong>Vl. Unit.:</strong>&#160;3,99
	//
	// The gap between </strong> and the value may be empty, whitespace, or an
	// HTML entity such as &#160; / &nbsp;. It must NOT be `[^<]*` — that is
	// greedy and consumes the number itself, backtracking only one char and
	// capturing just the last digit (e.g. "9" instead of "13,99").
	scGapPattern     = `(?:\s|&[^;\s]+;)*`
	itemSCQtyRegex   = regexp.MustCompile(`(?i)<span[^>]*class="Rqtd"[^>]*>.*?</strong>` + scGapPattern + `([\d.,]+)`)
	itemSCUnitRegex  = regexp.MustCompile(`(?i)<span[^>]*class="RUN"[^>]*>.*?</strong>` + scGapPattern + `(\S+?)\s*</span>`)
	itemSCPriceRegex = regexp.MustCompile(`(?is)<span[^>]*class="RvlUnit"[^>]*>.*?</strong>` + scGapPattern + `([\d.,]+)`)
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

		// SC-specific regexes are tried first because SC's HTML contains tokens
		// like `class="Rqtd"` / `class="RUN"` / `class="RvlUnit"` that would
		// otherwise be captured (as garbage) by the lenient generic regexes.
		// The SC regexes are scoped to their CSS class so they never match on
		// non-SC HTML, meaning the generic fallback still covers other states.

		// Quantity.
		if m := itemSCQtyRegex.FindStringSubmatch(rowHTML); len(m) >= 2 {
			item.Quantity, _ = parseBRLAmount(m[1])
		} else if qtyMatch := itemQtyRegex.FindStringSubmatch(rowHTML); len(qtyMatch) >= 2 {
			item.Quantity, _ = parseBRLAmount(qtyMatch[1])
		}
		if item.Quantity == 0 {
			item.Quantity = 1
		}

		// Unit.
		if m := itemSCUnitRegex.FindStringSubmatch(rowHTML); len(m) >= 2 {
			item.Unit = strings.TrimSpace(m[1])
		} else if unitMatch := itemUnitRegex.FindStringSubmatch(rowHTML); len(unitMatch) >= 2 {
			item.Unit = strings.TrimSpace(unitMatch[1])
		}
		if item.Unit == "" {
			item.Unit = "UN"
		}

		// Unit price.
		if m := itemSCPriceRegex.FindStringSubmatch(rowHTML); len(m) >= 2 {
			item.UnitPrice, _ = parseBRLAmount(m[1])
		} else if priceMatch := itemPriceRegex.FindStringSubmatch(rowHTML); len(priceMatch) >= 2 {
			item.UnitPrice, _ = parseBRLAmount(priceMatch[1])
		}

		// Total price — try labeled pattern first, then SC's <span class="valor">.
		totalMatch := itemTotalRegex.FindStringSubmatch(rowHTML)
		if len(totalMatch) >= 2 {
			item.TotalPrice, _ = parseBRLAmount(totalMatch[1])
		} else if m := itemValorRegex.FindStringSubmatch(rowHTML); len(m) >= 2 {
			item.TotalPrice, _ = parseBRLAmount(m[1])
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

// --- Detail page (Nfe_DetalheCert.aspx) ---

var (
	// "Ver NFC-e detalhada" anchor on the summary page.
	// Captures the href which may be relative (e.g. /tax.NET/Sat.NFe.Web/Consultas/Nfe_DetalheCert.aspx?rq=TOKEN).
	detailLinkRegex = regexp.MustCompile(`(?i)href="([^"]*Nfe_DetalheCert\.aspx[^"]*)"`)

	// Fallback: button onclick with window.location or similar JS navigation.
	detailOnclickRegex = regexp.MustCompile(`(?i)(?:window\.location|location\.href)\s*=\s*['"]([^'"]*Nfe_DetalheCert\.aspx[^'"]*)['"]\s*;`)

	// Fallback: match any <a> whose visible text is "Ver NFC-e detalhada" regardless of href.
	detailLinkTextRegex = regexp.MustCompile(`(?is)<a\b[^>]*href="([^"]+)"[^>]*>\s*Ver\s+NF[Cc]-e\s+detalhada\s*</a>`)

	// EAN Comercial: label in one cell, value in the next cell.
	// The value is either digits (EAN-8/13) or "SEM GTIN" (no barcode).
	detailEANComercialRegex = regexp.MustCompile(`(?is)C[oó]digo\s+EAN\s+Comercial\s*(?:</[^>]+>\s*<[^>]+>|[:\s]+)\s*(SEM\s+GTIN|[\d]+)`)

	// Fallback: EAN Tributável, same pattern.
	detailEANTributavelRegex = regexp.MustCompile(`(?is)C[oó]digo\s+EAN\s+Tribut[aá]vel\s*(?:</[^>]+>\s*<[^>]+>|[:\s]+)\s*(SEM\s+GTIN|[\d]+)`)
)

// ExtractDetailLink finds the URL of the "Ver NFC-e detalhada" link in the NFC-e
// summary page HTML and returns it as an absolute URL given the baseURL of the summary page.
// Returns an error if no such link is found.
func ExtractDetailLink(summaryHTML, baseURL string) (string, error) {
	if m := detailLinkRegex.FindStringSubmatch(summaryHTML); len(m) >= 2 {
		return makeAbsoluteURL(m[1], baseURL), nil
	}
	if m := detailOnclickRegex.FindStringSubmatch(summaryHTML); len(m) >= 2 {
		return makeAbsoluteURL(m[1], baseURL), nil
	}
	if m := detailLinkTextRegex.FindStringSubmatch(summaryHTML); len(m) >= 2 {
		return makeAbsoluteURL(m[1], baseURL), nil
	}
	return "", fmt.Errorf("detail page link (Nfe_DetalheCert.aspx) not found in summary HTML")
}

// makeAbsoluteURL resolves href relative to baseURL.
// If href already starts with "http", it is returned unchanged.
func makeAbsoluteURL(href, baseURL string) string {
	if strings.HasPrefix(href, "http://") || strings.HasPrefix(href, "https://") {
		return href
	}
	// Derive scheme+host from baseURL.
	schemeEnd := strings.Index(baseURL, "://")
	if schemeEnd < 0 {
		return href
	}
	hostEnd := strings.Index(baseURL[schemeEnd+3:], "/")
	if hostEnd < 0 {
		return baseURL + href
	}
	origin := baseURL[:schemeEnd+3+hostEnd]
	if strings.HasPrefix(href, "/") {
		return origin + href
	}
	// relative path — strip last path segment from base
	lastSlash := strings.LastIndex(baseURL, "/")
	if lastSlash < 0 {
		return href
	}
	return baseURL[:lastSlash+1] + href
}

// ParseDetailPage extracts EAN codes from the NFC-e detail consultation page HTML.
// It returns a slice of EAN strings in item order (same position = same item as summary).
// Items without EAN yield an empty string entry; the function never returns an error for
// individual missing EANs.
func ParseDetailPage(detailHTML string) ([]string, error) {
	if strings.TrimSpace(detailHTML) == "" {
		return nil, fmt.Errorf("empty detail page HTML")
	}

	// Split the HTML into per-item blocks delimited by each EAN Comercial occurrence.
	// Strategy: find ALL EAN Comercial + EAN Tributável pairs in document order.
	comercialMatches := detailEANComercialRegex.FindAllStringSubmatch(detailHTML, -1)
	tributavelMatches := detailEANTributavelRegex.FindAllStringSubmatch(detailHTML, -1)

	count := len(comercialMatches)
	if count == 0 {
		// If neither field is present at all, the page may not be fully rendered.
		return nil, fmt.Errorf("no EAN Comercial fields found in detail page HTML")
	}

	barcodes := make([]string, count)
	for i, m := range comercialMatches {
		if len(m) >= 2 {
			raw := strings.TrimSpace(m[1])
			if !strings.EqualFold(raw, "SEM GTIN") && raw != "" {
				barcodes[i] = raw
			}
		}
		// Fallback to Tributável if Comercial was empty / SEM GTIN.
		if barcodes[i] == "" && i < len(tributavelMatches) {
			if tv := tributavelMatches[i]; len(tv) >= 2 {
				raw := strings.TrimSpace(tv[1])
				if !strings.EqualFold(raw, "SEM GTIN") && raw != "" {
					barcodes[i] = raw
				}
			}
		}
	}

	slog.Info("detail page parsed", "ean_count", count)
	return barcodes, nil
}

// MergeBarcodes assigns EAN codes from the detail page into the corresponding items
// parsed from the summary page, matching by positional index.
// Returns an error (without modifying items) if slice lengths differ, because a
// mismatch suggests a page structure change that would cause incorrect associations.
func MergeBarcodes(items []domain.Item, barcodes []string) ([]domain.Item, error) {
	if len(items) != len(barcodes) {
		return items, fmt.Errorf("merge barcodes: item count (%d) != barcode count (%d); skipping merge", len(items), len(barcodes))
	}
	for i := range items {
		if barcodes[i] != "" {
			b := barcodes[i]
			items[i].Barcode = &b
		}
	}
	return items, nil
}
