package ui

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/gdamore/tcell/v2"
	"github.com/jd/devctl/cluster"
	"github.com/jd/devctl/config"
	"github.com/jd/devctl/env"
	"github.com/jd/devctl/logger"
	"github.com/jd/devctl/ssh"
	"github.com/rivo/tview"
)

type UI struct {
	app            *tview.Application
	pages          *tview.Pages
	envManager     *env.EnvManager
	clusterManager *cluster.ClusterManager
	currentEnv     string
	currentEnvID   string
	log            *logger.Logger
}

func NewUI(cfg *config.Config, log *logger.Logger) *UI {
	return &UI{
		app:        tview.NewApplication(),
		pages:      tview.NewPages(),
		envManager: env.NewEnvManager(cfg, log),
		log:        log,
	}
}

func (ui *UI) Run() error {
	ui.setupPages()
	return ui.app.SetRoot(ui.pages, true).EnableMouse(true).Run()
}

func (ui *UI) setupPages() {
	ui.pages.AddPage("envList", ui.createEnvListPage(), true, true)
}

func (ui *UI) createEnvListPage() tview.Primitive {
	envs := ui.envManager.ListEnvironments()
	selectedRow := 1

	table := tview.NewTable().
		SetBorders(false).
		SetSeparator(tview.Borders.Vertical)
	header := []string{"Name", "ID", "IP", "User", "Created", "Updated"}
	for i, title := range header {
		table.SetCell(0, i, tview.NewTableCell(title).SetTextColor(tcell.ColorYellow).SetExpansion(1.0))
	}

	refreshTable := func() {
		for i, env := range envs {
			cells := []string{env.Name, env.ID, env.IP, env.User, env.CreateTime, env.UpdateTime}
			for j, cell := range cells {
				tableCell := tview.NewTableCell(cell)
				if i+1 == selectedRow {
					tableCell.SetTextColor(tcell.ColorBlack).SetBackgroundColor(tcell.ColorWhite)
				} else {
					tableCell.SetTextColor(tcell.ColorWhite).SetBackgroundColor(tcell.ColorBlack)
				}
				table.SetCell(i+1, j, tableCell)
			}
		}
	}

	refreshTable()

	table.Select(selectedRow, 0).SetFixed(1, 0).SetDoneFunc(func(key tcell.Key) {
		if key == tcell.KeyEscape {
			ui.app.Stop()
		}
	}).SetSelectedFunc(func(row, column int) {
		if row > 0 && row <= len(envs) {
			ui.currentEnv = envs[row-1].Name
			ui.currentEnvID = envs[row-1].ID
			ui.clusterManager = cluster.NewClusterManager(ui.currentEnvID, ui.envManager.Config, ui.log)
			ui.showClusterListPage()
		}
	})

	table.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyRune:
			switch event.Rune() {
			case 'c':
				ui.showAddEnvironmentForm()
			case 'd':
				if selectedRow > 0 && selectedRow <= len(envs) {
					if envs[selectedRow-1].ID != "default" {
						ui.deleteSelectedEnvironment(table)
					}
				}
			case 'm':
				if selectedRow > 0 && selectedRow <= len(envs) {
					if envs[selectedRow-1].ID != "default" {
						ui.showUpdateEnvironmentForm(table)
					}
				}
			case 's':
				if selectedRow > 0 && selectedRow <= len(envs) {
					if envs[selectedRow-1].ID != "default" {
						ui.sshToEnvironment(envs[selectedRow-1])
					}
				}
			}
		case tcell.KeyUp:
			if selectedRow > 1 {
				selectedRow--
				refreshTable()
			}
		case tcell.KeyDown:
			if selectedRow < len(envs) {
				selectedRow++
				refreshTable()
			}
		case tcell.KeyEnter:
			if selectedRow > 0 && selectedRow <= len(envs) {
				ui.currentEnv = envs[selectedRow-1].Name
				ui.currentEnvID = envs[selectedRow-1].ID
				ui.clusterManager = cluster.NewClusterManager(ui.currentEnvID, ui.envManager.Config, ui.log)
				ui.showClusterListPage()
			}
		}
		return event
	})

	title := fmt.Sprintf("环境列表 (%d)", len(envs))
	frame := tview.NewFrame(table).
		SetBorders(1, 1, 1, 1, 1, 1).
		AddText(title, true, tview.AlignCenter, tcell.ColorWhite)

	infoBar := ui.createInfoBar()

	flex := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(infoBar, 5, 1, false).
		AddItem(frame, 0, 1, true)

	return flex
}

