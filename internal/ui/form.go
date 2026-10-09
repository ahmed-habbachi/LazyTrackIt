package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/ahmed-habbachi/lazytrackit/internal/config"
	"github.com/ahmed-habbachi/lazytrackit/internal/provider"
)

type formField int

// Field order matches the visual top-to-bottom layout in viewForm (WHAT
// group, then WHEN group) so tab/shift+tab moves the way it looks like it
// should instead of jumping between groups.
const (
	fieldProject formField = iota
	fieldDescription
	fieldTags
	fieldDate
	fieldFrom
	fieldTo
	fieldCount
)

type entryForm struct {
	editingID  string // "" => creating a new entry
	projects   []provider.Project
	projectIdx int

	date        dateField
	from        timeField
	to          timeField
	description textinput.Model
	tags        textinput.Model

	// availableTags holds the current project's defined tags, fetched
	// asynchronously, so the Tags field's hint can suggest names instead of
	// making the user guess tag IDs.
	availableTags []provider.Tag
	tagsLoading   bool
	tagsErr       string

	focus formField
	err   string
}

// currentProjectID returns the ID of whichever project is currently
// selected, or "" if there isn't one (e.g. no projects available).
func (f *entryForm) currentProjectID() string {
	if f.projectIdx < 0 || f.projectIdx >= len(f.projects) {
		return ""
	}
	return f.projects[f.projectIdx].ID
}

// beginLoadingTags resets tag state ahead of a fetch for the currently
// selected project. Tags are project-scoped, so switching projects discards
// whatever was fetched (or selected as a name) for the previous one.
func (f *entryForm) beginLoadingTags() {
	f.availableTags = nil
	f.tagsErr = ""
	f.tagsLoading = true
}

// setAvailableTags applies a tagsLoadedMsg's result to the form.
func (f *entryForm) setAvailableTags(tags []provider.Tag, err error) {
	f.tagsLoading = false
	if err != nil {
		f.tagsErr = err.Error()
		f.availableTags = nil
		return
	}
	f.tagsErr = ""
	f.availableTags = tags
	f.renderKnownTagIDsAsNames()
}

// renderKnownTagIDsAsNames rewrites the Tags field in place, replacing any
// token that's literally one of availableTags' IDs with that tag's name.
// This is what turns the remembered last-used tags (stored as IDs, since
// that's what gets sent to the API) back into readable names once the
// project's tags have loaded, the same way an edited entry's existing
// TagIDs get shown. Only exact ID matches are touched, so a name the user
// already typed, or an ID that doesn't resolve, is left alone.
func (f *entryForm) renderKnownTagIDsAsNames() {
	if len(f.availableTags) == 0 {
		return
	}
	nameByID := make(map[string]string, len(f.availableTags))
	for _, t := range f.availableTags {
		nameByID[t.ID] = t.Name
	}
	tokens := parseStrings(f.tags.Value())
	changed := false
	for i, tok := range tokens {
		if name, ok := nameByID[tok]; ok {
			tokens[i] = name
			changed = true
		}
	}
	if changed {
		f.tags.SetValue(joinStrings(tokens))
		f.tags.CursorEnd()
	}
}

// tagsHint renders the helper line shown under the Tags field: the fetch
// state, or the names of the tags usable on the current project — its own
// tags plus every system tag — grouped separately since they come from two
// different sources and a name collision would otherwise be ambiguous.
func (f *entryForm) tagsHint() string {
	switch {
	case f.tagsErr != "":
		return errorStyle.Render("couldn't load tags: " + f.tagsErr)
	case f.tagsLoading:
		return subtleStyle.Render("fetching tags…")
	case len(f.availableTags) == 0:
		return ""
	default:
		var project, system []string
		for _, t := range f.availableTags {
			if t.IsSystem {
				system = append(system, t.Name)
			} else {
				project = append(project, t.Name)
			}
		}
		var parts []string
		if len(project) > 0 {
			parts = append(parts, "project: "+strings.Join(project, ", "))
		}
		if len(system) > 0 {
			parts = append(parts, "system: "+strings.Join(system, ", "))
		}
		return subtleStyle.Render("available — " + strings.Join(parts, "   "))
	}
}

