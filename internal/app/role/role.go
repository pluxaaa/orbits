package role

type Role int

const (
	Guest Role = iota
	Client
	Moderator
)
