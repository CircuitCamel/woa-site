package sheet

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"regexp"
	"strconv"
	"strings"
)

const (
	maxDiceTerms = 20
	maxDiceCount = 100
	maxDiceSides = 1000
	maxDiceExpr  = 100
)

// Die rolls one die. It's a variable so tests can make rolls predictable.
var Die = func(sides int) int {
	n, err := rand.Int(rand.Reader, big.NewInt(int64(sides)))
	if err != nil {
		panic(err) // crypto/rand never fails on supported platforms
	}
	return int(n.Int64()) + 1
}

var diceTerm = regexp.MustCompile(`([+-]?)(?:(\d*)d(\d+)|(\d+))`)

type term struct {
	sign  int
	count int
	sides int // 0 for a flat modifier
	flat  int
}

func parseDice(expr string) ([]term, error) {
	clean := strings.ToLower(strings.ReplaceAll(expr, " ", ""))
	if clean == "" || len(clean) > maxDiceExpr {
		return nil, fmt.Errorf("enter dice like 2d6+3")
	}
	var terms []term
	pos := 0
	dice := 0
	for _, m := range diceTerm.FindAllStringSubmatchIndex(clean, -1) {
		if m[0] != pos || (pos > 0 && m[2] == m[3]) {
			return nil, fmt.Errorf("can't read dice %q", expr)
		}
		pos = m[1]
		t := term{sign: 1}
		if clean[m[2]:m[3]] == "-" {
			t.sign = -1
		}
		if m[6] >= 0 {
			t.count = 1
			if m[4] != m[5] {
				t.count, _ = strconv.Atoi(clean[m[4]:m[5]])
			}
			t.sides, _ = strconv.Atoi(clean[m[6]:m[7]])
			if t.count < 1 || t.sides < 1 || t.sides > maxDiceSides {
				return nil, fmt.Errorf("can't read dice %q", expr)
			}
			dice += t.count
		} else {
			flat, err := strconv.Atoi(clean[m[8]:m[9]])
			if err != nil || flat > 1000000 {
				return nil, fmt.Errorf("can't read dice %q", expr)
			}
			t.flat = flat
		}
		terms = append(terms, t)
	}
	if pos != len(clean) || len(terms) == 0 {
		return nil, fmt.Errorf("can't read dice %q", expr)
	}
	if len(terms) > maxDiceTerms || dice > maxDiceCount {
		return nil, fmt.Errorf("that's too many dice")
	}
	return terms, nil
}

// Result is a finished roll.
type Result struct {
	Expression string
	Detail     string
	Total      int
}

// RollDice rolls an expression such as 2d6+1d4-1. A critical doubles the dice.
func RollDice(expr string, critical bool) (Result, error) {
	terms, err := parseDice(expr)
	if err != nil {
		return Result{}, err
	}
	total := 0
	var parts []string
	for i, t := range terms {
		var text string
		if t.sides == 0 {
			total += t.sign * t.flat
			text = strconv.Itoa(t.flat)
		} else {
			count := t.count
			if critical {
				count *= 2
			}
			rolls := make([]string, count)
			for j := 0; j < count; j++ {
				n := Die(t.sides)
				total += t.sign * n
				rolls[j] = strconv.Itoa(n)
			}
			text = "[" + strings.Join(rolls, ", ") + "]"
		}
		switch {
		case t.sign < 0:
			text = "- " + text
		case i > 0:
			text = "+ " + text
		}
		parts = append(parts, text)
	}
	return Result{Expression: strings.ReplaceAll(expr, " ", ""), Detail: strings.Join(parts, " "), Total: total}, nil
}

// RollD20 makes a d20 check with a modifier. Mode is "adv", "dis" or anything
// else for a normal roll.
func RollD20(mod int, mode string) Result {
	first := Die(20)
	kept := first
	expr := fmt.Sprintf("1d20%+d", mod)
	detail := fmt.Sprintf("[%d] %+d", first, mod)
	switch mode {
	case "adv", "dis":
		second := Die(20)
		if (mode == "adv" && second > first) || (mode == "dis" && second < first) {
			kept = second
		}
		name := "advantage"
		if mode == "dis" {
			name = "disadvantage"
		}
		expr = fmt.Sprintf("2d20%+d, %s", mod, name)
		detail = fmt.Sprintf("[%d, %d] kept %d %+d", first, second, kept, mod)
	}
	switch kept {
	case 20:
		detail += " (natural 20)"
	case 1:
		detail += " (natural 1)"
	}
	return Result{Expression: expr, Detail: detail, Total: kept + mod}
}

// Roll4d6 rolls a set of ability scores, 4d6 dropping the lowest for each.
func Roll4d6() [6]int {
	var out [6]int
	for i := range out {
		lowest, sum := 7, 0
		for j := 0; j < 4; j++ {
			n := Die(6)
			sum += n
			if n < lowest {
				lowest = n
			}
		}
		out[i] = sum - lowest
	}
	return out
}

// ValidateDice reports whether an expression can be rolled.
func ValidateDice(expr string) error {
	_, err := parseDice(expr)
	return err
}
