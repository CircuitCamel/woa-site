package woa

type Session struct {
	ID    int
	Path  string
	Body  string
	Time  string
	Place string
}

type Rule struct {
	Path      string
	Body      string
	Title     string
	TitlePath string
}

type Character struct {
	Name     string
	NamePath string
	Race     string
	Class    string
	Age      string
	Level    string
	Body     string
}

type NPC struct {
	Name       string
	NamePath   string
	Race       string
	Occupation string
	Location   string
	Status     string
	Body       string
}

type Item struct {
	Name       string
	NamePath   string
	Type       string
	Rarity     string
	Attunement string
	Body       string
}
