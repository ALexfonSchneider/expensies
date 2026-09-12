package yandexpdf

import (
	"regexp"
	"strings"

	"github.com/ALexfonSchneider/expenses/internal/domain"
)

var (
	rePurchase     = regexp.MustCompile(`^Оплата товаров и услуг\s*(.*)$`)
	reSBPQR        = regexp.MustCompile(`^Оплата СБП QR \((.*)\)$`)
	reSberQR       = regexp.MustCompile(`^Оплата Сбер QR \((.*)\)$`)
	reSberBT       = regexp.MustCompile(`^Оплата Сбер Bluetooth\s*(.*)$`)
	reTransferIn   = regexp.MustCompile(`^Входящий перевод СБП,?\s*(.*)$`)
	reTransferOut  = regexp.MustCompile(`^Исходящий перевод СБП,?\s*(.*)$`)
	reInternalTo   = regexp.MustCompile(`^Внутрибанковский перевод на\s*(.*)$`)
	reInternalFrom = regexp.MustCompile(`^Внутрибанковский перевод (?:от|с)\s*(.*)$`)
	reMCC          = regexp.MustCompile(`\*(\d{4})\*`)
)

// merchantSuffixes are acquirer artifacts appended to QR merchant names
// ("Гастроном 28_P_QR"); stripping them lets one rule cover a shop paid
// through different channels.
var merchantSuffixes = []string{"_P_QR", "_SBP"}

// classify derives the operation kind and the merchant or counterparty
// from a reassembled description line.
func classify(desc string) (kind domain.Kind, merchant, mcc, counterparty string) {
	switch {
	case submatch(rePurchase, desc, &merchant):
		kind = domain.KindPurchase
	case submatch(reSBPQR, desc, &merchant):
		kind = domain.KindPurchase
	case submatch(reSberQR, desc, &merchant):
		kind = domain.KindPurchase
	case submatch(reSberBT, desc, &merchant):
		kind = domain.KindPurchase
	case submatch(reTransferIn, desc, &counterparty):
		kind = domain.KindTransferIn
	case submatch(reTransferOut, desc, &counterparty):
		kind = domain.KindTransferOut
	case submatch(reInternalTo, desc, &counterparty):
		kind = domain.KindTransferOut
	case submatch(reInternalFrom, desc, &counterparty):
		kind = domain.KindTransferIn
	case strings.HasPrefix(desc, "Компенсация"):
		kind = domain.KindCashback
	case strings.HasPrefix(desc, "Отмена оплаты"):
		kind = domain.KindRefund
	default:
		kind = domain.KindOther
	}

	if m := reMCC.FindStringSubmatch(merchant); m != nil {
		mcc = m[1]
	}
	merchant = cleanMerchant(merchant)
	counterparty = collapseSpaces(counterparty)
	return kind, merchant, mcc, counterparty
}

func submatch(re *regexp.Regexp, s string, out *string) bool {
	m := re.FindStringSubmatch(s)
	if m == nil {
		return false
	}
	*out = m[1]
	return true
}

func cleanMerchant(s string) string {
	s = strings.TrimSpace(s)
	for _, suffix := range merchantSuffixes {
		s = strings.TrimSuffix(s, suffix)
	}
	return collapseSpaces(s)
}
