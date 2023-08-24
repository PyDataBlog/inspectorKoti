package err

import (
	"log"
)

var debugMode bool


func SetDebugMode(d bool) {
	debugMode = d
}

func DebugPrint(v ...interface{}) {
	if debugMode {
		log.Println(v...)
	}

}