func (ui *UI) createInfoBar() tview.Primitive {
	info := fmt.Sprintf("DevCtl: v1.0.0\nCPU: %d%%\nMEM: %d%%", 7, 38) // Replace with actual CPU and MEM usage
	help := strings.Builder{}
	help.WriteString("操作指南:\n")
	help.WriteString("c: 创建环境\n")
	help.WriteString("d: 删除环境\n")
	help.WriteString("m: 修改环境\n")
	help.WriteString("s: 登录跳板机\n")
	help.WriteString("Enter: 进入集群列表\n")
	help.WriteString("Esc: 退出\n")
	banner := ui.loadBanner()

	grid := tview.NewGrid().
		SetColumns(30, 0, 30).
		SetRows(5).
		AddItem(tview.NewTextView().SetText(info).SetTextAlign(tview.AlignLeft), 0, 0, 1, 1, 0, 0, false).
		AddItem(tview.NewTextView().SetText(help.String()).SetTextAlign(tview.AlignCenter), 0, 1, 1, 1, 0, 0, false).
		AddItem(tview.NewTextView().SetText(banner).SetTextAlign(tview.AlignRight), 0, 2, 1, 1, 0, 0, false)

	return grid
}

func (ui *UI) createClusterInfoBar() tview.Primitive {
	info := fmt.Sprintf("DevCtl: v1.0.0\nCPU: %d%%\nMEM: %d%%", 7, 38) // Replace with actual CPU and MEM usage
	help := strings.Builder{}
	help.WriteString("操作说明:\n")
	help.WriteString("q: 查询\n")
	help.WriteString("Enter: 进入k9s界面\n")
	help.WriteString("Esc: 退出\n")

	banner := ui.loadBanner()

	grid := tview.NewGrid().
		SetColumns(30, 0, 30).
		SetRows(5).
		AddItem(tview.NewTextView().SetText(info).SetTextAlign(tview.AlignLeft), 0, 0, 1, 1, 0, 0, false).
		AddItem(tview.NewTextView().SetText(help.String()).SetTextAlign(tview.AlignCenter), 0, 1, 1, 1, 0, 0, false).
		AddItem(tview.NewTextView().SetText(banner).SetTextAlign(tview.AlignRight), 0, 2, 1, 1, 0, 0, false)

	return grid
}

func (ui *UI) loadBanner() string {
	content, err := os.ReadFile("banner.txt")
	if err != nil {
		ui.log.Error("Failed to load banner: %v", err)
		return "DevCtl"
	}
	return string(content)
}

func (ui *UI) deleteSelectedEnvironment(table *tview.Table) {
	row, _ := table.GetSelection()
	if row == 0 {
		return // This is the header row
	}

	envName := table.GetCell(row, 0).Text
	envID := table.GetCell(row, 1).Text

	modal := tview.NewModal().
		SetText(fmt.Sprintf("Are you sure you want to delete environment %s?", envName)).
		AddButtons([]string{"Yes", "No"}).
		SetDoneFunc(func(buttonIndex int, buttonLabel string) {
			if buttonLabel == "Yes" {
				if err := ui.envManager.DeleteEnvironment(envID); err != nil {
					ui.handleError(err, "Failed to delete environment")
				} else {
					ui.showSuccessModal("Environment deleted successfully")
					ui.setupPages() // Refresh the environment list
				}
			}
			ui.pages.RemovePage("deleteConfirm")
		})

	ui.pages.AddPage("deleteConfirm", modal, true, true)
}

