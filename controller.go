package goclair

import (
	"fmt"
	"io"
	"os"
	"sync"
	"syscall"

	"github.com/jroimartin/gocui"
)

const columns = 4

type InstanceController struct {
	gui            *gocui.Gui
	newCursorX     int
	newCursorY     int
	cursorX        int
	cursorY        int
	instancesLock  sync.Mutex
	instances      []*Instance
	instancesByCol [columns][]*Instance
	curInstance    *Instance
	refreshNeeded  bool
}

func NewInstanceController() *InstanceController {
	return &InstanceController{
		refreshNeeded: true,
	}
}

func (ctrl *InstanceController) SetInstances(instances []*Instance) {
	ctrl.instancesLock.Lock()
	ctrl.instances = instances

	// Divide instances along columns
	numEachCol := int((len(ctrl.instances) + 1) / 4)
	for i := 0; i < columns; i++ {
		if i == columns-1 {
			ctrl.instancesByCol[i] = ctrl.instances[numEachCol*i:]
		} else {
			ctrl.instancesByCol[i] = ctrl.instances[numEachCol*i : numEachCol*(i+1)]
		}
	}
	ctrl.instancesLock.Unlock()
	ctrl.RefreshView()
}

func (ctrl *InstanceController) RefreshView() {
	ctrl.refreshNeeded = true
	ctrl.gui.Update(func(g *gocui.Gui) error {
		return nil
	})
}

func (ctrl *InstanceController) InitializeGui(g *gocui.Gui) error {
	ctrl.gui = g
	g.SetManagerFunc(ctrl.layout)

	if err := g.SetKeybinding("", gocui.KeyCtrlC, gocui.ModNone, ctrl.quit); err != nil {
		return err
	}
	if err := g.SetKeybinding("", gocui.KeyArrowUp, gocui.ModNone, ctrl.arrowUp); err != nil {
		return err
	}
	if err := g.SetKeybinding("", gocui.KeyArrowDown, gocui.ModNone, ctrl.arrowDown); err != nil {
		return err
	}
	if err := g.SetKeybinding("", gocui.KeyArrowLeft, gocui.ModNone, ctrl.arrowLeft); err != nil {
		return err
	}
	if err := g.SetKeybinding("", gocui.KeyArrowRight, gocui.ModNone, ctrl.arrowRight); err != nil {
		return err
	}
	if err := g.SetKeybinding("", gocui.KeySpace, gocui.ModNone, ctrl.space); err != nil {
		return err
	}
	if err := g.SetKeybinding("", gocui.KeyEnter, gocui.ModNone, ctrl.enter); err != nil {
		return err
	}

	return nil
}

func (ctrl *InstanceController) arrowUp(g *gocui.Gui, v *gocui.View) error {
	if ctrl.newCursorY > 0 {
		ctrl.newCursorY -= 1
	}
	return nil
}

func (ctrl *InstanceController) arrowDown(g *gocui.Gui, v *gocui.View) error {
	if ctrl.newCursorY < len(ctrl.instancesByCol[ctrl.newCursorX])-1 {
		ctrl.newCursorY += 1
	}
	return nil
}

func (ctrl *InstanceController) arrowLeft(g *gocui.Gui, v *gocui.View) error {
	if ctrl.newCursorX > 0 {
		ctrl.newCursorX -= 1
	}
	// Limit cursor movement in shorter columns
	if ctrl.newCursorY >= len(ctrl.instancesByCol[ctrl.newCursorX]) {
		ctrl.newCursorY = len(ctrl.instancesByCol[ctrl.newCursorX]) - 1
	}
	return nil
}

func (ctrl *InstanceController) arrowRight(g *gocui.Gui, v *gocui.View) error {
	if ctrl.newCursorX < columns-1 {
		ctrl.newCursorX += 1
	}
	// Limit cursor movement in shorter columns
	if ctrl.newCursorY >= len(ctrl.instancesByCol[ctrl.newCursorX]) {
		ctrl.newCursorY = len(ctrl.instancesByCol[ctrl.newCursorX]) - 1
	}
	return nil
}

func (ctrl *InstanceController) space(g *gocui.Gui, v *gocui.View) error {
	ctrl.curInstance.SetSelected(!ctrl.curInstance.Selected())
	ctrl.RefreshView()
	return nil
}

