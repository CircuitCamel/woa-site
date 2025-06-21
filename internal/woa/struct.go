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
