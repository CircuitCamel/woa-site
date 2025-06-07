package woa

type Session struct {
	ID   int
	Path string
	Body string
}

type Rule struct {
	ID   int
	Path string
	Body string
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
