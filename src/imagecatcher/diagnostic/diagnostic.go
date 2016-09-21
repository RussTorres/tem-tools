package diagnostic

// Pingable is a type that can be pinged
type Pingable interface {
	Ping() error
}
