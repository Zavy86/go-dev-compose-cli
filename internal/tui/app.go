package tui

import (
	"context"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/Zavy86/go-dev-compose-cli/internal/config"
	"github.com/Zavy86/go-dev-compose-cli/internal/docker"
)

const projectSelectionKey = "__ALL__"

type ServiceState struct {
	Info   config.ServiceInfo
	Status string
}

type UI struct {
	App             *tview.Application
	List            *tview.List
	Logs            *tview.TextView
	Flex            *tview.Flex
	Services        []*ServiceState
	DockerClient    *docker.Client
	selectedService string

	logCancelCtx context.CancelFunc
}

func NewUI(services []config.ServiceInfo, statusMsg string, dockerCli *docker.Client) *UI {
	ui := &UI{
		App:          tview.NewApplication(),
		List:         tview.NewList().ShowSecondaryText(true),
		Logs:         tview.NewTextView().SetDynamicColors(true).SetScrollable(true),
		DockerClient: dockerCli,
	}

	for _, s := range services {
		ui.Services = append(ui.Services, &ServiceState{
			Info:   s,
			Status: "offline",
		})
	}

	ui.setupLayout(statusMsg)
	ui.setupEvents()

	ui.refreshStatuses()
	ui.renderServices()

	go ui.startStatusPolling()

	ui.selectedService = projectSelectionKey
	ui.attachLogs(ui.selectedService)

	return ui
}

func (ui *UI) setupLayout(statusMsg string) {
	ui.List.SetBorder(true).SetTitle(" SERVICES ")
	ui.Logs.SetBorder(true).SetTitle(" LOGS ")
	ui.Logs.SetText(statusMsg)

	projectName := "-"
	coposeFile := "-"
	if ui.DockerClient != nil {
		projectName = fmt.Sprintf("[green]%s[-]", ui.DockerClient.ProjectName())
		if path := ui.DockerClient.ComposePath(); path != "" {
			coposeFile = fmt.Sprintf("[blue]%s[-]", path)
		}
	}

	shortcuts := "[yellow]u[-]: Up | [yellow]d[-]: Down | [yellow]a[-]: Start All | [yellow]o[-]: Stop All | [yellow]s[-]: Start | [yellow]x[-]: Stop | [yellow]r[-]: Restart | [yellow]c[-]: Clear"
	footerText := fmt.Sprintf(" %s | %s | %s", projectName, shortcuts, coposeFile)

	footer := tview.NewTextView().
		SetDynamicColors(true).
		SetText(footerText)

	mainFlex := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(tview.NewFlex().
			AddItem(ui.List, 0, 1, true).
			AddItem(ui.Logs, 0, 2, false), 0, 1, true).
		AddItem(footer, 1, 0, false)

	ui.Flex = mainFlex
	ui.App.SetRoot(ui.Flex, true)
}

func (ui *UI) renderServices() {
	currentIndex := ui.List.GetCurrentItem()
	ui.List.Clear()

	if len(ui.Services) == 0 {
		ui.List.AddItem("No services found", "", 0, nil)
		return
	}

	for _, state := range ui.Services {
		var bullet string
		switch state.Status {
		case "running":
			bullet = "[green]●[-]"
		case "restarting", "created":
			bullet = "[yellow]●[-]"
		default:
			bullet = "[red]●[-]"
		}

		depsInfo := "dep: none"
		if len(state.Info.DependsOn) > 0 {
			depsInfo = fmt.Sprintf("deps: %s", strings.Join(state.Info.DependsOn, ", "))
		}

		label := fmt.Sprintf("%s %s", bullet, state.Info.Name)
		ui.List.AddItem(label, depsInfo, 0, nil)
	}

	if currentIndex >= 0 && currentIndex < ui.List.GetItemCount() {
		ui.List.SetCurrentItem(currentIndex)
	}
}

