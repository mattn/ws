package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strconv"
)

type Op int

const (
	Push Op = iota
	Signed
	Dup
	Copy
	Swap
	Discard
	Slide
	Add
	Sub
	Mul
	Div
	Mod
	Store
	Retrieve
	Mark
	Unsigned
	Call
	Jump
	Jz
	Jn
	Ret
	Exit
	OutC
	OutN
	InC
	InN
	Nop
)

type opcode struct {
	code Op
	arg  int
}

type opcodes []opcode

func (p *opcodes) jmp(arg int, pc *int) {
	for i, t := range *p {
		if t.code == Mark && t.arg == arg {
			*pc = i
		}
	}
}

var optable = map[string]Op{
	"  ":       Push,
	" \n ":     Dup,
	" \n\t":    Swap,
	" \n\n":    Discard,
	"\t   ":    Add,
	"\t  \t":   Sub,
	"\t  \n":   Mul,
	"\t \t ":   Div,
	"\t \t\t":  Mod,
	"\t\t ":    Store,
	"\t\t\t":   Retrieve,
	"\n  ":     Mark,
	"\n \t":    Call,
	"\n \n":    Jump,
	"\n\t ":    Jz,
	"\n\t\t":   Jn,
	"\n\t\n":   Ret,
	"\n\n\n":   Exit,
	"\t\n  ":   OutC,
	"\t\n \t":  OutN,
	"\t\n\t ":  InC,
	"\t\n\t\t": InN,
	//" \t ":     Copy,
	//" \t\n":    Slide,
}

type stack []int

func (s *stack) push(v int) {
	*s = append(*s, v)
}

func (s *stack) pop() int {
	l := len(*s)
	if l == 0 {
		return -1
	}
	v := (*s)[l-1]
	*s = (*s)[:l-1]
	return v
}

func (s *stack) dup() {
	l := len(*s)
	if l == 0 {
		return
	}
	v := (*s)[l-1]
	*s = append(*s, v)
}

func whitespace(src []byte) []byte {
	// remove needless comments
	for {
		pos := bytes.IndexFunc(src, func(r rune) bool {
			return r != ' ' && r != '\t' && r != '\n'
		})
		if pos < 0 {
			break
		}
		if pos == 0 {
			src = src[1:]
		} else {
			src = append(src[:pos], src[pos+1:]...)
		}
	}

	// parse whitespace into tokens
	tokens := opcodes{}
	for len(src) > 0 {
		op := ""
		code := Nop
		for k, v := range optable {
			if bytes.HasPrefix(src, []byte(k)) {
				op = k
				code = v
				break
			}
		}
		if op == "" {
			src = src[1:]
			continue
		}
		src = src[len(op):]
		var arg int
		switch code {
		case Push:
			// handle argument
		handle_signed_arg:
			for i := 1; i < len(src); i++ {
				switch src[i] {
				case ' ':
					arg = (arg << 1) | 0
				case '\t':
					arg = (arg << 1) | 1
				case '\n':
					// Push take singed argument
					if src[0] == '\t' {
						arg = -arg
					}
					src = src[i+1:]
					break handle_signed_arg
				}
			}
		case Mark, Call, Jump, Jz, Jn:
			// handle argument
		handle_unsigned_arg:
			for i := 0; i < len(src); i++ {
				switch src[i] {
				case ' ':
					arg = (arg << 1) | 0
				case '\t':
					arg = (arg << 1) | 1
				case '\n':
					src = src[i+1:]
					break handle_unsigned_arg
				}
			}
		}
		tokens = append(tokens, opcode{code, arg})
	}

	pc := 0
	ps := stack{}
	cs := stack{}
	heap := map[int]int{}
	for {
		token := tokens[pc]

		code, arg := token.code, token.arg
		//fmt.Println(pc, code, arg)
		pc++
		switch code {
		case Push:
			ps.push(arg)
		case Mark:
		case Dup:
			ps.dup()
		case OutN:
			fmt.Print(ps.pop())
		case OutC:
			fmt.Print(string(rune(ps.pop())))
		case Add:
			rhs := ps.pop()
			lhs := ps.pop()
			ps.push(lhs + rhs)
		case Sub:
			rhs := ps.pop()
			lhs := ps.pop()
			ps.push(lhs - rhs)
		case Mul:
			rhs := ps.pop()
			lhs := ps.pop()
			ps.push(lhs * rhs)
		case Div:
			rhs := ps.pop()
			lhs := ps.pop()
			ps.push(lhs / rhs)
		case Mod:
			rhs := ps.pop()
			lhs := ps.pop()
			ps.push(lhs % rhs)
		case Jz:
			if ps.pop() == 0 {
				tokens.jmp(arg, &pc)
			}
		case Jn:
			if ps.pop() < 0 {
				tokens.jmp(arg, &pc)
			}
		case Jump:
			tokens.jmp(arg, &pc)
		case Discard:
			ps.pop()
		case Exit:
			os.Exit(0)
		case Store:
			v := ps.pop()
			address := ps.pop()
			heap[address] = v
		case Call:
			cs.push(pc)
			tokens.jmp(arg, &pc)
		case Retrieve:
			ps.push(heap[ps.pop()])
		case Ret:
			pc = cs.pop()
		case InC:
			var b [1]byte
			os.Stdin.Read(b[:])
			heap[ps.pop()] = int(b[0])
		case InN:
			scanner := bufio.NewScanner(os.Stdin)
			if scanner.Scan() {
				i, _ := strconv.Atoi(scanner.Text())
				heap[ps.pop()] = i
			}
		case Swap:
			ps[len(ps)-1], ps[len(ps)-2] = ps[len(ps)-2], ps[len(ps)-1]
		default:
			panic(fmt.Sprintf("Unknown opcode: %v", code))
		}
	}
}

func main() {
	var content bytes.Buffer

	for _, f := range os.Args[1:] {
		b, err := ioutil.ReadFile(f)
		if err != nil {
			fmt.Fprintln(os.Stderr, "read file:", err)
			os.Exit(1)
		}
		content.Write(b)
	}

	if content.Len() == 0 {
		if _, err := io.Copy(&content, os.Stdin); err != nil {
			fmt.Fprintln(os.Stderr, "read stdin:", err)
			os.Exit(1)
		}
	}

	whitespace(content.Bytes())
}
