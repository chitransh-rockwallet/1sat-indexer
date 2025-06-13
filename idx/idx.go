package idx

import (
	"fmt"
	"log"
	"os"

	"github.com/GorillaPool/go-junglebus"
	"github.com/joho/godotenv"
)

var JUNGLEBUS string
var JB *junglebus.Client

var MempoolScore = HeightScore(50000000, 0)

func init() {
	wd, _ := os.Getwd()
	log.Println("CWD:", wd)
	godotenv.Load(fmt.Sprintf(`%s/../../.env`, wd))

	var err error
	JUNGLEBUS = os.Getenv("JUNGLEBUS")
	if JUNGLEBUS != "" {
		log.Println("JUNGLEBUS", JUNGLEBUS)
		JB, err = junglebus.New(
			junglebus.WithHTTP(JUNGLEBUS),
		)
		if err != nil {
			log.Panic(err)
		}
	}
}