func (ui *UI) showUpdateEnvironmentForm(table *tview.Table) {
	row, _ := table.GetSelection()
	if row == 0 {
		return // This is the header row
	}

	envID := table.GetCell(row, 1).Text
	env, err := ui.envManager.GetEnvironment(envID)
	if err != nil {
		ui.handleError(err, "Failed to get environment")
		return
	}

	form := tview.NewForm()

	form.AddInputField("IP", env.IP, 20, nil, func(text string) {
		env.IP = text
	})
	form.AddPasswordField("Password", env.Password, 20, '*', func(text string) {
		env.Password = text
	})

	form.AddButton("Save", func() {
		if err := ui.envManager.UpdateEnvironment(env); err != nil {
			ui.handleError(err, "Failed to update environment")
		} else {
			ui.showSuccessModal("Environment updated successfully")
			ui.pages.RemovePage("updateEnv")
			ui.setupPages() // Refresh the environment list
		}
	})
	form.AddButton("Cancel", func() {
		ui.pages.RemovePage("updateEnv")
	})

	ui.pages.AddPage("updateEnv", ui.modal(form, 60, 10), true, true)
}

func (ui *UI) showAddEnvironmentForm() {
	form := tview.NewForm()
	var env config.Environment

	form.AddInputField("Name", "", 20, nil, func(text string) {
		env.Name = text
	})
	form.AddInputField("ID", "", 20, nil, func(text string) {
		env.ID = text
	})
	form.AddInputField("IP", "", 20, nil, func(text string) {
		env.IP = text
	})
	form.AddInputField("User", "", 20, nil, func(text string) {
		env.User = text
	})
	form.AddPasswordField("Password", "", 20, '*', func(text string) {
		env.Password = text
	})

	form.AddButton("Test Connection", func() {
		sshClient := ssh.NewSSHClient(env.IP, env.User, env.Password)
		if err := sshClient.TestConnection(); err != nil {
			ui.handleError(err, "Connection test failed")
		} else {
			ui.showSuccessModal("Connection test successful")
		}
	})

	form.AddButton("Save", func() {
		sshClient := ssh.NewSSHClient(env.IP, env.User, env.Password)
		if err := sshClient.TestConnection(); err != nil {
			ui.handleError(err, "Connection test failed")
			return
		}

		if err := ui.envManager.AddEnvironment(env); err != nil {
			ui.handleError(err, "Failed to add environment")
		} else {
			ui.showSuccessModal("Environment added successfully")
			ui.pages.RemovePage("addEnv")
			ui.setupPages() // Refresh the environment list
		}
	})
	form.AddButton("Cancel", func() {
		ui.pages.RemovePage("addEnv")
	})

	ui.pages.AddPage("addEnv", ui.modal(form, 60, 20), true, true)
}

func (ui *UI) handleError(err error, context string) {
	ui.log.Error("Error in %s: %v", context, err)
	ui.showErrorModal(fmt.Sprintf("%s: %v", context, err))
}

func (ui *UI) showSuccessModal(message string) {
	ui.log.Info("Success: %s", message)
	modal := tview.NewModal().
		SetText(message).
		AddButtons([]string{"OK"}).
		SetDoneFunc(func(buttonIndex int, buttonLabel string) {
			ui.pages.RemovePage("success")
		})

	ui.pages.AddPage("success", modal, true, true)
}

func (ui *UI) showErrorModal(message string) {
	ui.log.Error(message)
	modal := tview.NewModal().
		SetText(message).
		AddButtons([]string{"OK"}).
		SetDoneFunc(func(buttonIndex int, buttonLabel string) {
			ui.pages.RemovePage("error")
		})

	ui.pages.AddPage("error", modal, true, true)
}

func (ui *UI) showInfoModal(message string) {
	ui.log.Info(message)
	modal := tview.NewModal().
		SetText(message).
		AddButtons([]string{"OK"}).
		SetDoneFunc(func(buttonIndex int, buttonLabel string) {
			ui.pages.RemovePage("info")
		})

	ui.pages.AddPage("info", modal, true, true)
}

func (ui *UI) modal(p tview.Primitive, width, height int) tview.Primitive {
	return tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(nil, 0, 1, false).
			AddItem(p, height, 1, true).
			AddItem(nil, 0, 1, false), width, 1, true).
		AddItem(nil, 0, 1, false)
}

