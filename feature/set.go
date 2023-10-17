package feature

// Set is an enum identifying various feature sets, which separates the single
// feature namespace into distinct categories depending what context a feature
// vector is being used.
type Set uint8

const (
	// SetInit identifies features that should be sent in an Init message to
	// a remote peer.
	SetInit Set = iota

	// SetLegacyGlobal identifies features that should be set in the legacy
	// GlobalFeatures field of an Init message, which maintains backwards
	// compatibility with nodes that haven't implemented flat features.
	SetLegacyGlobal

	// SetNodeAnn identifies features that should be advertised on node
	// announcements.
	SetNodeAnn

	// SetInvoice identifies features that should be advertised on invoices
	// generated by the daemon.
	SetInvoice

	// SetInvoiceAmp identifies the features that should be advertised on
	// AMP invoices generated by the daemon.
	SetInvoiceAmp

	// SetInvoiceNoMpp for Nayuta Wallet no MPP invoice
	SetInvoiceNoMpp
)

// String returns a human-readable description of a Set.
func (s Set) String() string {
	switch s {
	case SetInit:
		return "SetInit"
	case SetLegacyGlobal:
		return "SetLegacyGlobal"
	case SetNodeAnn:
		return "SetNodeAnn"
	case SetInvoice:
		return "SetInvoice"
	case SetInvoiceAmp:
		return "SetInvoiceAmp"
	case SetInvoiceNoMpp:
		return "SetInvoiceNoMpp"
	default:
		return "SetUnknown"
	}
}
