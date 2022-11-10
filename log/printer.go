package log

import "fmt"

type Printer interface {
	Print(lvl Level, msg string)
}

func Normal(p Printer, format string, v ...interface{}) {
	if leveler, ok := p.(Leveler); ok && leveler.Level() < NormalLevel {
		return
	}
	print(NormalLevel, p, format, v...)
}

func Verbose(p Printer, format string, v ...interface{}) {
	if leveler, ok := p.(Leveler); !ok || leveler.Level() < VerboseLevel {
		return
	}
	print(VerboseLevel, p, format, v...)
}

func print(lvl Level, p Printer, format string, v ...interface{}) {
	p.Print(lvl, fmt.Sprintf(format, v...))
}