// resolveTagIDs maps each entered token to a tag ID, accepting either a tag
// name (case-insensitive) or a raw ID. If available is empty (tags haven't
// loaded, or the provider has none), tokens are passed through unchanged so
// typing a raw ID still works exactly as before.
func resolveTagIDs(tokens []string, available []provider.Tag) ([]string, error) {
	if len(available) == 0 {
		return tokens, nil
	}
	byID := make(map[string]provider.Tag, len(available))
	byName := make(map[string]provider.Tag, len(available))
	for _, t := range available {
		byID[t.ID] = t
		byName[strings.ToLower(t.Name)] = t
	}
	out := make([]string, 0, len(tokens))
	for _, tok := range tokens {
		if t, ok := byID[tok]; ok {
			out = append(out, t.ID)
			continue
		}
		if t, ok := byName[strings.ToLower(tok)]; ok {
			out = append(out, t.ID)
			continue
		}
		return nil, fmt.Errorf("unknown tag %q", tok)
	}
	return out, nil
}

// dateField is a small YYYY-MM-DD spinner widget mirroring timeField:
// left/right move between the year/month/day segment, up/down nudge the
// active segment by one (wrapping within that segment, e.g. month wraps
// 12->1 without touching the year), and digit keys overwrite the active
// segment outright. The day is clamped to the active month's length
// whenever the year or month changes, so the field can never hold an
// invalid date.
type dateField struct {
	value   time.Time // normalized date, always midnight
	segment int       // 0 = year, 1 = month, 2 = day
	entered int       // digits typed into the active segment since it was last (re)focused
}

func newDateField(t time.Time) dateField {
	return dateField{value: startOfDay(t)}
}

func (d dateField) year() int  { return d.value.Year() }
func (d dateField) month() int { return int(d.value.Month()) }
func (d dateField) day() int   { return d.value.Day() }

func daysInMonth(year, month int) int {
	return time.Date(year, time.Month(month)+1, 0, 0, 0, 0, 0, time.UTC).Day()
}

func (d *dateField) setYear(y int) {
	day := d.day()
	if max := daysInMonth(y, d.month()); day > max {
		day = max
	}
	d.value = time.Date(y, time.Month(d.month()), day, 0, 0, 0, 0, time.UTC)
}

func (d *dateField) setMonth(m int) {
	if m < 1 {
		m = 1
	} else if m > 12 {
		m = 12
	}
	day := d.day()
	if max := daysInMonth(d.year(), m); day > max {
		day = max
	}
	d.value = time.Date(d.year(), time.Month(m), day, 0, 0, 0, 0, time.UTC)
}

func (d *dateField) setDay(day int) {
	max := daysInMonth(d.year(), d.month())
	if day < 1 {
		day = 1
	} else if day > max {
		day = max
	}
	d.value = time.Date(d.year(), time.Month(d.month()), day, 0, 0, 0, 0, time.UTC)
}

// focus resets the field so the year segment is active, matching timeField.
func (d *dateField) focus() {
	d.segment = 0
	d.entered = 0
}

func (d *dateField) handleKey(msg tea.KeyMsg) bool {
	switch msg.String() {
	case "left":
		if d.segment > 0 {
			d.segment--
		}
		d.entered = 0
		return true
	case "right":
		if d.segment < 2 {
			d.segment++
		}
		d.entered = 0
		return true
	case "up":
		d.step(1)
		return true
	case "down":
		d.step(-1)
		return true
	case "backspace":
		if d.entered == 0 {
			if d.segment > 0 {
				d.segment--
			}
		} else {
			d.entered = 0
		}
		return true
	}

	if r := msg.Runes; len(r) == 1 && r[0] >= '0' && r[0] <= '9' {
		d.typeDigit(int(r[0] - '0'))
		return true
	}
	return false
}

// step nudges the active segment by delta, wrapping within that segment
// only (e.g. day wraps within the current month's length).
func (d *dateField) step(delta int) {
	switch d.segment {
	case 0:
		d.setYear(d.year() + delta)
	case 1:
		d.setMonth(((d.month()-1+delta)%12+12)%12 + 1)
	default:
		max := daysInMonth(d.year(), d.month())
		d.setDay(((d.day()-1+delta)%max+max)%max + 1)
	}
	d.entered = 0
}