func (ui *UI) showClusterListPage() {
	ui.log.Info("Showing cluster list page")
	clusters, err := ui.clusterManager.ListClusters()
	if err != nil {
		ui.handleError(err, "Error listing clusters")
		return
	}

	table := tview.NewTable().
		SetBorders(true)

	headers := []string{"Cluster ID", "Cluster Name", "OS", "ARCH", "VERSION", "CRI", "Status"}
	for i, header := range headers {
		table.SetCell(0, i, tview.NewTableCell(header).SetTextColor(tcell.ColorYellow).SetExpansion(1.0))
	}
	selectedRow := 1

	var filteredClusters []cluster.ClusterInfo
	filterClusters := func(query string) {
		filteredClusters = make([]cluster.ClusterInfo, 0)
		for _, c := range clusters {
			if strings.Contains(strings.ToLower(c.Name), strings.ToLower(query)) {
				filteredClusters = append(filteredClusters, c)
			}
		}
	}

	refreshTable := func() {
		table.Clear()
		for i, header := range headers {
			table.SetCell(0, i, tview.NewTableCell(header).SetTextColor(tcell.ColorYellow).SetExpansion(1.0))
		}
		clustersToShow := clusters
		if len(filteredClusters) > 0 {
			clustersToShow = filteredClusters
		}
		for i, cluster := range clustersToShow {
			cells := []string{cluster.ID, cluster.Name, cluster.OS, cluster.ARCH, cluster.Version, cluster.Cri, cluster.Status}
			for j, cell := range cells {
				tableCell := tview.NewTableCell(cell)
				if i+1 == selectedRow {
					tableCell.SetTextColor(tcell.ColorBlack).SetBackgroundColor(tcell.ColorWhite)
				} else {
					tableCell.SetTextColor(tcell.ColorWhite).SetBackgroundColor(tcell.ColorBlack)
				}
				table.SetCell(i+1, j, tableCell)
			}
		}
	}

	refreshTable()

	table.Select(selectedRow, 0).SetFixed(1, 0).SetDoneFunc(func(key tcell.Key) {
		if key == tcell.KeyEscape {
			ui.pages.SwitchToPage("envList")
		}
	})

	showSearchBox := func() {
		inputField := tview.NewInputField().
			SetLabel("搜索集群: ").
			SetFieldWidth(30)

		inputField.SetDoneFunc(func(key tcell.Key) {
			if key == tcell.KeyEnter {
				query := inputField.GetText()
				filterClusters(query)
				refreshTable()
				ui.pages.RemovePage("searchBox")
				ui.app.SetFocus(table)
			}
		})

		searchBox := ui.modal(inputField, 50, 3)
		ui.pages.AddPage("searchBox", searchBox, true, true)
		ui.app.SetFocus(inputField)
	}

	table.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyRune:
			switch event.Rune() {
			case 'a':
				ui.showAddClusterForm()
			case 'd':
				ui.deleteSelectedCluster(table)
			case 'e':
				ui.editSelectedCluster(table)
			case 'q':
				showSearchBox()
			}
		case tcell.KeyUp:
			if selectedRow > 1 {
				selectedRow--
				refreshTable()
			}
		case tcell.KeyDown:
			if selectedRow < len(clusters) {
				selectedRow++
				refreshTable()
			}
		case tcell.KeyEnter:
			if selectedRow > 0 && selectedRow <= len(clusters) {
				clusterInfo := clusters[selectedRow-1]
				ui.openK9s(clusterInfo.ID)
			}
		}
		return event
	})

	title := fmt.Sprintf("集群列表 - %s (%d)", ui.currentEnv, len(clusters))
	frame := tview.NewFrame(table).
		SetBorders(0, 0, 0, 0, 0, 0).
		AddText(title, true, tview.AlignCenter, tcell.ColorWhite)

	infoBar := ui.createClusterInfoBar()

	flex := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(infoBar, 5, 1, false).
		AddItem(frame, 0, 1, true)

	ui.pages.AddPage("clusterList", flex, true, true)
	ui.pages.SwitchToPage("clusterList")
}

