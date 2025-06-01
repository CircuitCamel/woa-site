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
	Name  string
	Race  string
	Class string
	Age   string
	Body  string
}