func (d *dateField) curVal() int {
	switch d.segment {
	case 0:
		return d.year()
	case 1:
		return d.month()
	default:
		return d.day()
	}
}

func (d *dateField) setCur(v int) {
	switch d.segment {
	case 0:
		d.setYear(v)
	case 1:
		d.setMonth(v)
	default:
		d.setDay(v)
	}
}

// typeDigit overwrites the active segment the same way timeField does: the
// first digit after a segment change replaces its value outright, and
// further digits combine with it while the result stays valid. The year
// segment takes up to 4 digits; month and day advance to the next segment
// once they're unambiguously complete, exactly like timeField's hour/minute.
func (d *dateField) typeDigit(digit int) {
	if d.segment == 0 {
		const yearDigits = 4
		val := digit
		if d.entered > 0 && d.entered < yearDigits {
			val = d.curVal()*10 + digit
		}
		d.entered++
		d.setYear(val)
		if d.entered >= yearDigits {
			d.segment = 1
			d.entered = 0
		}
		return
	}

	max := 12
	if d.segment == 2 {
		max = daysInMonth(d.year(), d.month())
	}

	val := digit
	if d.entered == 1 {
		if candidate := d.curVal()*10 + digit; candidate <= max {
			val = candidate
			d.entered = 2
		} else {
			d.entered = 1
		}
	} else {
		d.entered = 1
	}
	d.setCur(val)

	if d.entered == 2 || val*10 > max {
		if d.segment < 2 {
			d.segment++
		}
		d.entered = 0
	}
}

// view renders "YYYY-MM-DD", highlighting the active segment when focused.
func (d dateField) view(focused bool) string {
	yyyy := fmt.Sprintf("%04d", d.year())
	mm := fmt.Sprintf("%02d", d.month())
	dd := fmt.Sprintf("%02d", d.day())
	if !focused {
		return yyyy + "-" + mm + "-" + dd
	}
	switch d.segment {
	case 0:
		return timeSegmentStyle.Render(yyyy) + "-" + mm + "-" + dd
	case 1:
		return yyyy + "-" + timeSegmentStyle.Render(mm) + "-" + dd
	default:
		return yyyy + "-" + mm + "-" + timeSegmentStyle.Render(dd)
	}
}

// timeField is a small HH:MM spinner widget: left/right move between the
// hour and minute segment, up/down nudge the active segment by one, and
// digit keys overwrite the active segment outright instead of inserting
// into it, so fixing a time never requires backspacing first. Values are
// clamped as they're typed, so the field can never hold an invalid time.
type timeField struct {
	minutes int // 0..1439, minutes after midnight
	segment int // 0 = hour, 1 = minute
	entered int // digits typed into the active segment since it was last (re)focused: 0, 1, or 2
}

func newTimeField(minutes int) timeField {
	return timeField{minutes: minutes}
}

func (t timeField) hour() int   { return t.minutes / 60 }
func (t timeField) minute() int { return t.minutes % 60 }

func (t *timeField) setHour(h int) {
	if h < 0 {
		h = 0
	} else if h > 23 {
		h = 23
	}
	t.minutes = h*60 + t.minute()
}

func (t *timeField) setMinute(mn int) {
	if mn < 0 {
		mn = 0
	} else if mn > 59 {
		mn = 59
	}
	t.minutes = t.hour()*60 + mn
}

// focus resets the field so the hour segment is active and ready to be
// typed over fresh, regardless of how it was left last time.
func (t *timeField) focus() {
	t.segment = 0
	t.entered = 0
}

// handleKey applies a key to the field and reports whether it owned it.
func (t *timeField) handleKey(msg tea.KeyMsg) bool {
	switch msg.String() {
	case "left", "h":
		t.segment = 0
		t.entered = 0
		return true
	case "right", "l":
		t.segment = 1
		t.entered = 0
		return true
	case "up":
		t.step(1)
		return true
	case "down":
		t.step(-1)
		return true
	case "backspace":
		if t.entered == 0 {
			if t.segment == 1 {
				t.segment = 0
			}
		} else {
			t.entered = 0
		}
		return true
	}

	if r := msg.Runes; len(r) == 1 && r[0] >= '0' && r[0] <= '9' {
		t.typeDigit(int(r[0] - '0'))
		return true
	}
	return false
}