func (ui *UI) setupEvents() {
	ui.List.SetChangedFunc(func(index int, mainText string, secondaryText string, shortcut rune) {
		var target string
		if index == 0 {
			target = projectSelectionKey
		} else if index-1 >= 0 && index-1 < len(ui.Services) {
			target = ui.Services[index-1].Info.Name
		} else {
			return
		}

		if target != ui.selectedService {
			ui.selectedService = target
			ui.Logs.Clear()
			ui.attachLogs(target)
		}
	})

	ui.App.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Rune() {
		case 'u':
			ui.runComposeAction("UP", func(ctx context.Context, w io.Writer) error {
				if ui.DockerClient == nil {
					return fmt.Errorf("Docker client unavailable")
				}
				return ui.DockerClient.ComposeUp(ctx, w)
			})
			return nil

		case 'd':
			ui.runComposeAction("DOWN", func(ctx context.Context, w io.Writer) error {
				if ui.DockerClient == nil {
					return fmt.Errorf("Docker client unavailable")
				}
				return ui.DockerClient.ComposeDown(ctx, w)
			})
			return nil

		case 'a':
			ui.runMassAction("START ALL", func(ctx context.Context) error {
				if ui.DockerClient == nil {
					return fmt.Errorf("Docker client unavailable")
				}
				return ui.DockerClient.StartAllContainers(ctx)
			})
			return nil

		case 'o':
			ui.runMassAction("STOP ALL", func(ctx context.Context) error {
				if ui.DockerClient == nil {
					return fmt.Errorf("Docker client unavailable")
				}
				return ui.DockerClient.StopAllContainers(ctx)
			})
			return nil

		case 's':
			ui.runSingleContainerAction("START", func(ctx context.Context, svc string) error {
				if ui.DockerClient == nil {
					return fmt.Errorf("Docker client unavailable")
				}
				return ui.DockerClient.StartContainer(ctx, svc)
			})
			return nil

		case 'x':
			ui.runSingleContainerAction("STOP", func(ctx context.Context, svc string) error {
				if ui.DockerClient == nil {
					return fmt.Errorf("Docker client unavailable")
				}
				return ui.DockerClient.StopContainer(ctx, svc)
			})
			return nil

		case 'r':
			ui.runSingleContainerAction("RESTART", func(ctx context.Context, svc string) error {
				if ui.DockerClient == nil {
					return fmt.Errorf("Docker client unavailable")
				}
				return ui.DockerClient.RestartContainer(ctx, svc)
			})
			return nil

		case 'c':
			ui.Logs.Clear()
			return nil
		}
		return event
	})
}

func (ui *UI) refreshStatuses() bool {
	if ui.DockerClient == nil {
		return false
	}

	ctx := context.Background()

	isUp, _ := ui.DockerClient.IsComposeUp(ctx)
	if isUp {
		ui.List.SetTitle(" COMPOSE Status: [green]UP[-] ")
	} else {
		ui.List.SetTitle(" COMPOSE Status: [red]DOWN[-] ")
	}

	statuses, err := ui.DockerClient.GetServicesStatus(ctx)
	if err != nil {
		return false
	}

	changed := false
	for _, s := range ui.Services {
		newStatus := "offline"
		if st, ok := statuses[s.Info.Name]; ok {
			newStatus = st
		}

		if s.Status != newStatus {
			s.Status = newStatus
			changed = true
		}
	}

	return changed
}

func (ui *UI) startStatusPolling() {
	ticker := time.NewTicker(2 * time.Second)
	for range ticker.C {
		if ui.refreshStatuses() {
			ui.App.QueueUpdateDraw(func() {
				ui.renderServices()
			})
		}
	}
}

