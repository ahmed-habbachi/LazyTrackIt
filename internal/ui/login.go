package ui

import "fmt"

func (m Model) viewLogin() string {
	if m.loginPrompt == nil {
		return fmt.Sprintf("\n  %s Starting login...\n", m.spinner.View())
	}
	p := m.loginPrompt

	body := titleStyle.Render("Log in to continue") + "\n\n"
	if p.Message != "" {
		body += p.Message + "\n\n"
	}
	if p.URL != "" {
		body += focusedFieldStyle.Render(p.URL) + "\n\n"
	}
	if p.Code != "" {
		body += titleStyle.Render(p.Code) + "\n\n"
	}
	body += subtleStyle.Render(fmt.Sprintf("%s Waiting for approval…  [q] cancel", m.spinner.View()))

	return "\n" + boxStyle.Render(body) + "\n"
}