func (t *timeField) step(delta int) {
	if t.segment == 0 {
		t.setHour(((t.hour()+delta)%24 + 24) % 24)
	} else {
		t.setMinute(((t.minute()+delta)%60 + 60) % 60)
	}
	t.entered = 0
}

func (t *timeField) curVal() int {
	if t.segment == 0 {
		return t.hour()
	}
	return t.minute()
}

func (t *timeField) setCur(v int) {
	if t.segment == 0 {
		t.setHour(v)
	} else {
		t.setMinute(v)
	}
}

// typeDigit overwrites the active segment: the first digit after a segment
// change replaces its value outright, and a second digit combines with it
// if the result is still valid. Once a segment is unambiguously complete
// (two digits entered, or a second digit couldn't possibly fit) the field
// advances from hour to minute automatically.
func (t *timeField) typeDigit(d int) {
	max := 23
	if t.segment == 1 {
		max = 59
	}

	val := d
	if t.entered == 1 {
		if candidate := t.curVal()*10 + d; candidate <= max {
			val = candidate
			t.entered = 2
		} else {
			t.entered = 1
		}
	} else {
		t.entered = 1
	}
	t.setCur(val)

	// Advance once the segment is unambiguously done: either two digits
	// have been entered, or even a trailing "0" would overflow (e.g. a
	// leading "3" in an hour can't be followed by anything valid).
	if t.entered == 2 || val*10 > max {
		if t.segment == 0 {
			t.segment = 1
		}
		t.entered = 0
	}
}

// view renders "HH:MM", highlighting the active segment when focused.
func (t timeField) view(focused bool) string {
	hh := fmt.Sprintf("%02d", t.hour())
	mm := fmt.Sprintf("%02d", t.minute())
	if !focused {
		return hh + ":" + mm
	}
	if t.segment == 0 {
		return timeSegmentStyle.Render(hh) + ":" + mm
	}
	return hh + ":" + timeSegmentStyle.Render(mm)
}

func newTextInput(placeholder, value string, charLimit, width int) textinput.Model {
	ti := textinput.New()
	ti.Placeholder = placeholder
	ti.SetValue(value)
	ti.CharLimit = charLimit
	ti.Width = width
	return ti
}

// newEntryForm builds a form. Pass an existing entry to edit it, or nil to
// create a new one defaulted to the given week's current day/hour and to
// defaults' project/tags (usually the ones last used, since entries tend to
// be logged against the same project/tags in a row).
func newEntryForm(existing *provider.TimeEntry, projects []provider.Project, weekStart time.Time, defaults config.LastEntryDefaults) *entryForm {
	f := &entryForm{projects: projects}

	date := defaultEntryDate(weekStart)
	fromMin := roundToHour(time.Now())
	toMin := fromMin + 60
	description := ""
	tagIDs := defaults.TagIDs
	projectID := defaults.ProjectID

	if existing != nil {
		f.editingID = existing.ID
		date = startOfDay(existing.Start)
		fromMin = minutesOfDay(existing.Start)
		toMin = minutesOfDay(existing.End)
		description = existing.Description
		tagIDs = existing.TagIDs
		projectID = existing.ProjectID
	}

	for i, p := range projects {
		if p.ID == projectID {
			f.projectIdx = i
			break
		}
	}

	f.date = newDateField(date)
	f.from = newTimeField(fromMin)
	f.to = newTimeField(toMin)
	f.description = newTextInput("What did you work on?", description, 500, 50)
	f.tags = newTextInput("comma-separated tag names (or IDs), optional", joinStrings(tagIDs), 100, 50)
	f.beginLoadingTags()

	f.focus = fieldProject
	f.focusCurrent()
	return f
}

func joinStrings(ids []string) string {
	return strings.Join(ids, ",")
}

func parseStrings(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		out = append(out, p)
	}
	return out
}

func roundToHour(t time.Time) int {
	return t.Hour() * 60
}

// minutesOfDay returns t's time-of-day as minutes after midnight.
func minutesOfDay(t time.Time) int {
	return t.Hour()*60 + t.Minute()
}

