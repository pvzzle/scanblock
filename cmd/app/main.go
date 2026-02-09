package main

import (
	"log"

	"github.com/pvzzle/scanblock/internal/app"
)

// Проталкиваем конфиги и запускаем апку тут. Мусорить запрещено!
func main() {
	err := app.Run()
	if err != nil {
		log.Fatal(err)
	}
}