func (ctrl *InstanceController) enter(g *gocui.Gui, v *gocui.View) error {
	instances := ctrl.selectedInstances()
	if len(instances) == 0 {
		ctrl.curInstance.SetSelected(true)
		instances = ctrl.selectedInstances()
	}
	for _, instance := range instances {
		connCmd, err := instance.ConnectCommand()
		if err != nil {
			continue
		}
		g.Close()
		fmt.Println(connCmd)
		args := []string{"/bin/bash", "-c", connCmd}
		env := os.Environ()
		syscall.Exec("/bin/bash", args, env)
		os.Exit(0)
		break
	}
	return nil
}

func (ctrl *InstanceController) quit(g *gocui.Gui, v *gocui.View) error {
	return gocui.ErrQuit
}

func (ctrl *InstanceController) layout(g *gocui.Gui) error {
	maxX, maxY := g.Size()
	var err error
	var views [columns]*gocui.View

	colWidth := int((maxX - 1) / 4)
	maxHeight := 5
	for colIdx := 0; colIdx < columns; colIdx++ {
		colHeight := len(ctrl.instancesByCol[colIdx])
		if maxHeight < colHeight {
			maxHeight = colHeight
		}
	}
	maxY = maxHeight + 2 // Top line + bottom line = 2

	// Create column views
	for i := 0; i < columns; i++ {
		viewName := fmt.Sprintf("instances%d", i)
		views[i], err = g.SetView(viewName, colWidth*i-1, 0, colWidth*(i+1)-1, maxY-1)
		views[i].Frame = false
		if err != nil && err != gocui.ErrUnknownView {
			return err
		}
	}

	// Refresh instance list columns as needed
	for colIdx := 0; colIdx < columns; colIdx++ {
		rowChanged := (ctrl.cursorY != ctrl.newCursorY) && (colIdx == ctrl.cursorX)
		columnChanged := (ctrl.cursorX != ctrl.newCursorX && (colIdx == ctrl.cursorX || colIdx == ctrl.newCursorX))
		refreshNeeded := rowChanged || columnChanged || ctrl.refreshNeeded
		if refreshNeeded {
			views[colIdx].Clear()
			for i, instance := range ctrl.instancesByCol[colIdx] {
				highlighted := i == ctrl.newCursorY && colIdx == ctrl.newCursorX
				if highlighted {
					ctrl.curInstance = instance
					instance.CheckConnectivity(func(instance *Instance) {
						ctrl.RefreshView()
					})
				}
				ctrl.renderItem(views[colIdx], highlighted, instance, colWidth)
			}
		}
	}

	ctrl.cursorX = ctrl.newCursorX
	ctrl.cursorY = ctrl.newCursorY

	return nil
}

func (ctrl *InstanceController) renderItem(writer io.Writer, highlighted bool, instance *Instance, colWidth int) {
	instanceLabel := instance.Label()
	var buttonLabel string
	if instance.Selected() {
		buttonLabel = fmt.Sprintf(" - %s *", instanceLabel)
	} else {
		buttonLabel = fmt.Sprintf(" - %s", instanceLabel)
	}

	const boldOn = "\x1b[1m"
	const boldOff = "\x1b[21m"
	const black = "\x1b[30m"
	const yellow = "\x1b[33m"
	const whiteBg = "\x1b[47m"
	const defaultFg = "\x1b[39m"
	const defaultBg = "\x1b[49m"

	if highlighted {
		fmt.Fprint(writer, whiteBg)
	}
	if instance.Selected() {
		fmt.Fprint(writer, yellow)
	} else if highlighted {
		fmt.Fprint(writer, black)
	}
	if highlighted || instance.Selected() {
		fmt.Fprint(writer, boldOn)
	}
	buttonLabel = RightPad(buttonLabel, colWidth, " ")
	fmt.Fprint(writer, buttonLabel)
	if highlighted || instance.Selected() {
		fmt.Fprint(writer, boldOff)
	}
	if instance.Selected() {
		fmt.Fprint(writer, defaultFg)
	} else if highlighted {
		fmt.Fprint(writer, defaultFg)
	}
	if highlighted {
		fmt.Fprint(writer, defaultBg)
	}
	fmt.Fprint(writer, "\n")
}

func (ctrl InstanceController) selectedInstances() []*Instance {
	selected := make([]*Instance, 0)
	for _, instance := range ctrl.instances {
		if instance.Selected() {
			selected = append(selected, instance)
		}
	}
	return selected
}
