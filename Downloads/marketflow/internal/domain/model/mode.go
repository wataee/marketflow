package model

// DataMode представляет режим работы системы
type DataMode int

const (
	LiveMode DataMode = iota
	TestMode
)

func (m DataMode) String() string {
	switch m {
	case LiveMode:
		return "live"
	case TestMode:
		return "test"
	default:
		return "unknown"
	}
}