// defaultEntryDate picks "today" when the currently-viewed week contains
// today, and the Monday of that week otherwise, so a new entry's default
// date always falls within the week the user is looking at.
func defaultEntryDate(weekStart time.Time) time.Time {
	today := startOfDay(time.Now())
	weekEnd := weekStart.AddDate(0, 0, 6)
	if !today.Before(weekStart) && !today.After(weekEnd) {
		return today
	}
	return weekStart
}

// inputs lists the plain-text fields; fieldProject, the dateField, and the
// timeFields (fieldFrom/fieldTo) manage their own focus and key handling
// instead.
func (f *entryForm) inputs() []*textinput.Model {
	return []*textinput.Model{&f.description, &f.tags}
}

// fieldToInputIndex maps a formField to its index within inputs(), or -1
// for fields that aren't plain-text inputs.
func fieldToInputIndex(fld formField) int {
	switch fld {
	case fieldDescription:
		return 0
	case fieldTags:
		return 1
	default:
		return -1
	}
}

func (f *entryForm) focusCurrent() {
	for _, in := range f.inputs() {
		in.Blur()
	}
	if idx := fieldToInputIndex(f.focus); idx >= 0 {
		f.inputs()[idx].Focus()
	}
	switch f.focus {
	case fieldDate:
		f.date.focus()
	case fieldFrom:
		f.from.focus()
	case fieldTo:
		f.to.focus()
	}
}

func (f *entryForm) next() {
	f.focus = (f.focus + 1) % fieldCount
	f.focusCurrent()
}

func (f *entryForm) prev() {
	f.focus = (f.focus - 1 + fieldCount) % fieldCount
	f.focusCurrent()
}

func (m Model) handleFormKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	f := m.form
	if f == nil {
		m.screen = screenList
		return m, nil
	}

	switch msg.String() {
	case "esc":
		m.form = nil
		m.screen = screenList
		return m, nil

	case "enter":
		return m.submitForm()
	}

	if f.focus == fieldFrom || f.focus == fieldTo {
		tf := &f.from
		if f.focus == fieldTo {
			tf = &f.to
		}
		if tf.handleKey(msg) {
			f.err = ""
			return m, nil
		}
	}

	if f.focus == fieldDate {
		if f.date.handleKey(msg) {
			f.err = ""
			return m, nil
		}
	}

	// "h"/"l" are only vim-style project-nav aliases while the Project field
	// is focused; everywhere else they're ordinary letters the user may want
	// to type (e.g. into Description or Tags), so they must fall through to
	// the text input below instead of being swallowed here.
	if f.focus == fieldProject && len(f.projects) > 0 {
		switch msg.String() {
		case "left", "h":
			f.projectIdx = (f.projectIdx - 1 + len(f.projects)) % len(f.projects)
			f.beginLoadingTags()
			go m.loadTags(f.currentProjectID())
			return m, nil

		case "right", "l":
			f.projectIdx = (f.projectIdx + 1) % len(f.projects)
			f.beginLoadingTags()
			go m.loadTags(f.currentProjectID())
			return m, nil
		}
	}

	switch msg.String() {
	case "tab", "down":
		f.next()
		return m, nil

	case "shift+tab", "up":
		f.prev()
		return m, nil
	}

	idx := fieldToInputIndex(f.focus)
	if idx < 0 {
		// Arrow keys have no effect on non-text fields.
		return m, nil
	}
	inputs := f.inputs()
	var cmd tea.Cmd
	*inputs[idx], cmd = inputs[idx].Update(msg)
	return m, cmd
}

func (m Model) submitForm() (tea.Model, tea.Cmd) {
	f := m.form
	if len(f.projects) == 0 {
		f.err = "No projects available; cannot create a time entry."
		return m, nil
	}

	date := f.date.value
	fromMin, toMin := f.from.minutes, f.to.minutes
	if toMin <= fromMin {
		f.err = "To must be after From."
		return m, nil
	}
	tagIDs, err := resolveTagIDs(parseStrings(f.tags.Value()), f.availableTags)
	if err != nil {
		f.err = err.Error() + " — see available tags below."
		return m, nil
	}

	proj := f.projects[f.projectIdx]
	entry := provider.TimeEntry{
		ID:          f.editingID,
		ProjectID:   proj.ID,
		ProjectName: proj.Name,
		MemberID:    m.member.ID,
		Start:       date.Add(time.Duration(fromMin) * time.Minute),
		End:         date.Add(time.Duration(toMin) * time.Minute),
		Description: strings.TrimSpace(f.description.Value()),
		TagIDs:      tagIDs,
	}

	isEdit := f.editingID != ""
	m.screen = screenLoading
	go m.saveEntry(entry, isEdit)
	return m, nil
}

