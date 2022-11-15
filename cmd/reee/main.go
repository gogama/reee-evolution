package main

import "github.com/alexflint/go-arg"

type foo struct {
}

func main() {
	arg.MustParse(&foo{})
}
