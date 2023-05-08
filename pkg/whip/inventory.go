package whip

type (
	Target struct {
		User string
		Host string
		Port int
		Tag  string
	}
	Inventory []Target
)