func (m Model) viewForm() string {
	f := m.form
	if f == nil {
		return ""
	}

	title := " New time entry "
	if f.editingID != "" {
		title = fmt.Sprintf(" Edit time entry #%s ", f.editingID)
	}

	var b strings.Builder
	b.WriteString(formTitleStyle.Render(title) + "\n\n")

	b.WriteString(groupHeadingStyle.Render("WHAT") + "\n")
	projectLabel := "(no projects)"
	if len(f.projects) > 0 {
		p := f.projects[f.projectIdx]
		projectLabel = fmt.Sprintf("◀ %s ▶", p.Name)
		if p.ClientName != "" {
			projectLabel = fmt.Sprintf("◀ %s — %s ▶", p.Name, p.ClientName)
		}
	}
	b.WriteString(renderField("📁 Project", projectLabel, f.focus == fieldProject))
	b.WriteString(renderField("📝 Description", f.description.View(), f.focus == fieldDescription))
	b.WriteString(renderField("🏷  Tags", f.tags.View(), f.focus == fieldTags))
	if hint := f.tagsHint(); hint != "" {
		b.WriteString(strings.Repeat(" ", 20) + hint + "\n")
	}

	b.WriteString("\n" + groupHeadingStyle.Render("WHEN") + "\n")
	b.WriteString(renderField("📅 Date", f.date.view(f.focus == fieldDate), f.focus == fieldDate))

	fromView := f.from.view(f.focus == fieldFrom)
	b.WriteString(renderField("🕐 From", fromView, f.focus == fieldFrom))

	toView := f.to.view(f.focus == fieldTo)
	toHint := subtleStyle.Render(fmt.Sprintf("(%s)", formatDuration(time.Duration(f.to.minutes-f.from.minutes)*time.Minute)))
	if f.to.minutes <= f.from.minutes {
		toHint = errorStyle.Render("⚠ must be after From")
	}
	b.WriteString(renderField("🕐 To", toView+"  "+toHint, f.focus == fieldTo))

	if f.err != "" {
		b.WriteString("\n" + errorStyle.Render(f.err) + "\n")
	}

	help := "[tab] next field  [enter] save  [esc] cancel"
	if f.focus == fieldFrom || f.focus == fieldTo {
		help = "[0-9] type  [←/→] hour/min  [↑/↓] ±1  [tab] next field  [enter] save  [esc] cancel"
	} else if f.focus == fieldProject {
		help = "[←/→] change project  [tab] next field  [enter] save  [esc] cancel"
	} else if f.focus == fieldDate {
		help = "[0-9] type  [←/→] year/month/day  [↑/↓] ±1  [tab] next field  [enter] save  [esc] cancel"
	}
	b.WriteString("\n" + helpStyle.Render(help))

	return "\n" + boxStyle.Render(b.String()) + "\n"
}

func renderField(label, value string, focused bool) string {
	marker := "  "
	l := fieldLabelStyle.Render(label + ":")
	if focused {
		marker = focusedFieldStyle.Render("▸ ")
		l = focusedFieldStyle.Render(label + ":")
	}
	return marker + l + " " + value + "\n"
}

func (m Model) viewConfirmDelete() string {
	e := m.deleteTarget
	if e == nil {
		return ""
	}
	body := errorStyle.Render("Delete this time entry?") + "\n\n" +
		fmt.Sprintf("%s  %s–%s  %s  %s\n",
			e.Start.Format("Mon 01/02"), formatClock(e.Start), formatClock(e.End),
			e.ProjectName, e.Description) +
		helpStyle.Render("[y] yes, delete   [n/esc] cancel")
	return "\n" + boxStyle.Render(body) + "\n"
}
