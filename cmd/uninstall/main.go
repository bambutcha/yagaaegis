// YagaAegis Uninstall — один бинарник для Linux и Windows.
//
//	Uninstall              — интерактивно (спросит подтверждение)
//	Uninstall -yes         — без вопросов, снести всё включая базы
//	Uninstall -yes -keep-data
//	Uninstall -dir /path   — указать каталог установки
package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/bambutcha/localguard/internal/uninstall"
	"github.com/bambutcha/localguard/internal/version"
)

func main() {
	yes := flag.Bool("yes", false, "без подтверждения")
	keep := flag.Bool("keep-data", false, "оставить базы/данные в каталоге пользователя")
	dirFlag := flag.String("dir", "", "каталог установки (по умолчанию — папка рядом с Uninstall)")
	flag.Parse()

	fmt.Printf("YagaAegis Uninstall %s\n", version.Label())

	dir := *dirFlag
	if dir == "" {
		d, err := uninstall.InstallDirFromExe()
		if err != nil {
			fail(err)
		}
		dir = d
	}

	ud := uninstall.UserDataDir()
	fmt.Printf("Каталог установки: %s\n", dir)
	if *keep {
		fmt.Println("Данные пользователя: оставить")
	} else {
		fmt.Printf("Данные пользователя: удалить (%s)\n", ud)
	}

	if !*yes {
		fmt.Print("Удалить YagaAegis? [y/N]: ")
		line, _ := bufio.NewReader(os.Stdin).ReadString('\n')
		line = strings.TrimSpace(strings.ToLower(line))
		if line != "y" && line != "yes" && line != "д" && line != "да" {
			fmt.Println("Отменено.")
			os.Exit(0)
		}
	}

	err := uninstall.Run(uninstall.Options{
		InstallDir:   dir,
		KeepUserData: *keep,
	}, func(_ float64, status string) {
		fmt.Println("…", status)
	})
	if err != nil {
		fail(err)
	}
	fmt.Println("YagaAegis удалён.")
}

func fail(err error) {
	fmt.Fprintf(os.Stderr, "ошибка: %v\n", err)
	os.Exit(1)
}
