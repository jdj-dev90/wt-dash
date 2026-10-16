package main

import "github.com/charmbracelet/lipgloss"

// Kanagawa (wave) palette.
var (
	cFg      = lipgloss.Color("#dcd7ba")
	cBorder  = lipgloss.Color("#363646")
	cDim     = lipgloss.Color("#727169")
	cBlue    = lipgloss.Color("#7e9cd8")
	cViolet  = lipgloss.Color("#957fb8")
	cAqua    = lipgloss.Color("#7aa89f")
	cGreen   = lipgloss.Color("#98bb6c")
	cYellow  = lipgloss.Color("#e6c384")
	cOrange  = lipgloss.Color("#ffa066")
	cRed     = lipgloss.Color("#e46876")
	cMagenta = lipgloss.Color("#d27e99")
)

var (
	sTitle   = lipgloss.NewStyle().Foreground(cViolet).Bold(true)
	sRepo    = lipgloss.NewStyle().Foreground(cBlue).Bold(true)
	sRule    = lipgloss.NewStyle().Foreground(cBorder)
	sColHead = lipgloss.NewStyle().Foreground(cDim).Bold(true)
	sDim     = lipgloss.NewStyle().Foreground(cDim)
	sFg      = lipgloss.NewStyle().Foreground(cFg)
	sBranch  = lipgloss.NewStyle().Foreground(cFg).Bold(true)
	sMainBr  = lipgloss.NewStyle().Foreground(cDim).Bold(true)
	sAdd     = lipgloss.NewStyle().Foreground(cGreen)
	sDel     = lipgloss.NewStyle().Foreground(cRed)
	sAhead   = lipgloss.NewStyle().Foreground(cAqua)
	sBehind  = lipgloss.NewStyle().Foreground(cOrange)
	sStatus  = lipgloss.NewStyle().Foreground(cYellow)
	sErr     = lipgloss.NewStyle().Foreground(cRed)
	sOK      = lipgloss.NewStyle().Foreground(cGreen)
	sCursor  = lipgloss.NewStyle().Foreground(cOrange).Bold(true)
	sPrompt  = lipgloss.NewStyle().Foreground(cViolet).Bold(true)
	sKey     = lipgloss.NewStyle().Foreground(cBlue)
	sCIPass  = lipgloss.NewStyle().Foreground(cGreen)
	sCIRun   = lipgloss.NewStyle().Foreground(cYellow)
	sCIFail  = lipgloss.NewStyle().Foreground(cRed)
	sCINone  = lipgloss.NewStyle().Foreground(cDim)
	sReview  = lipgloss.NewStyle().Foreground(cMagenta)
)

// selBg is the selected row's background (#2d4f67, Kanagawa waveBlue2) as a raw
// SGR so it can be re-applied after each styled cell's reset.
const selBg = "\x1b[48;2;45;79;103m"
