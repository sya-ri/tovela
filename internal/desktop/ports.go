package desktop

import (
	"errors"
	"slices"
	"strconv"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/sya-ri/tovela/internal/locale"
	"github.com/sya-ri/tovela/internal/relay"
)

type portEditor struct {
	content *fyne.Container
	list    *fyne.Container
	add     *widget.Button
	rows    []*portFields
}

type portFields struct {
	protocol *widget.Select
	local    *widget.Entry
	public   *widget.Entry
	remove   *widget.Button
	content  *fyne.Container
}

func newPortEditor(ports []relay.PortMapping) *portEditor {
	editor := &portEditor{list: container.NewVBox()}
	editor.add = widget.NewButtonWithIcon(locale.T("Add port"), theme.ContentAddIcon(), func() {
		editor.addRow(relay.PortMapping{Protocol: relay.TCP})
	})
	editor.add.Alignment = widget.ButtonAlignLeading
	editor.content = container.NewVBox(editor.list, editor.add)
	for _, port := range ports {
		editor.addRow(port)
	}
	return editor
}

func (e *portEditor) addRow(port relay.PortMapping) {
	protocol := portProtocolSelect()
	label := string(port.Protocol)
	if port.Protocol == relay.MinecraftJava {
		label = "Minecraft"
	}
	protocol.SetSelected(label)
	local := entry(portText(port.LocalPort), "")
	public := entry(portText(port.PublicPort), portText(port.LocalPort))
	local.OnChanged = func(value string) { public.SetPlaceHolder(value) }
	row := &portFields{protocol: protocol, local: local, public: public}
	row.remove = widget.NewButtonWithIcon("", theme.DeleteIcon(), func() {
		e.rows = slices.DeleteFunc(e.rows, func(field *portFields) bool { return field == row })
		e.list.Remove(row.content)
	})
	top := container.NewBorder(nil, nil, nil, row.remove, protocol)
	fields := container.NewGridWithColumns(2,
		container.NewVBox(widget.NewLabel(locale.T("Local")), local),
		container.NewVBox(widget.NewLabel(locale.T("Public")), public),
	)
	row.content = container.NewVBox(top, fields, widget.NewSeparator())
	e.rows = append(e.rows, row)
	e.list.Add(row.content)
}

func (e *portEditor) setEnabled(enabled bool) {
	setEnabled(e.add, enabled)
	for _, row := range e.rows {
		setEnabled(row.protocol, enabled)
		setEnabled(row.local, enabled)
		setEnabled(row.public, enabled)
		setEnabled(row.remove, enabled)
	}
}

func (e *portEditor) mappings() ([]relay.PortMapping, error) {
	ports := make([]relay.PortMapping, 0, len(e.rows))
	for _, row := range e.rows {
		local, err := strconv.Atoi(strings.TrimSpace(row.local.Text))
		if err != nil {
			return nil, errors.New(locale.T("Enter local and public ports from 1 to 65535."))
		}
		public := local
		if text := strings.TrimSpace(row.public.Text); text != "" {
			public, err = strconv.Atoi(text)
			if err != nil {
				return nil, errors.New(locale.T("Enter local and public ports from 1 to 65535."))
			}
		}
		protocol := relay.PortProtocol(row.protocol.Selected)
		if row.protocol.Selected == "Minecraft" {
			protocol = relay.MinecraftJava
		}
		ports = append(ports, relay.PortMapping{Protocol: protocol, LocalPort: local, PublicPort: public})
	}
	return ports, nil
}

func portProtocolSelect() *widget.Select {
	return &widget.Select{Options: []string{"Minecraft", "TCP", "UDP"}, PlaceHolder: "Minecraft"}
}

func portText(port int) string {
	if port == 0 {
		return ""
	}
	return strconv.Itoa(port)
}