func (ui *UI) attachLogs(target string) {
	if ui.logCancelCtx != nil {
		ui.logCancelCtx()
	}

	if ui.DockerClient == nil {
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	ui.logCancelCtx = cancel

	if target == projectSelectionKey {
		fmt.Fprintf(ui.Logs, "[green]--- Streaming %s Logs ---[-]\n", ui.DockerClient.ProjectName())
	} else {
		fmt.Fprintf(ui.Logs, "[yellow]--- Streaming %s Logs ---[-]\n", target)
	}

	ansiWriter := tview.ANSIWriter(ui.Logs)
	uiWriter := &uiLogWriter{
		writer: ansiWriter,
		app:    ui.App,
		logs:   ui.Logs,
	}

	go func() {
		var err error
		if target == projectSelectionKey {
			err = ui.DockerClient.StreamAllLogs(ctx, uiWriter)
		} else {
			err = ui.DockerClient.StreamLogs(ctx, target, uiWriter)
		}

		if err != nil && ctx.Err() == nil {
			ui.App.QueueUpdateDraw(func() {
				fmt.Fprintf(ui.Logs, "\n[red]Stream log error: %v[-]\n", err)
			})
		}
	}()
}

func (ui *UI) runComposeAction(action string, fn func(ctx context.Context, w io.Writer) error) {
	fmt.Fprintf(ui.Logs, "\n[blue]=== Executing COMPOSE %s ===[-]\n", action)
	ui.Logs.ScrollToEnd()
	writer := tview.ANSIWriter(ui.Logs)

	go func() {
		err := fn(context.Background(), writer)
		ui.App.QueueUpdateDraw(func() {
			if err != nil {
				fmt.Fprintf(ui.Logs, "\n[red]Error %s: %v[-]\n", action, err)
			} else {
				fmt.Fprintf(ui.Logs, "\n[green] %s Completed.[-]\n", action)
			}
			ui.Logs.ScrollToEnd()
			ui.refreshStatuses()
			ui.renderServices()
			ui.attachLogs(ui.selectedService)
		})
	}()
}

func (ui *UI) runMassAction(action string, fn func(ctx context.Context) error) {
	fmt.Fprintf(ui.Logs, "\n[blue]Executing %s on all containers...[-]\n", action)
	ui.Logs.ScrollToEnd()

	go func() {
		err := fn(context.Background())
		ui.App.QueueUpdateDraw(func() {
			if err != nil {
				fmt.Fprintf(ui.Logs, "[red]%s failed: %v[-]\n", action, err)
			} else {
				fmt.Fprintf(ui.Logs, "[green]%s completed.[-]\n", action)
			}
			ui.Logs.ScrollToEnd()
			ui.refreshStatuses()
			ui.renderServices()
			ui.attachLogs(ui.selectedService)
		})
	}()
}

func (ui *UI) runSingleContainerAction(action string, fn func(ctx context.Context, svc string) error) {
	idx := ui.List.GetCurrentItem()

	if idx == 0 {
		fmt.Fprintf(ui.Logs, "\n[yellow]Select a single container to execute %s[-]\n", action)
		return
	}

	if idx-1 < 0 || idx-1 >= len(ui.Services) {
		return
	}

	svcName := ui.Services[idx-1].Info.Name
	fmt.Fprintf(ui.Logs, "\n[blue]Executing %s on [%s]...[-]\n", action, svcName)
	ui.Logs.ScrollToEnd()

	go func() {
		err := fn(context.Background(), svcName)
		ui.App.QueueUpdateDraw(func() {
			if err != nil {
				fmt.Fprintf(ui.Logs, "[red]%s %s failed: %v[-]\n", svcName, action, err)
			} else {
				fmt.Fprintf(ui.Logs, "[green]%s %s completed.[-]\n", svcName, action)
			}
			ui.Logs.ScrollToEnd()
			ui.refreshStatuses()
			ui.renderServices()
			ui.attachLogs(ui.selectedService)
		})
	}()
}

type uiLogWriter struct {
	writer io.Writer
	app    *tview.Application
	logs   *tview.TextView
	mu     sync.Mutex
}

func (w *uiLogWriter) Write(p []byte) (n int, err error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	n, err = w.writer.Write(p)
	if n > 0 {
		w.app.QueueUpdateDraw(func() {
			w.logs.ScrollToEnd()
		})
	}
	return n, err
}
