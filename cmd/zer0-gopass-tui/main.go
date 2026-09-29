package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"

	"github.com/andrey-losikhin/zer0-gopass-tui/internal/app"
	"github.com/andrey-losikhin/zer0-gopass-tui/internal/gopass"
)

func main() {
	if len(os.Args) > 1 {
		if os.Args[1] != "gc" || len(os.Args) > 2 {
			fmt.Fprintln(os.Stderr, "usage: zer0-gopass-tui [gc]")
			os.Exit(2)
		}
		os.Exit(runGC(os.Stdin, os.Stdout))
	}
	if err := app.Run(os.Stdin, os.Stdout, gopass.ExecLister{}, gopass.ExecReader{}, gopass.ExecWriter{}); err != nil {
		fmt.Fprintln(os.Stderr, "zer0-gopass-tui: internal error")
		os.Exit(1)
	}
}

// runGC выводит только счётчики и opaque-пути служебного namespace и удаляет
// мусор лишь после явного ответа "y".
func runGC(stdin io.Reader, stdout io.Writer) int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	input := bufio.NewReader(stdin)
	confirmed := false
	confirm := func(report gopass.GCReport) bool {
		fmt.Fprintf(stdout, "Manifest: %d\n", report.Manifests)
		printGroup(stdout, "Каталоги bundle без manifest", report.OrphanBundles)
		printGroup(stdout, "Устаревшие revision", report.OrphanRevisions)
		printGroup(stdout, "Value-файлы без ссылки", report.OrphanValues)
		printGroup(stdout, "Пустые каталоги", report.EmptyDirs)
		fmt.Fprint(stdout, "Удалить перечисленное? [y/N] ")
		answer, _ := input.ReadString('\n')
		confirmed = strings.TrimSpace(strings.ToLower(answer)) == "y"
		return confirmed
	}
	report, result, err := gopass.ExecWriter{}.GarbageCollect(ctx, confirm)
	if len(report.DetachedManifests) > 0 {
		fmt.Fprintf(stdout, "Manifest без основной записи (не удаляются, проверьте вручную): %d\n", len(report.DetachedManifests))
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "zer0-gopass-tui gc: %v\n", err)
		return 1
	}
	if report.Empty() {
		fmt.Fprintln(stdout, "Мусора не найдено.")
		return 0
	}
	if !confirmed {
		fmt.Fprintln(stdout, "Отменено, ничего не удалено.")
		return 0
	}
	fmt.Fprintf(stdout, "Удалено: %d, ошибок: %d\n", result.Removed, result.Failed)
	if result.Failed > 0 {
		return 1
	}
	return 0
}

func printGroup(stdout io.Writer, title string, paths []string) {
	fmt.Fprintf(stdout, "%s: %d\n", title, len(paths))
	for _, path := range paths {
		fmt.Fprintf(stdout, "  %s\n", path)
	}
}