func (ui *UI) openK9s(clusterName string) {
	var kubeconfigPath string
	var err error

	if clusterName == "gaia" {
		kubeconfigPath = filepath.Join(os.Getenv("HOME"), ".devctl", "kubeconfigs", ui.currentEnvID, "config")
	} else {
		kubeconfigPath, err = ui.clusterManager.GetKubeconfig(clusterName)
		if err != nil {
			ui.handleError(err, "Error getting kubeconfig")
			return
		}
	}

	if _, err := os.Stat(kubeconfigPath); os.IsNotExist(err) {
		ui.handleError(err, fmt.Sprintf("Kubeconfig file not found for cluster %s", clusterName))
		return
	}
	ctx := context.Background()
	childCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		fmt.Println("Received interrupt signal, canceling context")
		cancel()
	}()

	cmd := exec.CommandContext(childCtx, "k9s", "--kubeconfig", kubeconfigPath, "--logLevel", "debug")
	cmd.Env = append(os.Environ(), "EDITOR=vim")
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	ui.app.Suspend(func() {
		err := cmd.Run()
		if err != nil {
			ui.handleError(err, "k9s command failed")
		}
	})
}

func (ui *UI) deleteSelectedCluster(table *tview.Table) {
	row, _ := table.GetSelection()
	if row == 0 {
		return // This is the header row
	}

	clusterName := table.GetCell(row, 0).Text

	if clusterName == "gaia" {
		ui.showErrorModal("Cannot delete management cluster (gaia)")
		return
	}

	modal := tview.NewModal().
		SetText(fmt.Sprintf("Are you sure you want to delete cluster %s?", clusterName)).
		AddButtons([]string{"Yes", "No"}).
		SetDoneFunc(func(buttonIndex int, buttonLabel string) {
			if buttonLabel == "Yes" {
				err := ui.clusterManager.DeleteCluster(clusterName)
				if err != nil {
					ui.handleError(err, fmt.Sprintf("Failed to delete cluster %s", clusterName))
				} else {
					ui.showSuccessModal(fmt.Sprintf("Cluster %s deleted successfully", clusterName))
					ui.showClusterListPage() // Refresh the cluster list
				}
			}
			ui.pages.RemovePage("deleteClusterConfirm")
		})

	ui.pages.AddPage("deleteClusterConfirm", modal, true, true)
}

func (ui *UI) editSelectedCluster(table *tview.Table) {
	row, _ := table.GetSelection()
	if row == 0 {
		return // This is the header row
	}

	clusterName := table.GetCell(row, 0).Text

	kubeconfigPath, err := ui.clusterManager.GetKubeconfig(clusterName)
	if err != nil {
		ui.handleError(err, "Error getting kubeconfig")
		return
	}

	cmd := exec.Command("vim", kubeconfigPath)
	if err := cmd.Start(); err != nil {
		ui.handleError(err, "Error opening vim")
		return
	}

	ui.app.Suspend(func() {
		cmd.Wait()
	})
}

func (ui *UI) showAddClusterForm() {
	form := tview.NewForm()
	var clusterName, kubeconfig string

	form.AddInputField("Cluster Name", "", 20, nil, func(text string) {
		clusterName = text
	})
	form.AddInputField("Kubeconfig", "", 40, nil, func(text string) {
		kubeconfig = text
	})

	form.AddButton("Save", func() {
		if err := ui.clusterManager.AddCluster(clusterName, kubeconfig); err != nil {
			ui.handleError(err, "Failed to add cluster")
		} else {
			ui.showSuccessModal(fmt.Sprintf("Cluster %s added successfully", clusterName))
			ui.pages.RemovePage("addCluster")
			ui.showClusterListPage() // Refresh the cluster list
		}
	})
	form.AddButton("Cancel", func() {
		ui.pages.RemovePage("addCluster")
	})

	ui.pages.AddPage("addCluster", ui.modal(form, 60, 10), true, true)
}

func (ui *UI) sshToEnvironment(env config.Environment) {
	ui.log.Info("Connecting to environment: %s", env.Name)

	cmd := exec.Command("sshpass", "-p", env.Password, "ssh", "-o", "StrictHostKeyChecking=no", "-o", "UserKnownHostsFile=/dev/null", fmt.Sprintf("%s@%s", env.User, env.IP))
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	ui.app.Suspend(func() {
		err := cmd.Run()
		if err != nil {
			ui.log.Error("Failed to connect to environment: %v", err)
		}
	})
}
