package log

import "fmt"

type Printer interface {
	Print(lvl Level, msg string)
}

func Normal(p Printer, format string, v ...interface{}) {
	if LevelOf(p) < NormalLevel {
		return
	}
	printMsg(NormalLevel, p, format, v...)
}

func Verbose(p Printer, format string, v ...interface{}) {
	if LevelOf(p) < VerboseLevel {
		return
	}
	printMsg(VerboseLevel, p, format, v...)
}

func printMsg(lvl Level, p Printer, format string, v ...interface{}) {
	p.Print(lvl, fmt.Sprintf(format, v...))
}

func LevelOf(p Printer) Level {
	if leveler, ok := p.(Leveler); ok {
		return leveler.Level()
	}
	return NormalLevel
}
